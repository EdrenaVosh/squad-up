package tui_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/EdrenaVosh/squadup/internal/tui"
)

func TestHomeAt80x24(t *testing.T) {
	h := newHarness(t)

	h.golden("home")
}

func TestSelectionMovesTheCard(t *testing.T) {
	h := newHarness(t)

	h.contains("Bug Hunt", "A debugger and a tester")
	h.press("down")
	h.contains("Refactor Day", "architect")
	h.lacks("Bug Hunt")
	h.press("j")
	h.contains("Solo Indie", "─ 3 ", "tester")
	h.press("up", "k")
	h.contains("Bug Hunt")
}

func TestStartsOnTheEquippedPresetAndMarksIt(t *testing.T) {
	h := equipped(t, "solo-indie")

	h.contains("▌ ◉ solo-indie", "● EQUIPPED", "Solo Indie")
	h.golden("home-equipped")
}

func TestInitialIsActiveUntilAPresetIsEquipped(t *testing.T) {
	h := newHarness(t)

	h.goTo("initial")

	h.contains("● ACTIVE", "your original setup · captured 2026-10-24", "─ 1 ", "docs-writer")
	h.golden("home-initial")
}

func TestManualEditsShowABanner(t *testing.T) {
	h := equipped(t, "solo-indie")
	h.lacks("Manual edits")
	if err := os.WriteFile(filepath.Join(h.paths.ClaudeDir, "agents", "planner.md"), []byte("edited by hand"), 0o644); err != nil {
		t.Fatal(err)
	}

	h2 := harnessFor(t, h.ws, tui.Env{Paths: h.paths, Now: h.ws.Now, ClaudeVersion: "2.1.309"})

	h2.contains("Manual edits detected")
	h2.fits(80, 24)
}

func TestFilterNarrowsTheListAndEscClearsIt(t *testing.T) {
	h := newHarness(t)

	h.press("/")
	h.typeText("sol")

	h.contains("solo-indie", "initial", "/ sol")
	h.lacks("bug-hunt", "refactor-day")
	h.press("enter")
	h.contains("Solo Indie")
	h.press("esc")
	h.contains("bug-hunt", "solo-indie")
}

func TestFilterWithNoMatch(t *testing.T) {
	h := newHarness(t)

	h.press("/")
	h.typeText("zzz")

	h.contains("nothing matches", "initial")
}

func TestHelpOpensAndClosesOnAnyKey(t *testing.T) {
	h := newHarness(t)

	h.press("?")
	h.contains("Help", "equip it", "squad restore")
	h.golden("help")
	h.press("x")
	h.contains("PRESETS")
	h.lacks("Help")
}

func TestQuit(t *testing.T) {
	h := newHarness(t)

	h.press("q")
	if !h.quit {
		t.Error("q did not quit")
	}

	h2 := newHarness(t)
	h2.press("ctrl+c")
	if !h2.quit {
		t.Error("ctrl+c did not quit")
	}
}

func TestNewPresetFlowCreatesABlankPresetAndOpensIt(t *testing.T) {
	h := newHarness(t)

	h.press("n")
	h.contains("New preset")
	h.typeText("fresh")
	h.press("enter")
	h.contains("Start fresh from", "Blank preset", `Copy of "bug-hunt"`, "My current Claude Code setup")
	h.press("enter")

	h.contains("fresh", "AGENTS", "add agent")
	if !h.ws.Store().Exists("fresh") {
		t.Fatal("the preset was not created")
	}
	h.press("esc")
	h.contains("fresh")
}

func TestNewPresetCanStartFromACopyOrTheCurrentSetup(t *testing.T) {
	h := newHarness(t)
	h.press("n")
	h.typeText("copied")
	h.press("enter", "down", "enter")
	b, err := h.ws.Store().Load("copied")
	if err != nil || len(b.Agents) != 2 || b.Preset.Main == nil {
		t.Fatalf("copy of bug-hunt = %+v, %v", b, err)
	}
	h.press("esc")

	h.press("n")
	h.typeText("live")
	h.press("enter", "down", "down", "enter")
	live, err := h.ws.Store().Load("live")
	if err != nil || len(live.Agents) != 3 || live.Preset.Settings.Model != "opus" {
		t.Fatalf("current setup = %+v, %v", live, err)
	}
}

func TestNewPresetNameIsValidatedInTheDialog(t *testing.T) {
	h := newHarness(t)

	h.press("n")
	h.typeText("Bad Name")
	h.press("enter")
	h.contains("New preset", "preset names use lowercase")
	h.clear(8)
	h.typeText("solo-indie")
	h.press("enter")
	h.contains("already exists")
	h.press("esc")
	h.contains("PRESETS")
}

func TestCloneFlow(t *testing.T) {
	h := newHarness(t)

	h.press("c")
	h.contains("Clone preset", "bug-hunt-copy")
	h.press("enter")

	h.contains("Copied", "bug-hunt-copy")
	if !h.ws.Store().Exists("bug-hunt-copy") {
		t.Fatal("the copy was not stored")
	}
}

func TestCloneOfInitialMakesAnEditableCopy(t *testing.T) {
	h := newHarness(t)
	h.goTo("initial")

	h.press("c")
	h.contains("Clone preset", "my-setup", "stays as it is")
	h.press("enter")

	b, err := h.ws.Store().Load("my-setup")
	if err != nil || len(b.Agents) != 3 || b.Preset.Settings.Model != "opus" {
		t.Fatalf("my-setup = %+v, %v", b, err)
	}
}

func TestRenameFlow(t *testing.T) {
	h := newHarness(t)

	h.press("r")
	h.contains("Rename preset", "bug-hunt")
	h.clear(len("bug-hunt"))
	h.typeText("hunter")
	h.press("enter")

	h.contains("Renamed", "hunter")
	if h.ws.Store().Exists("bug-hunt") || !h.ws.Store().Exists("hunter") {
		t.Fatal("the preset was not renamed")
	}
}

func TestDeleteFlowAsksFirstAndDefaultsToCancel(t *testing.T) {
	h := newHarness(t)

	h.press("d")
	h.contains("Delete preset", `Delete the preset "bug-hunt"?`)
	h.press("enter")
	if !h.ws.Store().Exists("bug-hunt") {
		t.Fatal("enter on the default button must cancel")
	}
	h.press("d", "y")
	if h.ws.Store().Exists("bug-hunt") {
		t.Fatal("y did not delete")
	}
	h.contains("Deleted", "refactor-day")
}

func TestDeletingTheEquippedPresetWarns(t *testing.T) {
	h := equipped(t, "solo-indie")

	h.press("d")

	h.contains("It is equipped", "until you restore")
	h.press("y")
	if h.ws.Store().Exists("solo-indie") {
		t.Fatal("the equipped preset was not deleted after confirmation")
	}
}

func TestInitialCannotBeRenamedOrDeleted(t *testing.T) {
	h := newHarness(t)
	h.goTo("initial")

	h.press("r")
	h.contains("cannot be renamed or removed")
	h.press("d")
	h.contains("cannot be renamed or removed")
	h.lacks("Delete preset")
}

func TestTheInterfaceSurvivesSmallAndNarrowTerminals(t *testing.T) {
	for _, size := range [][2]int{{60, 20}, {45, 12}, {30, 8}, {120, 40}, {200, 60}} {
		h := newHarness(t)
		h.resize(size[0], size[1])
		h.fits(size[0], size[1])
		h.press("down", "enter")
		h.fits(size[0], size[1])
		h.press("esc", "e")
		h.fits(size[0], size[1])
		h.press("esc", "?")
		h.fits(size[0], size[1])
	}
}

func TestTooSmallTerminalSaysSo(t *testing.T) {
	h := newHarness(t)

	h.resize(30, 8)

	h.contains("Terminal too small")
}

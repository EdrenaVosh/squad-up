package tui_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/EdrenaVosh/squad-up/internal/apply"
	"github.com/EdrenaVosh/squad-up/internal/testutil"
)

func state(t *testing.T, h *harness) apply.State {
	t.Helper()
	s, err := apply.LoadState(h.paths)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestEquipConfirmShowsTheDiff(t *testing.T) {
	h := newHarness(t)
	h.goTo("solo-indie")

	h.press("e")

	h.contains("Equip solo-indie", "AGENTS", "+ planner.md", "− docs-writer.md", "SETTINGS", `model: "opus" → "sonnet"`, "[ Equip ]", "[ Cancel ]")
	h.golden("equip")
}

func TestEquipAppliesThePresetAfterConfirmation(t *testing.T) {
	h := newHarness(t)
	h.goTo("solo-indie")
	before, _ := os.ReadFile(filepath.Join(h.paths.ClaudeDir, "agents", "docs-writer.md"))

	h.press("e", "enter")

	h.contains("Done.", "A backup of the previous state", "squad undo")
	h.golden("equip-done")
	if got := state(t, h).Active; got != "solo-indie" {
		t.Fatalf("active = %q", got)
	}
	if _, err := os.Stat(filepath.Join(h.paths.ClaudeDir, "agents", "planner.md")); err != nil {
		t.Errorf("planner.md was not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(h.paths.ClaudeDir, "agents", "docs-writer.md")); !os.IsNotExist(err) {
		t.Error("the Initial agent should be gone in replace mode")
	}
	h.press("x")
	h.contains("● EQUIPPED", "Equipped \"solo-indie\"")
	_ = before
}

func TestEquipCanBeCancelled(t *testing.T) {
	h := newHarness(t)
	h.goTo("solo-indie")

	h.press("e", "esc")
	h.contains("PRESETS")
	h.press("e", "right", "enter")
	h.contains("PRESETS")
	h.press("e", "n")
	h.contains("PRESETS")

	if got := state(t, h).Active; got != "" {
		t.Fatalf("active = %q: cancelling must change nothing", got)
	}
	if _, err := os.Stat(filepath.Join(h.paths.ClaudeDir, "agents", "planner.md")); !os.IsNotExist(err) {
		t.Error("a cancelled equip wrote files")
	}
}

func TestEquipYAppliesAtOnce(t *testing.T) {
	h := newHarness(t)
	h.goTo("bug-hunt")

	h.press("e", "y")

	h.contains("Done.")
	if state(t, h).Active != "bug-hunt" {
		t.Fatal("y did not equip")
	}
}

func TestEquipExplainsAnAlreadyEquippedPreset(t *testing.T) {
	h := equipped(t, "solo-indie")

	h.press("e")

	h.contains("already equipped — nothing to change")
	h.press("x")
	h.contains("PRESETS")
}

func TestEquipListsConflictsAndForceOverwrites(t *testing.T) {
	h := newHarness(t)
	testutil.WriteFiles(t, h.paths.ClaudeDir, map[string]string{"agents/planner.md": agentFile("planner", "My own planner. Use when planning.", "", "Mine.")})
	h.goTo("solo-indie")

	h.press("e")
	h.contains("Files are in the way:", "agents/planner.md")
	h.press("y")
	h.contains("press f to overwrite")
	if state(t, h).Active != "" {
		t.Fatal("conflicts must block the equip")
	}
	h.press("f")
	h.lacks("Files are in the way")
	h.press("y")
	h.contains("Done.")
}

func TestEquipAsksWhatToDoWithManualEdits(t *testing.T) {
	h := equipped(t, "solo-indie")
	if err := os.WriteFile(filepath.Join(h.paths.ClaudeDir, "agents", "planner.md"), []byte(agentFile("planner", "Edited by hand. Use when planning.", "", "Hand.")), 0o644); err != nil {
		t.Fatal(err)
	}
	h2 := harnessFor(t, h.ws, tuiEnv(h))
	h2.goTo("bug-hunt")

	h2.press("e")
	h2.contains("Manual edits detected in the equipped preset:", "agents/planner.md (modified)", "s keeps them")
	h2.press("y")
	h2.contains("press s to save the edits")
	h2.press("s")
	h2.contains("will be saved into the preset first")
	h2.press("y")
	h2.contains("Done.")

	b, err := h.ws.Store().Load("solo-indie")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b.Agents[0].Data), "Edited by hand") {
		t.Errorf("the edit was not saved into the preset: %s", b.Agents[0].Data)
	}
}

func TestEquipCanDiscardManualEdits(t *testing.T) {
	h := equipped(t, "solo-indie")
	if err := os.WriteFile(filepath.Join(h.paths.ClaudeDir, "agents", "planner.md"), []byte("hand edit"), 0o644); err != nil {
		t.Fatal(err)
	}
	h2 := harnessFor(t, h.ws, tuiEnv(h))
	h2.goTo("bug-hunt")

	h2.press("e", "d")
	h2.contains("will be overwritten")
	h2.press("y")

	h2.contains("Done.")
	b, _ := h.ws.Store().Load("solo-indie")
	if strings.Contains(string(b.Agents[0].Data), "hand edit") {
		t.Error("discarding must not touch the preset")
	}
}

func TestRestoringInitialIsAnEquipToo(t *testing.T) {
	h := equipped(t, "solo-indie")
	h.goTo("initial")

	h.press("e")
	h.contains("Restore your original setup", "[ Restore ]")
	h.press("y")
	h.contains("Done.")
	h.press("x")

	h.contains("● ACTIVE", "Restored your original setup")
	if state(t, h).ActivePreset() != "initial" {
		t.Fatal("not restored")
	}
	if _, err := os.Stat(filepath.Join(h.paths.ClaudeDir, "agents", "docs-writer.md")); err != nil {
		t.Errorf("docs-writer.md is back? %v", err)
	}
}

func TestRestoreOnAnUntouchedSetupSaysSo(t *testing.T) {
	h := newHarness(t)
	h.goTo("initial")

	h.press("e")

	h.contains("Already on your original setup")
}

func TestEquipWarnsAboutModelsTheAccountCannotUse(t *testing.T) {
	h := newHarness(t)
	testutil.WriteFiles(t, h.paths.ClaudeDir, map[string]string{"settings.json": `{"model": "sonnet", "availableModels": ["sonnet"]}`})
	h.goTo("bug-hunt") // sets model haiku

	h.press("e")

	h.contains(`default model "haiku" is unavailable`)
}

func TestEquipFailureLeavesEverythingAsItWas(t *testing.T) {
	h := newHarness(t)
	testutil.WriteFiles(t, h.paths.ClaudeDir, map[string]string{"settings.json": "{"})
	h.goTo("solo-indie")

	h.press("e")

	h.contains("✗", "settings.json")
	h.press("x")
	h.contains("PRESETS")
}

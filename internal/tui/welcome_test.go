package tui_test

import (
	"testing"

	"github.com/EdrenaVosh/squadup/internal/models"
	"github.com/EdrenaVosh/squadup/internal/testutil"
	"github.com/EdrenaVosh/squadup/internal/tui"
)

// firstRun is a harness whose interface opens on the welcome screen.
func firstRun(t *testing.T) *harness {
	t.Helper()
	ws, meta := newWorkspace(t)
	for _, name := range []string{"solo-indie", "bug-hunt", "refactor-day"} {
		if _, err := ws.Remove(name, false); err != nil {
			t.Fatal(err)
		}
	}
	return harnessFor(t, ws, tui.Env{Paths: ws.Paths, Now: ws.Now, ClaudeVersion: "2.1.309", Welcome: meta})
}

func tuiEnv(h *harness) tui.Env {
	return tui.Env{Paths: h.paths, Now: h.ws.Now, ClaudeVersion: "2.1.309"}
}

func TestWelcomeAt80x24(t *testing.T) {
	h := firstRun(t)

	h.contains("Welcome to squadup", "We found your existing Claude Code setup", "saved it as ◇ Initial", "Agents", "3   docs-writer · reviewer · tester",
		"Model", "opus", "Nothing was changed.", "Start from my current setup", "Create a first preset")
	h.golden("welcome")
}

func TestWelcomeOfACleanSetup(t *testing.T) {
	clean := testutilClean(t)

	h := harnessFor(t, clean.ws, tui.Env{Paths: clean.ws.Paths, Now: clean.ws.Now, ClaudeVersion: "2.1.309", Welcome: clean.meta})

	h.contains("Clean Claude Code setup — Initial is the defaults", "Nothing was changed.")
	h.lacks("Agents ")
}

func TestStartFromMyCurrentSetup(t *testing.T) {
	h := firstRun(t)

	h.press("enter")
	h.contains("Start from my current setup", "my-setup")
	h.press("enter")

	h.contains("my-setup", "AGENTS", "SLOT 1", "Created \"my-setup\"")
	b := h.stored("my-setup")
	if len(b.Agents) != 3 || b.Preset.Settings.Model != "opus" {
		t.Fatalf("my-setup = %+v", b.Preset)
	}
	h.press("esc")
	h.contains("PRESETS", "my-setup")
	h.lacks("Welcome to squadup")
}

func TestCreateAFirstPreset(t *testing.T) {
	h := firstRun(t)

	h.press("down", "enter")
	h.typeText("first")
	h.press("enter", "down", "enter") // not the template, which is the first option: the blank preset

	h.contains("first", "AGENTS")
	h.press("esc")
	h.contains("PRESETS")
	h.lacks("Welcome to squadup")
}

func TestSkippingTheWelcomeGoesToTheMainScreen(t *testing.T) {
	h := firstRun(t)

	h.press("esc")

	h.contains("PRESETS", "initial")
	h.lacks("Welcome to squadup")
}

func TestWelcomeCanCheckTheModelsAfterAskingWhatToCheck(t *testing.T) {
	h := firstRun(t)
	testutil.FakeClaude(t)

	h.press("down", "down", "down", "enter")
	h.contains("Check your models", "The four latest", "Every release", "roughly 20k tokens")
	h.press("esc")
	h.contains("Welcome to squadup")

	h.press("enter", "enter")

	h.contains("Check your models", "Done", "fable", "opus", "sonnet", "haiku", "available", "4 of 4 available")
	h.lacks("RELEASES")
	h.goldenAt("model-check", 80, 24)
	h.press("x") // any key closes it
	h.contains("Welcome to squadup")
}

func TestEveryReleaseCanBeCheckedByItsFullID(t *testing.T) {
	h := firstRun(t)
	testutil.FakeClaude(t)
	h.resize(100, 40)

	h.press("down", "down", "down", "enter", "down", "enter")

	h.contains("RELEASES", "Sonnet 5.5", "Sonnet 4.6", "Opus 4.6", "Haiku 4.5", "Fable 5.1", "claude-opus-4-6", "15 of 15 available")
	cache, err := models.LoadCache(h.paths)
	if err != nil || cache == nil || !cache.Results["claude-opus-4-6"].OK || !cache.Results["claude-sonnet-5-5"].OK {
		t.Errorf("the answers were not kept: %+v, %v", cache, err)
	}
}

func TestTheModelCheckShowsEachModelAsItAnswers(t *testing.T) {
	h := firstRun(t)
	testutil.FakeClaude(t)
	h.press("down", "down", "down", "enter", "enter")

	// the check has finished by the time the harness returns; every model has an answer
	for _, alias := range []string{"fable", "opus", "sonnet", "haiku"} {
		h.contains(alias)
	}
	h.lacks("Asking Claude about each model")
}

func TestTheMainScreenCanCheckTheModelsToo(t *testing.T) {
	h := newHarness(t)
	testutil.FakeClaude(t)

	h.press("m")
	h.contains("Check your models", "The four latest")
	h.press("enter")

	h.contains("Done", "4 of 4 available")
}

func TestWelcomeSaysWhatIsMissingForTheModelCheck(t *testing.T) {
	h := firstRun(t) // no claude on PATH

	h.press("down", "down", "down", "enter", "enter")

	h.contains("claude was not found", "Nothing was sent")
}

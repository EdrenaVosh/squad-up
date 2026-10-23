package tui_test

import (
	"testing"

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
	h.press("enter", "enter")

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

func TestWelcomeCanCheckTheModelsAfterConsent(t *testing.T) {
	h := firstRun(t)
	testutil.FakeClaude(t)

	h.press("down", "down", "enter")
	h.contains("Check your models", "one tiny request", "a few tokens")
	h.press("n")
	h.contains("Welcome to squadup")

	h.press("enter", "y")

	h.contains("Models checked")
}

func TestWelcomeSaysWhatIsMissingForTheModelCheck(t *testing.T) {
	h := firstRun(t) // no claude on PATH

	h.press("down", "down", "enter", "y")

	h.contains("claude was not found")
}

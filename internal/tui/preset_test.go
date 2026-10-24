package tui_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/EdrenaVosh/squadup/internal/preset"
	"github.com/EdrenaVosh/squadup/internal/testutil"
)

// open selects a preset on the main screen and opens it.
func (h *harness) open(name string) {
	h.t.Helper()
	h.goTo(name)
	h.press("enter")
}

// stored reads the preset back from disk.
func (h *harness) stored(name string) *preset.Bundle {
	h.t.Helper()
	b, err := h.ws.Store().Load(name)
	if err != nil {
		h.t.Fatalf("Load(%s): %v", name, err)
	}
	return b
}

func TestPresetScreenAt80x24(t *testing.T) {
	h := newHarness(t)

	h.open("solo-indie")

	h.contains("Solo Indie", "AGENTS", "SLOT 1", "planner", "add agent", "SETTINGS", "Default model", "sonnet", "LINT")
	h.golden("preset")
}

func TestInitialOpensReadOnly(t *testing.T) {
	h := newHarness(t)

	h.open("initial")

	h.contains("Initial", "docs-writer", "reviewer")
	h.lacks("add agent")
	h.golden("preset-initial")
	h.press("down", "a", "x", "enter")
	h.lacks("New agent", "Remove agent")
	h.press("c")
	h.contains("Clone preset", "my-setup")
}

func TestChangingTheDefaultModelSavesAtOnce(t *testing.T) {
	h := newHarness(t)
	h.open("solo-indie")

	h.press("down", "down", "down", "down") // 3 agents, add agent, Default model
	h.press("enter")
	h.contains("Default model", "Not set", "opus", "haiku")
	h.golden("preset-model")
	h.press("up", "enter") // sonnet -> opus

	if got := h.stored("solo-indie").Preset.Settings.Model; got != "opus" {
		t.Fatalf("model = %q, want opus", got)
	}
	h.contains("opus")
}

func TestUnsetModelClearsTheSetting(t *testing.T) {
	h := newHarness(t)
	h.open("solo-indie")

	h.press("down", "down", "down", "down", "enter", "up", "up", "up", "up", "enter")

	if got := h.stored("solo-indie").Preset.Settings.Model; got != "" {
		t.Fatalf("model = %q, want it cleared", got)
	}
}

func TestEffortModeAndMainAgentAreChosenFromLists(t *testing.T) {
	h := newHarness(t)
	h.open("solo-indie")

	h.press("down", "down", "down", "down", "down") // Effort
	h.press("enter", "up", "enter")                 // high -> medium
	if got := h.stored("solo-indie").Preset.Settings.EffortLevel; got != "medium" {
		t.Errorf("effort = %q, want medium", got)
	}

	h.press("down", "down")           // Permissions, Main agent
	h.press("enter", "down", "enter") // None -> planner
	b := h.stored("solo-indie")
	if b.Preset.Main == nil || *b.Preset.Main != "planner" {
		t.Errorf("main = %v, want planner", b.Preset.Main)
	}

	h.press("down", "down")           // Team flow, Mode
	h.press("enter", "down", "enter") // replace -> extend
	if got := h.stored("solo-indie").Preset.Mode; got != preset.ModeExtend {
		t.Errorf("mode = %q, want extend", got)
	}
	h.contains("extend")
}

func TestUnavailableModelsAreGreyedOutWithTheReason(t *testing.T) {
	h := newHarness(t)
	testutil.WriteFiles(t, h.paths.ClaudeDir, map[string]string{"settings.json": `{"availableModels": ["sonnet", "haiku"]}`})
	h.open("solo-indie")

	h.press("down", "down", "down", "down", "enter")

	h.contains("opus", "blocked by policy")
	h.press("up", "up", "enter") // sonnet -> fable, which is blocked
	h.contains("is unavailable")
	h.contains("Default model")
	if got := h.stored("solo-indie").Preset.Settings.Model; got != "sonnet" {
		t.Errorf("model = %q: an unavailable model must not be picked", got)
	}
}

func TestAddingAnAgentCreatesItAndOpensTheEditor(t *testing.T) {
	h := newHarness(t)
	h.open("solo-indie")

	h.press("a")
	h.contains("New agent")
	h.typeText("scribe")
	h.press("enter")

	h.contains("Agent scribe", "Describe what this agent does")
	b := h.stored("solo-indie")
	if len(b.Agents) != 4 || b.Agents[3].Name != "scribe" || b.Preset.Agents[3] != "scribe" {
		t.Fatalf("agents = %v", b.Preset.Agents)
	}
	h.press("esc")
	h.contains("SLOT 4", "scribe")
}

func TestNewAgentNamesAreChecked(t *testing.T) {
	h := newHarness(t)
	h.open("solo-indie")

	h.press("a")
	h.typeText("Bad Name")
	h.press("enter")
	h.contains("agent names use lowercase")
	h.clear(8)
	h.typeText("planner")
	h.press("enter")
	h.contains("already has an agent named")
}

func TestRemovingAnAgentAsksFirst(t *testing.T) {
	h := newHarness(t)
	h.open("bug-hunt")

	h.press("x")
	h.contains("Remove agent", `"debugger"`)
	h.press("n")
	if len(h.stored("bug-hunt").Agents) != 2 {
		t.Fatal("cancelling must keep the agent")
	}
	h.press("x", "y")

	b := h.stored("bug-hunt")
	if len(b.Agents) != 1 || b.Agents[0].Name != "tester" {
		t.Fatalf("agents = %v", b.Preset.Agents)
	}
	if b.Preset.Main != nil {
		t.Errorf("the removed agent was the main one: main = %v", *b.Preset.Main)
	}
	if _, err := os.Stat(filepath.Join(h.paths.PresetDir("bug-hunt"), "agents", "debugger.md")); !os.IsNotExist(err) {
		t.Error("the agent file is still there")
	}
}

func TestAgentsCanBeReordered(t *testing.T) {
	h := newHarness(t)
	h.open("solo-indie")

	h.press("shift+down")
	if got := h.stored("solo-indie").Preset.Agents; got[0] != "reviewer" || got[1] != "planner" {
		t.Fatalf("agents = %v", got)
	}
	h.contains("SLOT 1  ■ reviewer")
	h.press("K")
	if got := h.stored("solo-indie").Preset.Agents; got[0] != "planner" {
		t.Fatalf("agents = %v", got)
	}
	h.press("shift+up")
	if got := h.stored("solo-indie").Preset.Agents; got[0] != "planner" {
		t.Errorf("moving the first agent up must do nothing: %v", got)
	}
}

func TestPresetShowsWhatTheLinterFound(t *testing.T) {
	h := newHarness(t)
	testutil.WriteFiles(t, h.paths.PresetDir("bug-hunt"), map[string]string{
		"agents/tester.md": agentFile("tester", "Short.", "tools: Read\nmodel: gpt-5\n", "Body"),
	})

	h.open("bug-hunt")

	h.contains("1 error", "✗ model is gpt-5")
}

func TestABrokenPresetShowsTheProblemInsteadOfFailing(t *testing.T) {
	h := newHarness(t)
	testutil.WriteFiles(t, h.paths.PresetDir("bug-hunt"), map[string]string{"preset.yaml": "schema: [oops"})

	h.open("bug-hunt")

	h.contains("✗")
	h.fits(80, 24)
	h.press("esc")
	h.contains("PRESETS")
}

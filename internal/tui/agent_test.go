package tui_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/EdrenaVosh/squad-up/internal/agentfile"
	"github.com/EdrenaVosh/squad-up/internal/testutil"
)

// openAgent opens the n-th agent (0-based) of a preset in its editor.
func (h *harness) openAgent(preset string, n int) {
	h.t.Helper()
	h.open(preset)
	for i := 0; i < n; i++ {
		h.press("down")
	}
	h.press("enter")
}

// agent reads an agent of a stored preset.
func (h *harness) agent(preset, name string) *agentfile.Agent {
	h.t.Helper()
	data, err := os.ReadFile(filepath.Join(h.paths.PresetDir(preset), "agents", name+".md"))
	if err != nil {
		h.t.Fatal(err)
	}
	a, err := agentfile.Parse(data)
	if err != nil {
		h.t.Fatalf("%s.md no longer parses: %v\n%s", name, err, data)
	}
	return a
}

// field moves the cursor of the agent editor to the n-th field (0-based).
func (h *harness) field(n int) {
	h.t.Helper()
	for i := 0; i < 12; i++ {
		h.press("up")
	}
	for i := 0; i < n; i++ {
		h.press("down")
	}
}

const (
	fName = iota
	fDescription
	fIndependent
	fModel
	fEffort
	fPermission
	fTools
	fDisallowed
	fMaxTurns
	fColor
	fPrompt
)

func TestAgentEditorAt80x24(t *testing.T) {
	h := newHarness(t)

	h.openAgent("solo-indie", 0)

	h.contains("Agent planner", "Name", "planner", "Model", "opus", "Permission mode", "plan", "CHECKS", "nothing to fix")
	h.golden("agent")
}

func TestAgentEditorOnAWideTerminalIsAWorkbench(t *testing.T) {
	h := newHarness(t)
	h.resize(140, 36)

	h.openAgent("solo-indie", 0)

	h.contains("FIELDS", "Agent planner", "Breaks a task into steps", "Tools", "‹Read›", "CHECKS", "nothing to fix")
	h.contains("Name", "The name Claude sees") // the inspector explains the selected field
	h.fits(140, 36)
	h.goldenAt("agent-wide", 140, 36)

	h.field(fDescription)
	h.contains("Claude delegates by reading this", "it says what the agent does")
	h.field(fModel)
	h.contains("haiku", "inherit uses the model of the session")
	h.field(fEffort)
	h.contains("fastest and cheapest", "careful; for reviews and planning")
	h.field(fPermission)
	h.contains("read-only: plans, never edits", "asks for nothing")
	h.field(fTools)
	h.contains("checked", "Bash", "WebFetch")
	h.field(fColor)
	h.contains("cyan", "The color of the agent in Claude Code")
	h.press("p")
	h.contains("File preview", "name: planner", "model: opus")
	h.press("p")
	h.lacks("File preview")
}

func TestThePromptIsShownInFullWithItsActions(t *testing.T) {
	h := newHarness(t)
	h.resize(140, 36)
	h.openAgent("solo-indie", 0)

	h.field(fPrompt)

	h.contains("write here", "open in $EDITOR", "load from a file", "1 line", "Plan it.")
}

func writePromptFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "prompt.md")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestThePromptCanBeLoadedFromAMarkdownFile(t *testing.T) {
	h := newHarness(t)
	h.resize(140, 36)
	h.openAgent("solo-indie", 0)
	h.field(fPrompt)
	file := writePromptFile(t, "## Role\nYou plan carefully.\n\n## Response format\n- a plan\n")

	h.press("f")
	h.contains("Load the prompt from a file", "Drop the file")
	h.typeText("/no/such/file.md")
	h.press("enter")
	h.contains("does not exist")
	h.clear(len("/no/such/file.md"))
	h.typeText(file)
	h.press("enter")

	h.contains("Replace the prompt (1 line) with", "prompt.md (5 lines)")
	h.press("n")
	if got := h.agent("solo-indie", "planner").Prompt(); got != "Plan it.\n" {
		t.Fatalf("declining changed the prompt: %q", got)
	}

	h.press("f")
	h.typeText(file)
	h.press("enter", "y")
	h.contains("You plan carefully.", "Response format", "5 lines")
	if got := h.agent("solo-indie", "planner").Prompt(); !strings.Contains(got, "## Response format") || !strings.Contains(got, "- a plan") {
		t.Errorf("the prompt was not stored: %q", got)
	}
	if h.agent("solo-indie", "planner").Name() != "planner" {
		t.Error("the frontmatter was touched")
	}
}

func TestThePromptCanBeWrittenInTheBigEditor(t *testing.T) {
	h := newHarness(t)
	h.resize(140, 36)
	h.openAgent("solo-indie", 0)
	h.field(fPrompt)

	h.press("enter")
	h.contains("Prompt · planner", "saved", "ctrl+s")
	h.typeText(" More.")
	h.contains("unsaved")
	h.press("ctrl+s")

	h.contains("Prompt saved", "More.")
	if got := h.agent("solo-indie", "planner").Prompt(); !strings.Contains(got, "Plan it. More.") {
		t.Errorf("prompt = %q", got)
	}
}

func TestLeavingTheBigEditorWithChangesAsksFirst(t *testing.T) {
	h := newHarness(t)
	h.resize(140, 36)
	h.openAgent("solo-indie", 0)
	h.field(fPrompt)
	h.press("enter")
	h.typeText("xyz")

	h.press("esc")
	h.contains("Discard your changes?")
	h.press("n")
	h.contains("unsaved")
	h.press("esc", "y")

	h.contains("Agent planner")
	if got := h.agent("solo-indie", "planner").Prompt(); strings.Contains(got, "xyz") {
		t.Errorf("discarded text was saved: %q", got)
	}

	h.press("enter", "esc") // nothing changed: no question
	h.contains("Agent planner")
	h.lacks("Discard")
}

func TestAFileCanBeLoadedIntoTheBigEditorWithoutSavingIt(t *testing.T) {
	h := newHarness(t)
	h.resize(140, 36)
	h.openAgent("solo-indie", 0)
	h.field(fPrompt)
	h.press("enter")
	file := writePromptFile(t, "From a file.\n")

	h.press("ctrl+l")
	h.typeText(file)
	h.press("enter")

	h.contains("From a file.", "unsaved")
	if got := h.agent("solo-indie", "planner").Prompt(); got != "Plan it.\n" {
		t.Errorf("the file was saved before ctrl+s: %q", got)
	}
	h.press("ctrl+s")
	if got := h.agent("solo-indie", "planner").Prompt(); got != "From a file.\n" {
		t.Errorf("prompt = %q", got)
	}
}

func TestALongPromptScrolls(t *testing.T) {
	h := newHarness(t)
	h.resize(140, 30)
	h.openAgent("solo-indie", 0)
	h.field(fPrompt)
	var long strings.Builder
	for i := 1; i <= 80; i++ {
		long.WriteString("Line number " + strings.Repeat("x", i%7) + "\n")
	}
	file := writePromptFile(t, strings.ReplaceAll(long.String(), "x", "y"))
	h.press("f")
	h.typeText(file)
	h.press("enter", "y")

	h.contains("lines 1–", "of 83", "pgup/pgdn scroll")
	h.press("pgdown")
	h.lacks("lines 1–")
	h.contains("Line number")
	h.press("pgup")
	h.contains("lines 1–")
}

func TestPreviewTogglesOnNarrowTerminals(t *testing.T) {
	h := newHarness(t)
	h.openAgent("solo-indie", 0)
	h.lacks("PREVIEW")

	h.press("p")

	h.contains("PREVIEW — planner.md", "---", "name: planner", "description: Breaks a task")
	h.golden("agent-preview")
	h.press("p")
	h.lacks("PREVIEW")
}

func TestRenamingAnAgentUpdatesThePresetAndTheMainAgent(t *testing.T) {
	h := newHarness(t)
	h.openAgent("bug-hunt", 0) // debugger, the main agent

	h.press("enter")
	h.contains("Agent name", "debugger")
	h.clear(len("debugger"))
	h.typeText("detective")
	h.press("enter")

	b := h.stored("bug-hunt")
	if b.Preset.Agents[0] != "detective" || b.Agents[0].Name != "detective" || b.Preset.Main == nil || *b.Preset.Main != "detective" {
		t.Fatalf("agents = %v, main = %v", b.Preset.Agents, b.Preset.Main)
	}
	if h.agent("bug-hunt", "detective").Name() != "detective" {
		t.Error("the name inside the file was not changed")
	}
	h.contains("Agent detective")
}

func TestAgentNamesAreCheckedInTheEditor(t *testing.T) {
	h := newHarness(t)
	h.openAgent("bug-hunt", 0)

	h.press("enter")
	h.clear(8)
	h.typeText("tester")
	h.press("enter")
	h.contains("already has an agent named")
	h.clear(6)
	h.typeText("Bad Name")
	h.press("enter")
	h.contains("agent names use lowercase")
}

func TestEditingTheDescriptionInATextArea(t *testing.T) {
	h := newHarness(t)
	h.openAgent("solo-indie", 0)
	h.field(fDescription)

	h.press("enter")
	h.contains("Description — what it does")
	h.typeText(" Call it first.")
	h.press("ctrl+s")

	if got := h.agent("solo-indie", "planner").Description(); !strings.HasSuffix(got, "Call it first.") {
		t.Fatalf("description = %q", got)
	}
	h.contains("Agent planner")
}

func TestChoosingTheModelOfAnAgent(t *testing.T) {
	h := newHarness(t)
	h.openAgent("solo-indie", 1) // reviewer: sonnet
	h.field(fModel)

	h.press("enter")
	h.contains("Model", "inherit", "fable", "haiku", "Sonnet 5.5", "claude-sonnet-5-5 · not checked")
	h.press("down", "enter") // sonnet -> haiku (inherit, fable, opus, sonnet, haiku)

	if v, _ := h.agent("solo-indie", "reviewer").Get("model"); v != "haiku" {
		t.Fatalf("model = %v, want haiku", v)
	}

	h.field(fModel)
	h.press("enter", "up", "up", "up", "up", "enter") // back to inherit
	if _, ok := h.agent("solo-indie", "reviewer").Get("model"); ok {
		t.Error("inherit must remove the field")
	}
}

func TestAnAgentCanUseAFullModelID(t *testing.T) {
	h := newHarness(t)
	h.openAgent("solo-indie", 1)
	h.field(fModel)

	h.press("enter", "up", "up", "up", "up", "up")
	h.press("enter") // wraps? stays on the first entry: inherit
	h.field(fModel)
	h.press("enter")
	for i := 0; i < 40; i++ {
		h.press("down") // to the last entry
	}
	h.contains("Other model ID…")
	h.press("enter")
	h.contains("Model ID")
	h.typeText("not a model")
	h.press("enter")
	h.contains("does not look like a model ID")
	h.clear(11)
	h.typeText("claude-sonnet-4-5")
	h.press("enter")

	if v, _ := h.agent("solo-indie", "reviewer").Get("model"); v != "claude-sonnet-4-5" {
		t.Fatalf("model = %v", v)
	}
}

func TestAnAgentCanUseASpecificRelease(t *testing.T) {
	h := newHarness(t)
	h.resize(140, 36)
	h.openAgent("solo-indie", 1) // reviewer: sonnet
	h.field(fModel)

	h.press("enter")
	h.contains("Sonnet 4.6", "claude-sonnet-4-6", "Opus 4.6", "Sonnet 5 ", "Haiku 4.5")
	h.press("down", "down", "down", "down", "down", "down", "down", "down", "down") // sonnet -> Sonnet 4.6
	h.press("enter")

	if v, _ := h.agent("solo-indie", "reviewer").Get("model"); v != "claude-sonnet-4-6" {
		t.Fatalf("model = %v, want claude-sonnet-4-6", v)
	}
	h.contains("sonnet 4.6") // the field says it the way a person does, not as the long ID
}

func TestEffortAndPermissionModeAreChosenFromLists(t *testing.T) {
	h := newHarness(t)
	h.openAgent("solo-indie", 1)

	h.field(fEffort)
	h.press("enter", "down", "enter") // high -> xhigh? list: Not set, low, medium, high, xhigh, max
	if v, _ := h.agent("solo-indie", "reviewer").Get("effort"); v != "xhigh" {
		t.Errorf("effort = %v, want xhigh", v)
	}

	h.field(fPermission)
	h.press("enter")
	h.contains("Permission mode", "bypassPermissions", "asks for nothing")
	h.press("down", "enter") // default -> acceptEdits
	if v, _ := h.agent("solo-indie", "reviewer").Get("permissionMode"); v != "acceptEdits" {
		t.Errorf("permissionMode = %v", v)
	}
}

func TestToolsAreChosenWithCheckboxes(t *testing.T) {
	h := newHarness(t)
	h.openAgent("solo-indie", 1) // tools: Read, Grep

	h.field(fTools)
	h.press("enter")
	h.contains("Tools", "[x] Read", "[ ] Write", "[x] Grep")
	h.golden("agent-tools")
	h.press("space")                          // Read off
	h.press("down", "space")                  // Write on
	h.press("down", "down", "space", "enter") // Edit, Bash on

	if v, _ := h.agent("solo-indie", "reviewer").Get("tools"); v != "Write, Bash, Grep" {
		t.Fatalf("tools = %v", v)
	}
}

func TestToolsThatAreNotBuiltInStayChecked(t *testing.T) {
	h := newHarness(t)
	testutil.WriteFiles(t, h.paths.PresetDir("solo-indie"), map[string]string{
		"agents/reviewer.md": agentFile("reviewer", "Reviews diffs for bugs. Use after any code change, before commit.", "tools: Read, mcp__github__get_file_contents\n", "Review it."),
	})
	h.openAgent("solo-indie", 1)

	h.field(fTools)
	h.press("enter")

	h.contains("[x] mcp__github__get_file_contents")
	h.press("enter")
	if v, _ := h.agent("solo-indie", "reviewer").Get("tools"); !strings.Contains(v.(string), "mcp__github__get_file_contents") {
		t.Errorf("the MCP tool was lost: %v", v)
	}
}

func TestDisallowedToolsAndColor(t *testing.T) {
	h := newHarness(t)
	h.openAgent("solo-indie", 1)

	h.field(fDisallowed)
	h.press("enter", "down", "down", "down", "space", "enter") // Bash
	if v, _ := h.agent("solo-indie", "reviewer").Get("disallowedTools"); v != "Bash" {
		t.Errorf("disallowedTools = %v", v)
	}

	h.field(fColor)
	h.press("enter")
	h.contains("Color", "red", "cyan")
	h.press("down", "down", "down", "enter") // green -> ... Not set, red, blue, green, yellow: from green(3) down 3 = pink?
	if v, ok := h.agent("solo-indie", "reviewer").Get("color"); !ok || v == "green" {
		t.Errorf("color = %v: it should have changed", v)
	}
}

func TestMaxTurnsMustBeAPositiveNumber(t *testing.T) {
	h := newHarness(t)
	h.openAgent("solo-indie", 0)
	h.field(fMaxTurns)

	h.press("enter")
	h.typeText("abc")
	h.press("enter")
	h.contains("whole number of 1 or more")
	h.clear(3)
	h.typeText("12")
	h.press("enter")

	if v, _ := h.agent("solo-indie", "planner").Get("maxTurns"); v != uint64(12) && v != int64(12) {
		t.Fatalf("maxTurns = %#v", v)
	}
	h.field(fMaxTurns)
	h.press("enter")
	h.clear(2)
	h.press("enter")
	if _, ok := h.agent("solo-indie", "planner").Get("maxTurns"); ok {
		t.Error("an empty value must remove the field")
	}
}

func TestChangesKeepUnknownFieldsAndOrderTheFileCanonically(t *testing.T) {
	h := newHarness(t)
	testutil.WriteFiles(t, h.paths.PresetDir("solo-indie"), map[string]string{
		"agents/planner.md": "---\ncolor: cyan\nname: planner\ndescription: Breaks a task into steps. Use before starting any large change.\nmy_custom: keep me\n---\nPlan it.\n",
	})
	h.openAgent("solo-indie", 0)

	h.field(fModel)
	h.press("enter", "down", "enter") // inherit -> fable

	data, _ := os.ReadFile(filepath.Join(h.paths.PresetDir("solo-indie"), "agents", "planner.md"))
	want := "---\nname: planner\ndescription: Breaks a task into steps. Use before starting any large change.\nmodel: fable\ncolor: cyan\nmy_custom: keep me\n---\nPlan it.\n"
	if string(data) != want {
		t.Fatalf("planner.md = %q\nwant %q", data, want)
	}
}

func TestTheEditorShowsLintHintsForTheAgent(t *testing.T) {
	h := newHarness(t)
	testutil.WriteFiles(t, h.paths.PresetDir("solo-indie"), map[string]string{
		"agents/planner.md": agentFile("planner", "Plans.", "tools: Read\nbogus_field: 1\n", "Plan it."),
	})

	h.openAgent("solo-indie", 0)

	h.contains("CHECKS", "shorter than 40 characters", `unknown field "bogus_field"`)
	h.fits(80, 24)
}

func TestAnUnreadableAgentFileExplainsHowToFixIt(t *testing.T) {
	h := newHarness(t)
	testutil.WriteFiles(t, h.paths.PresetDir("solo-indie"), map[string]string{"agents/planner.md": "no frontmatter"})

	h.openAgent("solo-indie", 0)

	h.contains("✗", "squad edit solo-indie planner --raw")
	h.press("down", "enter")
	h.press("esc")
	h.contains("Solo Indie")
}

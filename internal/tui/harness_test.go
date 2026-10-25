package tui_test

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/EdrenaVosh/squad-up/internal/apply"
	"github.com/EdrenaVosh/squad-up/internal/initial"
	"github.com/EdrenaVosh/squad-up/internal/paths"
	"github.com/EdrenaVosh/squad-up/internal/preset"
	"github.com/EdrenaVosh/squad-up/internal/testutil"
	"github.com/EdrenaVosh/squad-up/internal/tui"
	"github.com/EdrenaVosh/squad-up/internal/workspace"
)

var update = flag.Bool("screens", false, "rewrite the golden screens in testdata")

var fixedNow = time.Date(2026, 10, 24, 9, 30, 0, 0, time.UTC)

func agentFile(name, desc, extra, body string) string {
	return "---\nname: " + name + "\ndescription: " + desc + "\n" + extra + "---\n" + body + "\n"
}

// harness runs the interface without a terminal: keys go straight into the
// model, and the commands it returns are run unless they would block.
type harness struct {
	t     *testing.T
	m     tea.Model
	ws    workspace.Workspace
	paths paths.Paths
	quit  bool
}

// newWorkspace builds a foreign Claude Code setup with Initial captured and
// three stored presets. Nothing is equipped.
func newWorkspace(t *testing.T) (workspace.Workspace, *initial.Meta) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)        // so that paths under it are shown as ~/…, the same on every machine
	t.Setenv("USERPROFILE", home) // the same for Windows, which does not read HOME
	p := paths.Paths{ClaudeDir: filepath.Join(t.TempDir(), "claude"), Home: filepath.Join(home, "squad-up")}
	testutil.WriteFiles(t, p.ClaudeDir, map[string]string{
		"agents/reviewer.md":    agentFile("reviewer", "Reviews diffs for bugs. Use after any code change.", "tools: Read, Grep\ncolor: green\n", "Be strict."),
		"agents/team/tester.md": agentFile("tester", "Writes tests. Use when coverage is missing.", "tools: Read, Bash\n", "Test it."),
		"agents/docs-writer.md": agentFile("docs-writer", "Updates docs. Use after a user-facing change.", "tools: Read, Edit\ncolor: cyan\n", "Keep it short."),
		"settings.json":         "{\n  \"theme\": \"dark\",\n  \"model\": \"opus\",\n  \"effortLevel\": \"high\",\n  \"permissions\": {\n    \"defaultMode\": \"acceptEdits\",\n    \"allow\": [\"Bash(git status)\"]\n  }\n}\n",
	})
	meta, err := initial.Capture(p, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	ws := workspace.Workspace{Paths: p, Now: func() time.Time { return fixedNow }}

	debugger := "debugger"
	for _, b := range []preset.Bundle{
		{
			Preset: preset.Preset{
				Schema: 1, Name: "solo-indie", Title: "Solo Indie", Description: "Planner, reviewer and tester for one-person apps",
				Accent: "cyan", Mode: preset.ModeReplace, Agents: []string{"planner", "reviewer", "tester"},
				Settings: preset.Settings{Model: "sonnet", EffortLevel: "high", Permissions: &preset.Permissions{DefaultMode: "acceptEdits", Ask: []string{"Bash(git push *)"}}},
			},
			Agents: []preset.AgentFile{
				{Name: "planner", Data: []byte(agentFile("planner", "Breaks a task into steps. Use before starting any large change.", "tools: Read, Grep, Glob\nmodel: opus\neffort: high\npermissionMode: plan\ncolor: cyan\n", "Plan it."))},
				{Name: "reviewer", Data: []byte(agentFile("reviewer", "Reviews diffs for bugs. Use after any code change, before commit.", "tools: Read, Grep\nmodel: sonnet\neffort: high\ncolor: green\n", "Review it."))},
				{Name: "tester", Data: []byte(agentFile("tester", "Writes and runs tests. Use when a change needs coverage.", "tools: Read, Bash\nmodel: haiku\neffort: medium\npermissionMode: acceptEdits\ncolor: yellow\n", "Test it."))},
			},
		},
		{
			Preset: preset.Preset{
				Schema: 1, Name: "bug-hunt", Title: "Bug Hunt", Description: "A debugger and a tester for tracking down a failure",
				Mode: preset.ModeReplace, Main: &debugger, Agents: []string{"debugger", "tester"}, Settings: preset.Settings{Model: "haiku"},
			},
			Agents: []preset.AgentFile{
				{Name: "debugger", Data: []byte(agentFile("debugger", "Finds the root cause of a failure. Use when a test or the app fails.", "tools: Read, Bash\ncolor: red\n", "Find it."))},
				{Name: "tester", Data: []byte(agentFile("tester", "Writes a failing test for a bug. Use before fixing it.", "tools: Read, Bash\ncolor: yellow\n", "Reproduce it."))},
			},
		},
		{
			Preset: preset.Preset{
				Schema: 1, Name: "refactor-day", Title: "Refactor Day", Description: "Keeps your agents and adds an architect",
				Mode: preset.ModeExtend, Agents: []string{"architect"},
			},
			Agents: []preset.AgentFile{
				{Name: "architect", Data: []byte(agentFile("architect", "Designs the structure of a change. Use before a large refactor.", "tools: Read, Grep\ncolor: purple\n", "Design it."))},
			},
		},
	} {
		if err := ws.Store().Save(b); err != nil {
			t.Fatal(err)
		}
	}
	return ws, meta
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	ws, _ := newWorkspace(t)
	return harnessFor(t, ws, tui.Env{Paths: ws.Paths, Now: ws.Now, ClaudeVersion: "2.1.309"})
}

func harnessFor(t *testing.T, ws workspace.Workspace, env tui.Env) *harness {
	t.Helper()
	testutil.HideClaude(t)
	h := &harness{t: t, m: tui.New(env), ws: ws, paths: ws.Paths}
	h.send(tea.WindowSizeMsg{Width: 80, Height: 24})
	h.run(h.m.Init())
	return h
}

func (h *harness) resize(w, ht int) { h.send(tea.WindowSizeMsg{Width: w, Height: ht}) }

// send delivers a message and runs what comes back.
func (h *harness) send(msg tea.Msg) {
	h.t.Helper()
	if _, ok := msg.(tea.QuitMsg); ok {
		h.quit = true
		return
	}
	m, cmd := h.m.Update(msg)
	h.m = m
	h.run(cmd)
}

// run executes a command and feeds its message back. A command that does not
// finish quickly (a cursor blink, an external editor) is left alone.
func (h *harness) run(cmd tea.Cmd) {
	h.t.Helper()
	if cmd == nil {
		return
	}
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	select {
	case msg := <-done:
		switch m := msg.(type) {
		case nil:
		case tea.BatchMsg:
			for _, c := range m {
				h.run(c)
			}
		default:
			h.send(msg)
		}
	case <-time.After(2 * time.Second):
	}
}

// press sends keys by name: "enter", "esc", "down", "shift+up", "ctrl+c", or a
// single character.
func (h *harness) press(keys ...string) {
	h.t.Helper()
	for _, k := range keys {
		h.send(keyMsg(k))
	}
}

// typeText sends each character as a key press.
func (h *harness) typeText(s string) {
	h.t.Helper()
	for _, r := range s {
		h.send(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

func keyMsg(k string) tea.KeyPressMsg {
	switch k {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	case "space":
		return tea.KeyPressMsg{Code: ' ', Text: " "}
	case "shift+up":
		return tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModShift}
	case "shift+down":
		return tea.KeyPressMsg{Code: tea.KeyDown, Mod: tea.ModShift}
	case "ctrl+c":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	case "ctrl+s":
		return tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}
	case "ctrl+l":
		return tea.KeyPressMsg{Code: 'l', Mod: tea.ModCtrl}
	case "pgdown":
		return tea.KeyPressMsg{Code: tea.KeyPgDown}
	case "pgup":
		return tea.KeyPressMsg{Code: tea.KeyPgUp}
	}
	r := []rune(k)
	if len(r) != 1 {
		panic("unknown key " + k)
	}
	return tea.KeyPressMsg{Code: r[0], Text: k}
}

// view is the screen without styling.
func (h *harness) view() string {
	h.t.Helper()
	return ansi.Strip(h.m.View().Content)
}

// contains fails the test unless the screen shows all the texts.
func (h *harness) contains(texts ...string) {
	h.t.Helper()
	v := h.view()
	for _, want := range texts {
		if !strings.Contains(v, want) {
			h.t.Errorf("the screen lacks %q:\n%s", want, v)
		}
	}
}

// lacks fails the test if the screen shows any of the texts.
func (h *harness) lacks(texts ...string) {
	h.t.Helper()
	v := h.view()
	for _, bad := range texts {
		if strings.Contains(v, bad) {
			h.t.Errorf("the screen should not show %q:\n%s", bad, v)
		}
	}
}

// fits checks that the screen is exactly w×h cells.
func (h *harness) fits(w, ht int) {
	h.t.Helper()
	lines := strings.Split(h.view(), "\n")
	if len(lines) != ht {
		h.t.Errorf("the screen has %d lines, want %d:\n%s", len(lines), ht, h.view())
	}
	for i, l := range lines {
		if width := ansi.StringWidth(l); width > w {
			h.t.Errorf("line %d is %d cells wide, want at most %d: %q", i+1, width, w, l)
		}
	}
}

// golden compares the screen with testdata/<name>.golden.
func (h *harness) golden(name string) {
	h.t.Helper()
	h.goldenAt(name, 80, 24)
}

// goldenAt is golden for a terminal of another size.
func (h *harness) goldenAt(name string, w, ht int) {
	h.t.Helper()
	h.fits(w, ht)
	path := filepath.Join("testdata", name+".golden")
	got := h.view()
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			h.t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		h.t.Fatalf("no golden screen %s (run the tests with -screens): %v", path, err)
	}
	// a Git checkout on Windows may turn the file's line ends into CRLF, and
	// a path under the home folder is shown with the separator of the system
	if slashed(got) != strings.ReplaceAll(string(want), "\r\n", "\n") {
		h.t.Errorf("the screen differs from %s (run the tests with -screens to accept)\n--- got ---\n%s\n--- want ---\n%s", path, got, want)
	}
}

// homePath matches a path shown under the home folder: ~/… or ~\… on Windows.
var homePath = regexp.MustCompile(`~[^\s│]*`)

// slashed turns the backslashes of the paths under ~ into slashes, so that one
// golden screen serves every system.
func slashed(screen string) string {
	return homePath.ReplaceAllStringFunc(screen, func(p string) string { return strings.ReplaceAll(p, `\`, "/") })
}

// selectedRow matches the highlighted line of the preset list.
var selectedRow = regexp.MustCompile(`▌ \S [^│\n]*`)

// goTo moves the selection of the main screen to the named preset.
func (h *harness) goTo(name string) {
	h.t.Helper()
	for i := 0; i < 12; i++ {
		v := h.view()
		// The wide layout shows the title ("Solo Indie"), the narrow one the name.
		line := strings.ReplaceAll(strings.ToLower(selectedRow.FindString(v)), " ", "-")
		if selectedRow.MatchString(v) && strings.Contains(line, "-"+name) {
			return
		}
		h.press("down")
	}
	h.t.Fatalf("could not select %q:\n%s", name, h.view())
}

// equipped returns a harness whose Claude config already has the preset equipped.
func equipped(t *testing.T, name string) *harness {
	t.Helper()
	ws, _ := newWorkspace(t)
	plan, err := ws.Engine().PlanUse(name, applyOptions())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ws.Engine().Execute(plan, applyOptions()); err != nil {
		t.Fatal(err)
	}
	return harnessFor(t, ws, tui.Env{Paths: ws.Paths, Now: ws.Now, ClaudeVersion: "2.1.309"})
}

// clear deletes what is in a text field by pressing backspace n times.
func (h *harness) clear(n int) {
	h.t.Helper()
	for i := 0; i < n; i++ {
		h.press("backspace")
	}
}

func applyOptions() apply.Options { return apply.Options{} }

type cleanSetup struct {
	ws   workspace.Workspace
	meta *initial.Meta
}

// testutilClean builds a workspace whose Claude config has nothing in it.
func testutilClean(t *testing.T) cleanSetup {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)        // so that paths under it are shown as ~/…, the same on every machine
	t.Setenv("USERPROFILE", home) // the same for Windows, which does not read HOME
	p := paths.Paths{ClaudeDir: filepath.Join(t.TempDir(), "claude"), Home: filepath.Join(home, "squad-up")}
	meta, err := initial.Capture(p, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	return cleanSetup{ws: workspace.Workspace{Paths: p, Now: func() time.Time { return fixedNow }}, meta: meta}
}

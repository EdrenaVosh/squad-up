package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFlowShowsSetsAndRemovesTheLoop(t *testing.T) {
	e := newEnv(t)
	e.addPresets()

	if r := e.run("flow", "solo"); r.code != 0 || !strings.Contains(r.stdout, "no flow") {
		t.Fatalf("show: %d %q", r.code, r.stdout)
	}

	r := e.run("flow", "solo", "--gate", "reviewer", "--fix", "planner", "--rounds", "2")
	if r.code != 0 {
		t.Fatalf("set: %d %s", r.code, r.stderr)
	}
	for _, want := range []string{"planner → reviewer → tester", "reviewer checks, planner fixes", "2 rounds", "No main agent"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("stdout lacks %q:\n%s", want, r.stdout)
		}
	}
	yaml, _ := os.ReadFile(filepath.Join(e.home, "presets", "solo", "preset.yaml"))
	if !strings.Contains(string(yaml), "gate: reviewer") || !strings.Contains(string(yaml), "rounds: 2") {
		t.Errorf("preset.yaml:\n%s", yaml)
	}

	if r := e.run("flow", "solo", "--rounds", "4"); r.code != 0 || !strings.Contains(r.stdout, "reviewer checks") || !strings.Contains(r.stdout, "4 rounds") {
		t.Errorf("--rounds keeps the loop: %d %q", r.code, r.stdout)
	}
	if r := e.run("flow", "solo", "--off"); r.code != 0 || !strings.Contains(r.stdout, "no flow") {
		t.Errorf("--off: %d %q", r.code, r.stdout)
	}
	yaml, _ = os.ReadFile(filepath.Join(e.home, "presets", "solo", "preset.yaml"))
	if strings.Contains(string(yaml), "flow:") {
		t.Errorf("the flow is still stored:\n%s", yaml)
	}
}

func TestFlowRefusesWhatDoesNotAddUp(t *testing.T) {
	e := newEnv(t)
	e.addPresets()

	for name, args := range map[string][]string{
		"unknown agent": {"flow", "solo", "--gate", "ghost", "--fix", "planner"},
		"half a loop":   {"flow", "solo", "--gate", "reviewer"},
		"same agent":    {"flow", "solo", "--gate", "tester", "--fix", "tester"},
		"too many":      {"flow", "solo", "--gate", "tester", "--fix", "planner", "--rounds", "99"},
		"off and gate":  {"flow", "solo", "--off", "--gate", "tester"},
		"no preset":     {"flow", "nope", "--order"},
		"initial":       {"flow", "initial", "--order"},
	} {
		if r := e.run(args...); r.code == 0 {
			t.Errorf("%s: accepted: %q", name, r.stdout)
		}
	}
	yaml, _ := os.ReadFile(filepath.Join(e.home, "presets", "solo", "preset.yaml"))
	if strings.Contains(string(yaml), "flow:") {
		t.Errorf("a refused flow was stored:\n%s", yaml)
	}
}

func TestEditCanIsolateAnAgent(t *testing.T) {
	e := newEnv(t)
	e.addPresets()

	if r := e.run("edit", "solo", "reviewer", "--isolate"); r.code != 0 || !strings.Contains(r.stdout, "isolation") {
		t.Fatalf("isolate: %d %s %s", r.code, r.stdout, r.stderr)
	}
	md, _ := os.ReadFile(filepath.Join(e.home, "presets", "solo", "agents", "reviewer.md"))
	if !strings.Contains(string(md), "squadShare: false") {
		t.Errorf("reviewer.md:\n%s", md)
	}

	if r := e.run("edit", "solo", "reviewer", "--share"); r.code != 0 || !strings.Contains(r.stdout, "shares") {
		t.Fatalf("share: %d %s", r.code, r.stderr)
	}
	md, _ = os.ReadFile(filepath.Join(e.home, "presets", "solo", "agents", "reviewer.md"))
	if strings.Contains(string(md), "squadShare") {
		t.Errorf("sharing is the default and must not be written:\n%s", md)
	}

	if r := e.run("edit", "solo", "--isolate"); r.code != 2 {
		t.Errorf("no agent: %d", r.code)
	}
	if r := e.run("edit", "solo", "ghost", "--isolate"); r.code == 0 {
		t.Error("an unknown agent was accepted")
	}
}

func TestNotesShowsAndClearsTheSharedFile(t *testing.T) {
	e := newEnv(t)
	dir := t.TempDir()
	t.Chdir(dir)

	if r := e.run("notes"); r.code != 0 || !strings.Contains(r.stdout, "No notes here yet") {
		t.Fatalf("empty: %d %q", r.code, r.stdout)
	}

	if err := os.MkdirAll(".squad", 0o755); err != nil {
		t.Fatal(err)
	}
	notes := "goal: ship it\nD auth: bcrypt\n! sec: no rotation @a.go:1 →dev\n"
	if err := os.WriteFile(".squad/notes.md", []byte(notes), 0o644); err != nil {
		t.Fatal(err)
	}
	r := e.run("notes")
	if r.code != 0 || !strings.Contains(r.stdout, "D auth: bcrypt") || !strings.Contains(r.stderr, "3 lines") || !strings.Contains(r.stderr, "1 open") {
		t.Errorf("show: %d %q %q", r.code, r.stdout, r.stderr)
	}

	if r := e.run("notes", "--clear"); r.code != 0 || !strings.Contains(r.stdout, "Cleared") {
		t.Errorf("clear: %d %q", r.code, r.stdout)
	}
	if _, err := os.Stat(filepath.Join(dir, ".squad")); err == nil {
		t.Error("the empty .squad folder was left behind")
	}
	if r := e.run("notes", "--clear"); r.code != 0 || !strings.Contains(r.stdout, "nothing to clear") {
		t.Errorf("clear again: %d %q", r.code, r.stdout)
	}
}

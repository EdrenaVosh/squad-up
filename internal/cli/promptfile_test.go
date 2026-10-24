package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditCanSetThePromptFromAFile(t *testing.T) {
	e := newEnv(t)
	e.addPresets()
	file := filepath.Join(t.TempDir(), "prompt.md")
	if err := os.WriteFile(file, []byte("## Role\nYou plan.\n\n## Response format\n- steps\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	r := e.run("edit", "solo", "planner", "--prompt-file", file)

	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	if !strings.Contains(r.stdout, "planner") || !strings.Contains(r.stdout, "5 lines") {
		t.Errorf("stdout = %q", r.stdout)
	}
	data, _ := os.ReadFile(filepath.Join(e.home, "presets", "solo", "agents", "planner.md"))
	if !strings.Contains(string(data), "## Response format") || !strings.Contains(string(data), "name: planner") {
		t.Errorf("agent file:\n%s", data)
	}
}

func TestEditPromptFileNeedsAnAgentAndAReadableFile(t *testing.T) {
	e := newEnv(t)
	e.addPresets()
	file := filepath.Join(t.TempDir(), "p.md")
	if err := os.WriteFile(file, []byte("x\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if r := e.run("edit", "solo", "--prompt-file", file); r.code != 2 || !strings.Contains(r.stderr, "agent") {
		t.Errorf("no agent: %d %q", r.code, r.stderr)
	}
	if r := e.run("edit", "solo", "planner", "--prompt-file", filepath.Join(t.TempDir(), "nope.md")); r.code != 1 || !strings.Contains(r.stderr, "does not exist") {
		t.Errorf("missing file: %d %q", r.code, r.stderr)
	}
	if r := e.run("edit", "solo", "ghost", "--prompt-file", file); r.code != 1 || !strings.Contains(r.stderr, "ghost") {
		t.Errorf("unknown agent: %d %q", r.code, r.stderr)
	}
	if r := e.run("edit", "initial", "reviewer", "--prompt-file", file); r.code != 1 || !strings.Contains(r.stderr, "squad clone initial") {
		t.Errorf("initial: %d %q", r.code, r.stderr)
	}
}

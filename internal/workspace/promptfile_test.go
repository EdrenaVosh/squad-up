package workspace_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/EdrenaVosh/squad-up/internal/workspace"
)

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReadPromptFileReturnsTheTextWithOneTrailingNewline(t *testing.T) {
	path := writeTemp(t, "p.md", "\n\n## Role\nYou review code.\n\n\n")

	got, err := workspace.ReadPromptFile(path)

	if err != nil || got != "## Role\nYou review code.\n" {
		t.Errorf("got %q, %v", got, err)
	}
}

func TestReadPromptFileTakesTheBodyOfAnAgentFile(t *testing.T) {
	path := writeTemp(t, "agent.md", "---\nname: other\ndescription: d\n---\nBe strict.\n")

	got, err := workspace.ReadPromptFile(path)

	if err != nil || got != "Be strict.\n" {
		t.Errorf("got %q, %v", got, err)
	}
}

func TestReadPromptFileAcceptsWhatATerminalPastes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "my prompt.md")
	if err := os.WriteFile(path, []byte("Text.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir) // Windows does not read HOME

	for _, in := range []string{
		"'" + path + "'",
		`"` + path + `"`,
		"  " + strings.ReplaceAll(path, " ", `\ `) + "  ",
		"~/my prompt.md",
	} {
		if got, err := workspace.ReadPromptFile(in); err != nil || got != "Text.\n" {
			t.Errorf("ReadPromptFile(%q) = %q, %v", in, got, err)
		}
	}
}

func TestReadPromptFileRefusesWhatIsNotAPrompt(t *testing.T) {
	dir := t.TempDir()
	big := writeTemp(t, "big.md", strings.Repeat("x", 300<<10))
	bin := writeTemp(t, "bin.md", "\xff\xfe\x00bad")
	empty := writeTemp(t, "empty.md", "  \n\n")
	for name, tc := range map[string]struct{ path, want string }{
		"missing": {filepath.Join(dir, "nope.md"), "does not exist"},
		"folder":  {dir, "folder"},
		"big":     {big, "at most 256 KB"},
		"binary":  {bin, "UTF-8"},
		"empty":   {empty, "no text"},
		"nothing": {"  ", "no file"},
	} {
		if _, err := workspace.ReadPromptFile(tc.path); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want it to mention %q", name, err, tc.want)
		}
	}
}

func TestSetPromptReplacesTheBodyAndKeepsTheFrontmatter(t *testing.T) {
	w := setup(t)

	if err := w.SetPrompt("solo", "planner", "New prompt.\n"); err != nil {
		t.Fatal(err)
	}

	b, _ := w.Store().Load("solo")
	got := string(b.Agents[0].Data)
	if !strings.HasSuffix(got, "---\nNew prompt.\n") || !strings.Contains(got, "name: planner") {
		t.Errorf("agent file:\n%s", got)
	}
	if string(b.Agents[1].Data) != agentMD("tester", "Solo tester.") {
		t.Error("another agent changed")
	}
	if err := w.SetPrompt("solo", "ghost", "x"); !errors.Is(err, workspace.ErrNoAgent) {
		t.Errorf("unknown agent: %v", err)
	}
	if err := w.SetPrompt("initial", "reviewer", "x"); !errors.Is(err, workspace.ErrProtected) {
		t.Errorf("initial: %v", err)
	}
}

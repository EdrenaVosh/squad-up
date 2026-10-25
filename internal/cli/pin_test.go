package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestPinWritesTheFileInTheCurrentFolder(t *testing.T) {
	e := newEnv(t)
	e.addPresets()
	dir := t.TempDir()
	t.Chdir(dir)

	r := e.run("pin", "solo")

	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".squad-up"))
	if string(data) != "solo\n" || !strings.Contains(r.stdout, "solo") {
		t.Errorf("file %q, stdout %q", data, r.stdout)
	}

	if r := e.run("pin", "--clear"); r.code != 0 || !strings.Contains(r.stdout, "Removed") {
		t.Errorf("--clear: %d %q", r.code, r.stdout)
	}
	if _, err := os.Stat(filepath.Join(dir, ".squad-up")); err == nil {
		t.Error("the file is still there")
	}
	if r := e.run("pin", "--clear"); r.code != 0 || !strings.Contains(r.stdout, "nothing") {
		t.Errorf("--clear with no file: %d %q", r.code, r.stdout)
	}
}

func TestPinRefusesWhatCannotBePinned(t *testing.T) {
	e := newEnv(t)
	e.addPresets()
	t.Chdir(t.TempDir())

	if r := e.run("pin", "nope"); r.code != 1 || !strings.Contains(r.stderr, "squad list") {
		t.Errorf("unknown preset: %d %q", r.code, r.stderr)
	}
	if r := e.run("pin", "initial"); r.code != 1 || !strings.Contains(r.stderr, "initial") {
		t.Errorf("initial: %d %q", r.code, r.stderr)
	}
	if r := e.run("pin"); r.code != 2 {
		t.Errorf("no name: %d %q", r.code, r.stderr)
	}
	if _, err := os.Stat(".squad-up"); err == nil {
		t.Error("a refused pin left a file")
	}
}

func TestRunWithoutANameUsesTheNearestPin(t *testing.T) {
	e := newEnv(t)
	e.addPresets()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".squad-up"), []byte("bug-hunt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "src")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(sub)

	r := e.run("run", "--dry-run")

	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	if !strings.Contains(r.stdout, "Would run") || !strings.Contains(r.stdout, "debugger") {
		t.Errorf("stdout = %q", r.stdout)
	}
	if !strings.Contains(r.stderr, "bug-hunt") || !strings.Contains(r.stderr, ".squad-up") {
		t.Errorf("the pin should be announced: %q", r.stderr)
	}
}

func TestRunWithAnExplicitNameIgnoresThePin(t *testing.T) {
	e := newEnv(t)
	e.addPresets()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".squad-up"), []byte("bug-hunt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)

	r := e.run("run", "solo", "--dry-run")

	if r.code != 0 || strings.Contains(r.stdout, "debugger") || !strings.Contains(r.stdout, "planner") {
		t.Errorf("%d %q", r.code, r.stdout)
	}
}

func TestRunWithoutANameOrPinExplainsBoth(t *testing.T) {
	e := newEnv(t)
	e.addPresets()
	t.Chdir(t.TempDir())

	r := e.run("run")

	if r.code != 1 || !strings.Contains(r.stderr, "squad run <preset>") || !strings.Contains(r.stderr, "squad pin") {
		t.Errorf("%d %q", r.code, r.stderr)
	}
}

func TestRunNamesABrokenPinFile(t *testing.T) {
	e := newEnv(t)
	e.addPresets()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".squad-up"), []byte("Not A Name\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)

	r := e.run("run", "--dry-run")

	if r.code != 1 || !strings.Contains(r.stderr, filepath.Join(root, ".squad-up")) {
		t.Errorf("%d %q", r.code, r.stderr)
	}
}

func TestStatusNotesWhenTheProjectPinsAnotherPreset(t *testing.T) {
	e := newEnv(t)
	e.addPresets()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".squad-up"), []byte("bug-hunt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	if r := e.run("use", "solo", "--yes"); r.code != 0 {
		t.Fatalf("use: %s", r.stderr)
	}

	r := e.run("status")
	if !strings.Contains(r.stdout, "pins bug-hunt") || !strings.Contains(r.stdout, "solo is equipped") || !strings.Contains(r.stdout, "squad use bug-hunt") {
		t.Errorf("mismatch should be noted:\n%s", r.stdout)
	}

	var st struct {
		Pinned *struct {
			Preset  string
			File    string
			Matches bool
		}
	}
	if err := json.Unmarshal([]byte(e.run("status", "--json").stdout), &st); err != nil || st.Pinned == nil || st.Pinned.Preset != "bug-hunt" || st.Pinned.Matches {
		t.Errorf("json pinned = %+v, %v", st.Pinned, err)
	}

	if r := e.run("use", "bug-hunt", "--yes"); r.code != 0 {
		t.Fatalf("use: %s", r.stderr)
	}
	r = e.run("status")
	if strings.Contains(r.stdout, "pins") || !strings.Contains(r.stdout, "Pinned") {
		t.Errorf("a matching pin needs no warning:\n%s", r.stdout)
	}
}

func TestStatusWithoutAPinSaysNothingAboutIt(t *testing.T) {
	e := newEnv(t)
	e.addPresets()
	t.Chdir(t.TempDir())

	r := e.run("status")

	if strings.Contains(r.stdout, "Pinned") || strings.Contains(r.stdout, "pins") {
		t.Errorf("stdout = %q", r.stdout)
	}
	if strings.Contains(e.run("status", "--json").stdout, `"pinned"`) {
		t.Error("json should leave out pinned")
	}
}

func TestThePluginOnlyCallsCommandsThatExist(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "plugin", "skills", "squad", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	used := map[string]bool{}
	for _, m := range regexp.MustCompile("`squad ([a-z]+)( --?[a-z-]+)?").FindAllStringSubmatch(string(data), -1) {
		used[m[1]] = true
	}
	if !used["status"] || !used["list"] || !used["use"] {
		t.Fatalf("the skill should use status, list and use, found %v", used)
	}
	e := newEnv(t)
	for name := range used {
		if r := e.run(name, "--help"); r.code != 0 {
			t.Errorf("the plugin calls `squad %s`, which does not exist: %s", name, r.stderr)
		}
	}
	for _, flag := range []string{"--json", "--yes"} {
		if !strings.Contains(string(data), flag) {
			t.Errorf("the skill no longer mentions %s", flag)
		}
	}
	if r := e.run("status", "--json", "--help"); r.code != 0 {
		t.Errorf("status --json: %s", r.stderr)
	}
}

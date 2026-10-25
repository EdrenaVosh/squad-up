package cli_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/EdrenaVosh/squadup/internal/preset"
	"github.com/EdrenaVosh/squadup/internal/testutil"
)

func loadPreset(t *testing.T, e *env, name string) *preset.Bundle {
	t.Helper()
	b, err := preset.Store{Dir: filepath.Join(e.home, "presets")}.Load(name)
	if err != nil {
		t.Fatalf("Load(%s): %v", name, err)
	}
	return b
}

// ---- new ----

func TestNewCreatesABlankPreset(t *testing.T) {
	e := ready(t)

	r := e.run("new", "fresh")

	if r.code != 0 || !strings.Contains(r.stdout, `Created preset "fresh"`) {
		t.Fatalf("exit code = %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	b := loadPreset(t, e, "fresh")
	if b.Preset.Mode != preset.ModeReplace || len(b.Agents) != 0 {
		t.Errorf("preset = %+v", b.Preset)
	}
	if !strings.Contains(e.run("list").stdout, "fresh") {
		t.Error("the new preset is not listed")
	}
}

func TestNewNameProblems(t *testing.T) {
	e := ready(t)

	for _, name := range []string{"initial", "   ", strings.Repeat("я", 81)} {
		if r := e.run("new", name); r.code != 2 || !strings.Contains(r.stderr, "Fix:") {
			t.Errorf("new %q: exit code = %d\n%s", name, r.code, r.stderr)
		}
	}
	if r := e.run("new", "solo"); r.code != 1 || !strings.Contains(r.stderr, "already exists") {
		t.Errorf("new solo: exit code = %d\n%s", r.code, r.stderr)
	}
	if r := e.run("new"); r.code != 1 || !strings.Contains(r.stderr, "squad new <name>") {
		t.Errorf("new without a name: exit code = %d\n%s", r.code, r.stderr)
	}
}

func TestNewFromAPresetTheCurrentSetupOrInitial(t *testing.T) {
	e := ready(t)

	if r := e.run("new", "from-solo", "--from", "solo"); r.code != 0 {
		t.Fatalf("--from solo: %d\n%s", r.code, r.stderr)
	}
	if got := loadPreset(t, e, "from-solo"); len(got.Agents) != 3 || got.Preset.Settings.Model != "sonnet" {
		t.Errorf("from-solo = %+v", got.Preset)
	}

	if r := e.run("new", "from-live", "--from", "current"); r.code != 0 || !strings.Contains(r.stdout, "Captured preset") {
		t.Fatalf("--from current: %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	if got := loadPreset(t, e, "from-live"); len(got.Agents) != 3 || got.Preset.Settings.Model != "opus" {
		t.Errorf("from-live = %+v", got.Preset)
	}

	if r := e.run("new", "from-initial", "--from", "initial"); r.code != 0 {
		t.Fatalf("--from initial: %d\n%s", r.code, r.stderr)
	}
	if got := loadPreset(t, e, "from-initial"); got.Preset.Description != "Copy of your original setup" {
		t.Errorf("from-initial = %+v", got.Preset)
	}

	if r := e.run("new", "x", "--from", "nope"); r.code != 1 || !strings.Contains(r.stderr, "squad list") {
		t.Errorf("--from nope: %d\n%s", r.code, r.stderr)
	}
}

func TestNewAppliesTitleDescriptionAndMode(t *testing.T) {
	e := ready(t)

	r := e.run("new", "fancy", "--title", "Fancy Team", "--description", "For fancy work", "--mode", "extend")

	if r.code != 0 {
		t.Fatalf("exit code = %d\n%s", r.code, r.stderr)
	}
	got := loadPreset(t, e, "fancy").Preset
	if got.Title != "Fancy Team" || got.Description != "For fancy work" || got.Mode != preset.ModeExtend {
		t.Errorf("preset = %+v", got)
	}
	if bad := e.run("new", "other", "--mode", "merge"); bad.code != 2 {
		t.Errorf("--mode merge: exit code = %d, want 2\n%s", bad.code, bad.stderr)
	}
	if e.run("list").code != 0 || strings.Contains(e.run("list").stdout, "other") {
		t.Error("a failed new must not leave a preset behind")
	}
}

// ---- clone ----

func TestCloneCopiesAPreset(t *testing.T) {
	e := ready(t)

	r := e.run("clone", "solo", "alpha")

	if r.code != 0 || !strings.Contains(r.stdout, `Copied "solo" to "alpha"`) {
		t.Fatalf("exit code = %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	if got := loadPreset(t, e, "alpha"); len(got.Agents) != 3 || got.Preset.Name != "alpha" {
		t.Errorf("alpha = %+v", got.Preset)
	}
	if r := e.run("clone", "nope", "x"); r.code != 1 || !strings.Contains(r.stderr, "squad list") {
		t.Errorf("clone nope: %d\n%s", r.code, r.stderr)
	}
	if r := e.run("clone", "solo", "alpha"); r.code != 1 || !strings.Contains(r.stderr, "already exists") {
		t.Errorf("clone onto an existing name: %d\n%s", r.code, r.stderr)
	}
}

func TestCloneOfInitialReportsAgentsThatCannotBeCopied(t *testing.T) {
	e := newEnv(t)
	testutil.WriteFiles(t, e.claudeDir, map[string]string{
		"agents/good.md":   goodReviewer,
		"agents/broken.md": "no frontmatter",
	})
	e.run("list")

	r := e.run("clone", "initial", "mine")

	if r.code != 0 || !strings.Contains(r.stderr, "Skipped broken.md") || !strings.Contains(r.stdout, "1 agent") {
		t.Fatalf("exit code = %d\nstdout:\n%s\nstderr:\n%s", r.code, r.stdout, r.stderr)
	}
}

// ---- rename ----

func TestRenameChangesTheNameAndFollowsTheEquippedPreset(t *testing.T) {
	e := ready(t)
	e.run("use", "solo", "--yes")

	r := e.run("rename", "solo", "alpha")

	if r.code != 0 || !strings.Contains(r.stdout, `Renamed "solo" to "alpha"`) || !strings.Contains(r.stdout, "stays equipped") {
		t.Fatalf("exit code = %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	if !strings.Contains(e.run("status").stdout, "Active preset  alpha") {
		t.Errorf("status:\n%s", e.run("status").stdout)
	}
	if list := e.run("list").stdout; !strings.Contains(list, "● alpha") || strings.Contains(list, "solo") {
		t.Errorf("list:\n%s", list)
	}
	if again := e.run("use", "bug-hunt", "--yes"); again.code != 0 {
		t.Errorf("the renamed preset must still switch cleanly: %d\n%s", again.code, again.stderr)
	}
}

func TestRenameProblems(t *testing.T) {
	e := ready(t)

	for args, want := range map[string]int{
		"initial x":     1,
		"nope x":        1,
		"solo bug-hunt": 1,
		"solo initial":  2,
	} {
		if r := e.run(append([]string{"rename"}, strings.Fields(args)...)...); r.code != want || !strings.Contains(r.stderr, "Fix:") {
			t.Errorf("rename %s: exit code = %d, want %d\n%s", args, r.code, want, r.stderr)
		}
	}
}

// ---- rm ----

func TestRmNeedsConfirmation(t *testing.T) {
	e := ready(t)

	noYes := e.run("rm", "solo")
	if noYes.code != 1 || !strings.Contains(noYes.stderr, "--yes") {
		t.Fatalf("exit code = %d\n%s", noYes.code, noYes.stderr)
	}
	declined := e.runWith("n\n", true, "rm", "solo")
	if declined.code != 3 || !strings.Contains(declined.stderr, "Cancelled") {
		t.Fatalf("declined: exit code = %d\n%s", declined.code, declined.stderr)
	}
	accepted := e.runWith("y\n", true, "rm", "solo")
	if accepted.code != 0 || !strings.Contains(accepted.stdout, `Removed preset "solo"`) {
		t.Fatalf("accepted: exit code = %d\n%s%s", accepted.code, accepted.stdout, accepted.stderr)
	}
	if strings.Contains(e.run("list").stdout, "solo") {
		t.Error("the preset is still listed")
	}
	if yes := e.run("rm", "bug-hunt", "--yes"); yes.code != 0 {
		t.Errorf("--yes: exit code = %d\n%s", yes.code, yes.stderr)
	}
}

func TestRmProtectsInitialAndTheEquippedPreset(t *testing.T) {
	e := ready(t)
	e.run("use", "solo", "--yes")

	initial := e.run("rm", "initial", "--yes", "--force")
	if initial.code != 1 || !strings.Contains(initial.stderr, "squad restore") {
		t.Errorf("rm initial: exit code = %d\n%s", initial.code, initial.stderr)
	}
	equipped := e.run("rm", "solo", "--yes")
	if equipped.code != 1 || !strings.Contains(equipped.stderr, "--force") {
		t.Errorf("rm equipped: exit code = %d\n%s", equipped.code, equipped.stderr)
	}
	forced := e.run("rm", "solo", "--yes", "--force")
	if forced.code != 0 || !strings.Contains(forced.stdout, "squad restore") {
		t.Errorf("rm --force: exit code = %d\n%s", forced.code, forced.stdout)
	}
	if r := e.run("restore", "--yes"); r.code != 0 {
		t.Errorf("restore after removing the equipped preset must still work: %d\n%s", r.code, r.stderr)
	}
}

func TestRmUnknownPreset(t *testing.T) {
	e := ready(t)

	if r := e.run("rm", "nope", "--yes"); r.code != 1 || !strings.Contains(r.stderr, "squad list") {
		t.Fatalf("exit code = %d\n%s", r.code, r.stderr)
	}
}

// ---- edit ----

// editorScript installs an "editor" that appends a line to the file it is given.
func editorScript(t *testing.T, line string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the test editor is a shell script")
	}
	path := filepath.Join(t.TempDir(), "editor.sh")
	script := "#!/bin/sh\nprintf '%s\\n' '" + line + "' >> \"$1\"\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", path)
}

func TestEditRawOpensTheAgentFileAndLintsTheResult(t *testing.T) {
	e := ready(t)
	editorScript(t, "Extra instructions.")

	r := e.run("edit", "solo", "planner", "--raw")

	if r.code != 0 {
		t.Fatalf("exit code = %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	b := loadPreset(t, e, "solo")
	if !strings.HasSuffix(string(b.Agents[0].Data), "Extra instructions.\n") {
		t.Errorf("planner.md = %q", b.Agents[0].Data)
	}
	if !strings.Contains(r.stdout, "solo") {
		t.Errorf("the lint report should follow the edit:\n%s", r.stdout)
	}
}

func TestEditRawOfThePresetFileReportsMistakes(t *testing.T) {
	e := ready(t)
	editorScript(t, "bogus_field: 1")

	r := e.run("edit", "solo", "--raw")

	if r.code != 2 || !strings.Contains(r.stdout, "invalid-preset") {
		t.Fatalf("exit code = %d: a preset.yaml that no longer parses is a lint error\n%s%s", r.code, r.stdout, r.stderr)
	}
}

func TestEditProblems(t *testing.T) {
	e := ready(t)
	editorScript(t, "x")

	if r := e.run("edit", "initial", "--raw"); r.code != 1 || !strings.Contains(r.stderr, "squad clone initial") {
		t.Errorf("edit initial: %d\n%s", r.code, r.stderr)
	}
	if r := e.run("edit", "nope", "--raw"); r.code != 1 || !strings.Contains(r.stderr, "squad list") {
		t.Errorf("edit nope: %d\n%s", r.code, r.stderr)
	}
	if r := e.run("edit", "solo", "ghost", "--raw"); r.code != 1 || !strings.Contains(r.stderr, "ghost") {
		t.Errorf("edit solo ghost: %d\n%s", r.code, r.stderr)
	}
	if r := e.run("edit", "solo"); r.code != 1 || !strings.Contains(r.stderr, "--raw") {
		t.Errorf("edit without a terminal: %d\n%s", r.code, r.stderr)
	}
}

func TestStatusNotesAnEquippedPresetThatWasRemoved(t *testing.T) {
	e := ready(t)
	e.run("use", "solo", "--yes")
	e.run("rm", "solo", "--yes", "--force")

	r := e.run("status")

	if !strings.Contains(r.stdout, "Active preset  solo (preset removed)") {
		t.Errorf("status:\n%s", r.stdout)
	}
}

func TestNamesMayHaveCapitalsSpacesAndSymbols(t *testing.T) {
	e := ready(t)

	r := e.run("new", "Cube creator: v2 ★")
	if r.code != 0 || !strings.Contains(r.stdout, `"Cube creator: v2 ★" (id: cube-creator-v2)`) {
		t.Fatalf("new: %d %q %q", r.code, r.stdout, r.stderr)
	}
	b := loadPreset(t, e, "cube-creator-v2")
	if b.Preset.Title != "Cube creator: v2 ★" || b.Preset.Name != "cube-creator-v2" {
		t.Errorf("preset = %+v", b.Preset)
	}
	if list := e.run("list").stdout; !strings.Contains(list, "cube-creator-v2") {
		t.Errorf("list:\n%s", list)
	}

	// the same title again gets the next free id instead of failing
	if r := e.run("new", "Cube creator: v2 ★"); r.code != 0 || !strings.Contains(r.stdout, "cube-creator-v2-2") {
		t.Errorf("second: %d %q %q", r.code, r.stdout, r.stderr)
	}

	if r := e.run("clone", "cube-creator-v2", "Куб — копия"); r.code != 0 || !strings.Contains(r.stdout, "kub-kopiya") {
		t.Errorf("clone: %d %q %q", r.code, r.stdout, r.stderr)
	}
	if r := e.run("rename", "kub-kopiya", "Another Name!"); r.code != 0 || !strings.Contains(r.stdout, `"Another Name!" (id: another-name)`) {
		t.Errorf("rename: %d %q %q", r.code, r.stdout, r.stderr)
	}
	if b := loadPreset(t, e, "another-name"); b.Preset.Title != "Another Name!" {
		t.Errorf("renamed = %+v", b.Preset)
	}
}

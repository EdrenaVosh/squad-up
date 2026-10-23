package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/EdrenaVosh/squadup/internal/testutil"
)

// ready prepares a foreign setup, captures Initial and stores two presets.
func ready(t *testing.T) *env {
	t.Helper()
	e := newEnv(t)
	e.writeSetup()
	e.run("list") // first run captures Initial
	e.addPresets()
	return e
}

func TestUsePrintsThePlanAndNeedsConfirmationWithoutATerminal(t *testing.T) {
	e := ready(t)
	before := e.tree()

	r := e.run("use", "solo")

	if r.code != 1 || !strings.Contains(r.stderr, "--yes") {
		t.Fatalf("exit code = %d, stderr:\n%s", r.code, r.stderr)
	}
	for _, want := range []string{
		`Equip "solo"`,
		"+ agents/planner.md",
		"~ agents/reviewer.md",
		"- agents/team/tester.md",
		`~ model: "opus" → "sonnet"`,
	} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("plan lacks %q:\n%s", want, r.stdout)
		}
	}
	if strings.Contains(r.stdout, "effortLevel") {
		t.Errorf("effortLevel is already high and must not be listed:\n%s", r.stdout)
	}
	if !reflect.DeepEqual(e.tree(), before) {
		t.Error("nothing may change before confirmation")
	}
}

func TestUseYesEquipsAndPointsToUndo(t *testing.T) {
	e := ready(t)

	r := e.run("use", "solo", "--yes")

	if r.code != 0 {
		t.Fatalf("exit code = %d\n%s", r.code, r.stderr)
	}
	for _, want := range []string{`Equipped "solo"`, "squad undo", "squad restore"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("stdout lacks %q:\n%s", want, r.stdout)
		}
	}
	if !strings.Contains(e.tree()["agents/planner.md"], "Plan it.") {
		t.Error("the preset was not written")
	}
	if status := e.run("status"); !strings.Contains(status.stdout, "Active preset  solo") {
		t.Errorf("status:\n%s", status.stdout)
	}
	if list := e.run("list"); !strings.Contains(list.stdout, "● solo") {
		t.Errorf("list:\n%s", list.stdout)
	}
}

func TestUseAsksOnATerminal(t *testing.T) {
	e := ready(t)
	before := e.tree()

	declined := e.runWith("n\n", true, "use", "solo")
	if declined.code != 3 || !strings.Contains(declined.stderr, "Cancelled") {
		t.Fatalf("declined: exit code = %d\n%s", declined.code, declined.stderr)
	}
	if !reflect.DeepEqual(e.tree(), before) {
		t.Fatal("declining must change nothing")
	}

	accepted := e.runWith("y\n", true, "use", "solo")
	if accepted.code != 0 || !strings.Contains(accepted.stderr, "Apply these changes?") {
		t.Fatalf("accepted: exit code = %d\n%s", accepted.code, accepted.stderr)
	}
}

func TestUseDryRunShowsThePlanAndWritesNothing(t *testing.T) {
	e := ready(t)
	before := e.tree()

	r := e.run("use", "solo", "--dry-run")

	if r.code != 0 {
		t.Fatalf("exit code = %d\n%s", r.code, r.stderr)
	}
	if !strings.Contains(r.stdout, "Dry run") || !strings.Contains(r.stdout, "+ agents/planner.md") {
		t.Errorf("stdout:\n%s", r.stdout)
	}
	if !reflect.DeepEqual(e.tree(), before) {
		t.Error("--dry-run changed the config")
	}
	for _, name := range []string{"backups", "log.jsonl", "state.json", ".lock"} {
		if _, err := os.Stat(filepath.Join(e.home, name)); !os.IsNotExist(err) {
			t.Errorf("--dry-run created %s", name)
		}
	}
}

func TestUseUseRestoreThroughTheCommandsIsByteExact(t *testing.T) {
	e := ready(t)
	original := e.tree()

	for _, args := range [][]string{
		{"use", "solo", "--yes"},
		{"use", "bug-hunt", "--yes"},
		{"use", "solo", "--yes"},
		{"restore", "--yes"},
	} {
		if r := e.run(args...); r.code != 0 {
			t.Fatalf("%v: exit code = %d\n%s", args, r.code, r.stderr)
		}
	}

	if got := e.tree(); !reflect.DeepEqual(got, original) {
		t.Fatalf("the tree differs from the original\n got: %v\nwant: %v", got, original)
	}
}

func TestRestoreOnInitialSaysSo(t *testing.T) {
	e := ready(t)

	r := e.run("restore", "--yes")

	if r.code != 0 || !strings.Contains(r.stdout, "Already on Initial") {
		t.Fatalf("exit code = %d, stdout:\n%s", r.code, r.stdout)
	}
}

func TestUseUnknownPresetPointsToList(t *testing.T) {
	e := ready(t)

	r := e.run("use", "nope", "--yes")

	if r.code != 1 || !strings.Contains(r.stderr, "nope") || !strings.Contains(r.stderr, "squad list") {
		t.Fatalf("exit code = %d, stderr:\n%s", r.code, r.stderr)
	}
}

func TestUseRequiresAPresetName(t *testing.T) {
	e := ready(t)

	if r := e.run("use"); r.code != 1 {
		t.Fatalf("exit code = %d, want 1", r.code)
	}
}

func TestUseConflictNeedsForce(t *testing.T) {
	e := ready(t)
	testutil.WriteFiles(t, e.claudeDir, map[string]string{"agents/planner.md": agentMD("planner", "the user's own")})
	before := e.tree()

	r := e.run("use", "solo", "--yes")

	if r.code != 1 || !strings.Contains(r.stdout, "agents/planner.md") || !strings.Contains(r.stderr, "--force") {
		t.Fatalf("exit code = %d\nstdout:\n%s\nstderr:\n%s", r.code, r.stdout, r.stderr)
	}
	if !reflect.DeepEqual(e.tree(), before) {
		t.Error("a conflict must not write anything")
	}
	if forced := e.run("use", "solo", "--yes", "--force"); forced.code != 0 {
		t.Fatalf("--force: exit code = %d\n%s", forced.code, forced.stderr)
	}
}

func TestUseWithManualEditsNeedsADecision(t *testing.T) {
	e := ready(t)
	e.run("use", "solo", "--yes")
	testutil.WriteFiles(t, e.claudeDir, map[string]string{"agents/planner.md": agentMD("planner", "edited by hand")})

	r := e.run("use", "bug-hunt", "--yes")

	if r.code != 1 || !strings.Contains(r.stderr, "--save-edits") || !strings.Contains(r.stderr, "--discard-edits") {
		t.Fatalf("exit code = %d\n%s", r.code, r.stderr)
	}
	if !strings.Contains(r.stdout, "Manual edits detected") || !strings.Contains(r.stdout, "agents/planner.md") {
		t.Errorf("the plan must show the edits:\n%s", r.stdout)
	}

	if saved := e.run("use", "bug-hunt", "--yes", "--save-edits"); saved.code != 0 {
		t.Fatalf("--save-edits: exit code = %d\n%s", saved.code, saved.stderr)
	}
	got, err := os.ReadFile(filepath.Join(e.home, "presets", "solo", "agents", "planner.md"))
	if err != nil || string(got) != agentMD("planner", "edited by hand") {
		t.Errorf("the edit was not saved into solo: %q, %v", got, err)
	}
}

func TestUseDiscardEditsOverwrites(t *testing.T) {
	e := ready(t)
	e.run("use", "solo", "--yes")
	testutil.WriteFiles(t, e.claudeDir, map[string]string{"agents/planner.md": agentMD("planner", "edited by hand")})

	r := e.run("use", "bug-hunt", "--yes", "--discard-edits")

	if r.code != 0 {
		t.Fatalf("exit code = %d\n%s", r.code, r.stderr)
	}
}

func TestUseOffersTheDriftChoiceOnATerminal(t *testing.T) {
	e := ready(t)
	e.run("use", "solo", "--yes")
	testutil.WriteFiles(t, e.claudeDir, map[string]string{"agents/planner.md": agentMD("planner", "edited by hand")})

	cancelled := e.runWith("c\n", true, "use", "bug-hunt")
	if cancelled.code != 3 {
		t.Fatalf("cancel: exit code = %d\n%s", cancelled.code, cancelled.stderr)
	}

	discarded := e.runWith("d\ny\n", true, "use", "bug-hunt")
	if discarded.code != 0 || !strings.Contains(discarded.stderr, "[s]ave") {
		t.Fatalf("discard: exit code = %d\n%s", discarded.code, discarded.stderr)
	}
}

func TestSaveAndDiscardEditsExcludeEachOther(t *testing.T) {
	e := ready(t)

	r := e.run("use", "solo", "--yes", "--save-edits", "--discard-edits")

	if r.code != 1 {
		t.Fatalf("exit code = %d, want 1", r.code)
	}
}

func TestUseWarnsWhenAnthropicModelOverridesTheModel(t *testing.T) {
	e := ready(t)
	t.Setenv("ANTHROPIC_MODEL", "opus")

	r := e.run("use", "solo", "--dry-run")

	if !strings.Contains(r.stdout, "ANTHROPIC_MODEL is set (opus)") {
		t.Errorf("stdout lacks the warning:\n%s", r.stdout)
	}
}

func TestUseSaysWhenAnAgentsFolderNeedsARestart(t *testing.T) {
	e := newEnv(t)
	testutil.WriteFiles(t, e.claudeDir, map[string]string{"settings.json": "{}\n"})
	e.run("list")
	e.addPresets()

	r := e.run("use", "solo", "--yes")

	if !strings.Contains(r.stdout, "restart claude") {
		t.Errorf("stdout lacks the restart hint:\n%s", r.stdout)
	}
}

func TestAHeldLockIsReportedClearly(t *testing.T) {
	e := ready(t)
	testutil.WriteFiles(t, e.home, map[string]string{".lock": "pid 4242"})

	r := e.run("use", "solo", "--yes")

	if r.code != 1 || !strings.Contains(r.stderr, "another squad is running") || !strings.Contains(r.stderr, ".lock") {
		t.Fatalf("exit code = %d\n%s", r.code, r.stderr)
	}
}

func TestInvalidSettingsJSONIsReportedWithTheFile(t *testing.T) {
	e := ready(t)
	testutil.WriteFiles(t, e.claudeDir, map[string]string{"settings.json": `{"model": `})

	r := e.run("use", "solo", "--yes")

	if r.code != 1 || !strings.Contains(r.stderr, "settings.json") {
		t.Fatalf("exit code = %d\n%s", r.code, r.stderr)
	}
}

// ---- undo ----

func TestUndoReturnsToThePreviousState(t *testing.T) {
	e := ready(t)
	original := e.tree()
	e.run("use", "solo", "--yes")

	r := e.run("undo", "--yes")

	if r.code != 0 {
		t.Fatalf("exit code = %d\n%s", r.code, r.stderr)
	}
	if !reflect.DeepEqual(e.tree(), original) {
		t.Fatal("undo did not return to the original tree")
	}
	if !strings.Contains(r.stdout, "Undone") {
		t.Errorf("stdout:\n%s", r.stdout)
	}
}

func TestUndoNeedsConfirmationAndHasADryRun(t *testing.T) {
	e := ready(t)
	e.run("use", "solo", "--yes")
	before := e.tree()

	noYes := e.run("undo")
	if noYes.code != 1 || !strings.Contains(noYes.stderr, "--yes") {
		t.Fatalf("exit code = %d\n%s", noYes.code, noYes.stderr)
	}
	dry := e.run("undo", "--dry-run")
	if dry.code != 0 || !strings.Contains(dry.stdout, "Dry run") {
		t.Fatalf("dry run: exit code = %d\n%s", dry.code, dry.stdout)
	}
	if !reflect.DeepEqual(e.tree(), before) {
		t.Error("neither may change the config")
	}
}

func TestUndoWithNothingToUndo(t *testing.T) {
	e := ready(t)

	r := e.run("undo", "--yes")

	if r.code != 1 || !strings.Contains(r.stderr, "nothing to undo") {
		t.Fatalf("exit code = %d\n%s", r.code, r.stderr)
	}
}

func TestUndoToAnEarlierPoint(t *testing.T) {
	e := ready(t)
	original := e.tree()
	e.run("use", "solo", "--yes")
	e.run("use", "bug-hunt", "--yes")
	id := firstLogID(t, e)

	r := e.run("undo", "--to", id, "--yes")

	if r.code != 0 {
		t.Fatalf("exit code = %d\n%s", r.code, r.stderr)
	}
	if !reflect.DeepEqual(e.tree(), original) {
		t.Fatal("undo --to the first operation must return to the original tree")
	}
}

func TestUndoToAnUnknownPoint(t *testing.T) {
	e := ready(t)
	e.run("use", "solo", "--yes")

	r := e.run("undo", "--to", "2000-01-01T00-00-00", "--yes")

	if r.code != 1 || !strings.Contains(r.stderr, "squad log") {
		t.Fatalf("exit code = %d\n%s", r.code, r.stderr)
	}
}

// firstLogID returns the id of the oldest operation in `squad log --json`.
func firstLogID(t *testing.T, e *env) string {
	t.Helper()
	var entries []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(e.run("log", "--json").stdout), &entries); err != nil || len(entries) == 0 {
		t.Fatalf("log --json: %v, %v", entries, err)
	}
	return entries[len(entries)-1].ID // newest first
}

// ---- log ----

func TestLogListsOperationsNewestFirst(t *testing.T) {
	e := ready(t)
	e.run("use", "solo", "--yes")
	e.run("use", "bug-hunt", "--yes")
	e.run("restore", "--yes")

	r := e.run("log")

	if r.code != 0 {
		t.Fatalf("exit code = %d\n%s", r.code, r.stderr)
	}
	lines := strings.Split(strings.TrimSpace(r.stdout), "\n")
	if len(lines) != 4 {
		t.Fatalf("log has %d lines, want a header and 3 operations:\n%s", len(lines), r.stdout)
	}
	if !strings.HasPrefix(lines[0], "ID") || !strings.Contains(lines[0], "COMMAND") {
		t.Errorf("header = %q", lines[0])
	}
	idRE := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2}(-\d+)?\s`)
	for i, wantCmd := range []string{"restore", "use", "use"} {
		if !idRE.MatchString(lines[i+1]) || !strings.Contains(lines[i+1], wantCmd) {
			t.Errorf("line %d = %q, want an id and %q", i+1, lines[i+1], wantCmd)
		}
	}
}

func TestLogJSON(t *testing.T) {
	e := ready(t)
	e.run("use", "solo", "--yes")

	r := e.run("log", "--json")

	var got []struct {
		ID        string `json:"id"`
		Time      string `json:"time"`
		Command   string `json:"command"`
		Preset    string `json:"preset"`
		Available bool   `json:"backupAvailable"`
	}
	if err := json.Unmarshal([]byte(r.stdout), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, r.stdout)
	}
	if len(got) != 1 || got[0].Command != "use" || got[0].Preset != "solo" || got[0].ID == "" || !got[0].Available {
		t.Errorf("log = %+v", got)
	}
}

func TestLogOfAFreshSetup(t *testing.T) {
	e := ready(t)

	r := e.run("log")

	if r.code != 0 || !strings.Contains(r.stdout, "No operations yet") {
		t.Fatalf("exit code = %d\n%s", r.code, r.stdout)
	}
	if j := e.run("log", "--json"); strings.TrimSpace(j.stdout) != "[]" {
		t.Errorf("log --json = %q, want []", j.stdout)
	}
}

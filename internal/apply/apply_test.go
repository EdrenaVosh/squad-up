package apply_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/EdrenaVosh/squad-up/internal/apply"
	"github.com/EdrenaVosh/squad-up/internal/testutil"
	"github.com/tidwall/gjson"
)

func has(tree map[string]string, path string) bool { _, ok := tree[path]; return ok }

// ---- the main promise ----

func TestUseUseRestoreReturnsTheConfigByteForByte(t *testing.T) {
	f := newFx(t, nil)

	f.use("solo", apply.Options{})
	afterSolo := f.tree()
	f.use("bug-hunt", apply.Options{})
	f.use("solo", apply.Options{})
	f.use("extra", apply.Options{})
	f.use("bug-hunt", apply.Options{})
	f.use("initial", apply.Options{})

	if got := f.tree(); !reflect.DeepEqual(got, f.origin) {
		t.Fatalf("restore did not return the original tree\n got: %v\nwant: %v", got, f.origin)
	}
	if afterSolo["agents/planner.md"] == "" {
		t.Fatal("solo was never really applied")
	}
	if s := f.state(); s.ActivePreset() != "initial" || len(s.Manifest) != 0 {
		t.Errorf("state after restore = %+v", s)
	}
}

func TestUseReplacesInitialAgentsAndSetsManagedSettings(t *testing.T) {
	f := newFx(t, nil)

	f.use("solo", apply.Options{})

	tree := f.tree()
	for _, gone := range []string{"agents/team/tester.md", "agents/docs-writer.md"} {
		if has(tree, gone) {
			t.Errorf("%s survived replace mode", gone)
		}
	}
	for _, want := range []string{"agents/planner.md", "agents/reviewer.md", "agents/tester.md", "agents/notes.txt", "projects/p/session.json"} {
		if !has(tree, want) {
			t.Errorf("%s is missing", want)
		}
	}
	if !strings.Contains(tree["agents/reviewer.md"], "Solo reviewer.") {
		t.Errorf("reviewer.md = %q, want the preset's agent", tree["agents/reviewer.md"])
	}
	for path, want := range map[string]string{"model": "sonnet", "effortLevel": "high", "permissions.defaultMode": "acceptEdits", "theme": "dark", "env.SECRET": "keep-out"} {
		if got := f.settingsString(path); got != want {
			t.Errorf("settings %s = %q, want %q", path, got, want)
		}
	}
	if gjson.Get(tree["settings.json"], "agent").Exists() {
		t.Error("`agent` must stay absent: the preset has no main agent and Initial had none")
	}
}

func TestForeignSettingsKeysKeepTheirTextAndOrder(t *testing.T) {
	f := newFx(t, nil)

	f.use("solo", apply.Options{})

	settings := f.tree()["settings.json"]
	for _, keep := range []string{
		`"theme": "dark",`,
		`"env": { "SECRET": "keep-out" },`,
		`"hooks": { "SessionStart": [ { "hooks": [{ "type": "command", "command": "true" }] } ] },`,
	} {
		if !strings.Contains(settings, keep) {
			t.Errorf("settings.json lost %q:\n%s", keep, settings)
		}
	}
	if strings.Index(settings, `"theme"`) > strings.Index(settings, `"env"`) || strings.Index(settings, `"env"`) > strings.Index(settings, `"hooks"`) {
		t.Errorf("foreign keys were reordered:\n%s", settings)
	}
}

func TestUnsetPresetKeysFallBackToInitial(t *testing.T) {
	f := newFx(t, nil)

	f.use("bug-hunt", apply.Options{}) // sets model and main agent only

	if got := f.settingsString("effortLevel"); got != "medium" {
		t.Errorf("effortLevel = %q, want Initial's medium", got)
	}
	if got := f.settingsString("permissions.defaultMode"); got != "default" {
		t.Errorf("permissions.defaultMode = %q, want Initial's default", got)
	}
	if got := f.settingsString("model"); got != "haiku" {
		t.Errorf("model = %q", got)
	}
	if got := f.settingsString("agent"); got != "debugger" {
		t.Errorf("agent = %q", got)
	}
	f.use("solo", apply.Options{})
	if gjson.Get(f.tree()["settings.json"], "agent").Exists() {
		t.Error("switching to a preset without a main agent must remove the key again")
	}
}

func TestExtendModeKeepsInitialAgents(t *testing.T) {
	f := newFx(t, nil)

	f.use("extra", apply.Options{})

	tree := f.tree()
	for _, want := range []string{"agents/team/tester.md", "agents/docs-writer.md", "agents/reviewer.md", "agents/helper.md"} {
		if !has(tree, want) {
			t.Errorf("%s is missing in extend mode", want)
		}
	}
	f.use("solo", apply.Options{})
	if has(f.tree(), "agents/helper.md") {
		t.Error("switching away must remove the extend preset's own agents")
	}
	f.use("extra", apply.Options{})
	if !has(f.tree(), "agents/docs-writer.md") {
		t.Error("switching back to extend must bring the Initial agents back")
	}
}

func TestCleanSetupIsRestoredWithoutAgentsDirOrSettingsFile(t *testing.T) {
	f := newFx(t, map[string]string{"projects/p/session.json": "{}"})

	f.use("solo", apply.Options{})
	if !has(f.tree(), "agents/planner.md") || !has(f.tree(), "settings.json") {
		t.Fatalf("solo was not applied: %v", f.tree())
	}
	f.use("initial", apply.Options{})

	if got := f.tree(); !reflect.DeepEqual(got, f.origin) {
		t.Fatalf("restore left traces\n got: %v\nwant: %v", got, f.origin)
	}
	if _, err := os.Stat(f.p.AgentsDir()); !os.IsNotExist(err) {
		t.Errorf("agents dir should be gone: %v", err)
	}
}

func TestSurfacesTheRestartHintForANewAgentsFolder(t *testing.T) {
	f := newFx(t, map[string]string{"settings.json": "{}\n"})

	plan, err := f.e.PlanUse("solo", apply.Options{})

	if err != nil {
		t.Fatal(err)
	}
	if !containsNote(plan.Notes, "restart claude") {
		t.Errorf("Notes = %v, want a restart hint", plan.Notes)
	}
	if !containsNote(plan.Notes, "new sessions") {
		t.Errorf("Notes = %v, want the model hint", plan.Notes)
	}

	g := newFx(t, nil)
	plan, _ = g.e.PlanUse("extra", apply.Options{})
	if containsNote(plan.Notes, "restart claude") {
		t.Errorf("Notes = %v: the agents folder already exists", plan.Notes)
	}
}

func containsNote(notes []string, sub string) bool {
	for _, n := range notes {
		if strings.Contains(n, sub) {
			return true
		}
	}
	return false
}

// ---- foreign files and conflicts ----

func TestForeignFilesAreNeverTouched(t *testing.T) {
	f := newFx(t, nil)
	testutil.WriteFiles(t, f.p.ClaudeDir, map[string]string{
		"agents/mine.md":       agentMD("mine", "added by the user after Initial"),
		"agents/sub/other.txt": "x",
	})

	f.use("solo", apply.Options{})
	f.use("bug-hunt", apply.Options{})
	f.use("initial", apply.Options{})

	tree := f.tree()
	if tree["agents/mine.md"] != agentMD("mine", "added by the user after Initial") || tree["agents/sub/other.txt"] != "x" {
		t.Fatalf("foreign files changed: %v", tree)
	}
}

func TestForeignFileAtATargetPathIsAConflictAndNothingIsWritten(t *testing.T) {
	f := newFx(t, nil)
	testutil.WriteFiles(t, f.p.ClaudeDir, map[string]string{"agents/planner.md": agentMD("planner", "the user's own planner")})
	before := f.tree()

	plan, err := f.e.PlanUse("solo", apply.Options{})
	if err != nil {
		t.Fatal(err)
	}
	_, execErr := f.e.Execute(plan, apply.Options{})

	if len(plan.Conflicts) == 0 || !strings.Contains(plan.Conflicts[0], "planner.md") {
		t.Errorf("Conflicts = %v", plan.Conflicts)
	}
	if !errors.Is(execErr, apply.ErrConflicts) {
		t.Fatalf("Execute() error = %v, want ErrConflicts", execErr)
	}
	if !reflect.DeepEqual(f.tree(), before) {
		t.Error("a conflicted use must not write anything")
	}
}

func TestForeignAgentWithTheSameNameIsAConflict(t *testing.T) {
	f := newFx(t, nil)
	testutil.WriteFiles(t, f.p.ClaudeDir, map[string]string{"agents/my-planner.md": agentMD("planner", "same name, other file")})

	plan, err := f.e.PlanUse("solo", apply.Options{})

	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Conflicts) != 1 || !strings.Contains(plan.Conflicts[0], "planner") || !strings.Contains(plan.Conflicts[0], "my-planner.md") {
		t.Fatalf("Conflicts = %v", plan.Conflicts)
	}
}

func TestInitialAgentWithTheSameNameConflictsInExtendMode(t *testing.T) {
	f := newFx(t, nil)
	f.save(extendPresetNamed("clash", "reviewer"))

	plan, err := f.e.PlanUse("clash", apply.Options{})

	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Conflicts) == 0 {
		t.Fatal("an extend preset must not duplicate an Initial agent name")
	}
}

func TestForceOverwritesAForeignFileAtATargetPath(t *testing.T) {
	f := newFx(t, nil)
	testutil.WriteFiles(t, f.p.ClaudeDir, map[string]string{"agents/planner.md": agentMD("planner", "the user's own planner")})

	f.use("solo", apply.Options{Force: true})

	if !strings.Contains(f.tree()["agents/planner.md"], "Plan it.") {
		t.Error("--force should have replaced the file")
	}
	backups, err := f.e.Backups()
	if err != nil || len(backups) != 1 {
		t.Fatalf("Backups() = %v, %v", backups, err)
	}
}

func TestModifiedInitialAgentIsKeptAndRestoreNeedsForce(t *testing.T) {
	f := newFx(t, nil)
	testutil.WriteFiles(t, f.p.ClaudeDir, map[string]string{"agents/docs-writer.md": agentMD("docs-writer", "edited by hand")})
	plan, err := f.e.PlanUse("solo", apply.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !containsNote(plan.Warnings, "docs-writer.md") {
		t.Errorf("Warnings = %v, want a note about the kept file", plan.Warnings)
	}
	if _, err := f.e.Execute(plan, apply.Options{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.tree()["agents/docs-writer.md"], "edited by hand") {
		t.Fatal("the user's edit of an Initial agent must survive replace mode")
	}

	restore, err := f.e.PlanUse("initial", apply.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.e.Execute(restore, apply.Options{}); !errors.Is(err, apply.ErrConflicts) {
		t.Fatalf("restore error = %v, want ErrConflicts", err)
	}
	f.use("initial", apply.Options{Force: true})
	if !strings.Contains(f.tree()["agents/docs-writer.md"], "Initial docs.") {
		t.Error("--force should bring back the snapshot version")
	}
}

// ---- plan and dry run ----

func TestPlanListsEveryChangeAndWritesNothing(t *testing.T) {
	f := newFx(t, nil)
	before := f.tree()

	plan, err := f.e.PlanUse("solo", apply.Options{})

	if err != nil {
		t.Fatal(err)
	}
	wantFiles := []apply.FileOp{
		{Kind: apply.OpAdd, Path: "agents/planner.md"},
		{Kind: apply.OpRemove, Path: "agents/docs-writer.md"},
		{Kind: apply.OpModify, Path: "agents/reviewer.md"},
		{Kind: apply.OpRemove, Path: "agents/team/tester.md"},
		{Kind: apply.OpAdd, Path: "agents/tester.md"},
	}
	if len(plan.Files) != len(wantFiles) {
		t.Fatalf("Files = %+v, want %+v", plan.Files, wantFiles)
	}
	got := map[string]apply.OpKind{}
	for _, op := range plan.Files {
		got[op.Path] = op.Kind
	}
	for _, want := range wantFiles {
		if got[want.Path] != want.Kind {
			t.Errorf("%s: kind = %q, want %q", want.Path, got[want.Path], want.Kind)
		}
	}
	keys := map[string]apply.SettingOp{}
	for _, op := range plan.Settings {
		keys[op.Key] = op
	}
	if keys["model"].Kind != apply.OpModify || keys["model"].Old != `"opus"` || keys["model"].New != `"sonnet"` {
		t.Errorf("model op = %+v", keys["model"])
	}
	if keys["effortLevel"].Kind != apply.OpModify || keys["permissions"].Kind != apply.OpModify {
		t.Errorf("settings ops = %+v", plan.Settings)
	}
	if _, ok := keys["agent"]; ok {
		t.Error("`agent` is unchanged and must not be listed")
	}
	if !reflect.DeepEqual(f.tree(), before) {
		t.Error("planning must not write files")
	}
	if _, err := os.Stat(f.p.LockFile()); !os.IsNotExist(err) {
		t.Error("planning must not take the lock")
	}
	if _, err := os.Stat(f.p.BackupsDir()); !os.IsNotExist(err) {
		t.Error("planning must not create backups")
	}
}

func TestUsingTheActivePresetAgainIsANoOp(t *testing.T) {
	f := newFx(t, nil)
	f.use("solo", apply.Options{})
	before := f.tree()
	backups, _ := f.e.Backups()

	plan, err := f.e.PlanUse("solo", apply.Options{})
	if err != nil {
		t.Fatal(err)
	}
	res, err := f.e.Execute(plan, apply.Options{})

	if err != nil || !plan.NoOp {
		t.Fatalf("NoOp = %v, err = %v", plan.NoOp, err)
	}
	after, _ := f.e.Backups()
	if res.BackupID != "" || len(after) != len(backups) || !reflect.DeepEqual(f.tree(), before) {
		t.Error("a no-op must not back up or write")
	}
}

func TestRestoreOnAnUntouchedSetupIsANoOp(t *testing.T) {
	f := newFx(t, nil)
	// Claude Code itself rewrites settings.json on start; that is not squad's to undo.
	testutil.WriteFiles(t, f.p.ClaudeDir, map[string]string{"settings.json": strings.Replace(originalSettings, `"opus"`, `"opus[1m]"`, 1)})
	before := f.tree()

	plan, err := f.e.PlanUse("initial", apply.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.e.Execute(plan, apply.Options{}); err != nil {
		t.Fatal(err)
	}

	if !plan.NoOp || !reflect.DeepEqual(f.tree(), before) {
		t.Errorf("NoOp = %v; the tree must stay as it is", plan.NoOp)
	}
}

func TestUseUnknownPresetFails(t *testing.T) {
	f := newFx(t, nil)

	_, err := f.e.PlanUse("nope", apply.Options{})

	if err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("PlanUse() error = %v", err)
	}
}

func TestUseRejectsAnAgentFileWhoseNameDiffersFromTheList(t *testing.T) {
	f := newFx(t, nil)
	path := filepath.Join(f.p.PresetDir("solo"), "agents", "planner.md")
	if err := os.WriteFile(path, []byte(agentMD("somebody-else", "x")), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := f.e.PlanUse("solo", apply.Options{})

	if err == nil || !strings.Contains(err.Error(), "planner") {
		t.Fatalf("PlanUse() error = %v, want a mismatch error naming planner", err)
	}
}

func TestInvalidSettingsJSONBlocksUseBeforeAnyWrite(t *testing.T) {
	f := newFx(t, nil)
	testutil.WriteFiles(t, f.p.ClaudeDir, map[string]string{"settings.json": `{"model": `})
	before := f.tree()

	_, err := f.e.PlanUse("solo", apply.Options{})

	if err == nil || !strings.Contains(err.Error(), "settings.json") {
		t.Fatalf("PlanUse() error = %v", err)
	}
	if !reflect.DeepEqual(f.tree(), before) {
		t.Error("nothing may be written")
	}
}

// ---- failure, lock, staleness ----

func TestFailureHalfwayRollsEverythingBack(t *testing.T) {
	f := newFx(t, nil)
	// A directory squats on the path of an agent the preset must write.
	if err := os.MkdirAll(filepath.Join(f.p.AgentsDir(), "zz-block"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(f.p.AgentsDir(), "tester.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	f.origin = f.tree()

	plan, err := f.e.PlanUse("solo", apply.Options{})
	if err != nil {
		t.Fatal(err)
	}
	_, execErr := f.e.Execute(plan, apply.Options{})

	if execErr == nil {
		t.Fatal("Execute() succeeded although tester.md cannot be written")
	}
	if got := f.tree(); !reflect.DeepEqual(got, f.origin) {
		t.Fatalf("rollback left changes\n got: %v\nwant: %v", got, f.origin)
	}
	if s := f.state(); s.Active != "" {
		t.Errorf("state = %+v, want untouched", s)
	}
	if _, err := os.Stat(f.p.LockFile()); !os.IsNotExist(err) {
		t.Error("the lock must be released after a failure")
	}
}

func TestAHeldLockStopsAWriter(t *testing.T) {
	f := newFx(t, nil)
	plan, err := f.e.PlanUse("solo", apply.Options{})
	if err != nil {
		t.Fatal(err)
	}
	testutil.WriteFiles(t, f.p.Home, map[string]string{".lock": "pid 1"})

	_, err = f.e.Execute(plan, apply.Options{})

	if err == nil || !strings.Contains(err.Error(), ".lock") {
		t.Fatalf("Execute() error = %v, want it to name the lock", err)
	}
}

func TestExecuteRefusesAPlanThatWentStale(t *testing.T) {
	f := newFx(t, nil)
	plan, err := f.e.PlanUse("solo", apply.Options{})
	if err != nil {
		t.Fatal(err)
	}
	testutil.WriteFiles(t, f.p.ClaudeDir, map[string]string{"agents/team/tester.md": agentMD("tester", "changed while reviewing")})

	_, err = f.e.Execute(plan, apply.Options{})

	if !errors.Is(err, apply.ErrStale) {
		t.Fatalf("Execute() error = %v, want ErrStale", err)
	}
}

// ---- drift ----

func TestDriftBlocksUseUntilTheUserDecides(t *testing.T) {
	f := newFx(t, nil)
	f.use("solo", apply.Options{})
	testutil.WriteFiles(t, f.p.ClaudeDir, map[string]string{"agents/planner.md": agentMD("planner", "edited by hand")})

	plan, err := f.e.PlanUse("bug-hunt", apply.Options{})
	if err != nil {
		t.Fatal(err)
	}
	_, execErr := f.e.Execute(plan, apply.Options{})

	if len(plan.Drift) != 1 || plan.Drift[0].Path != "agents/planner.md" || plan.Drift[0].Kind != apply.DriftModified {
		t.Errorf("Drift = %+v", plan.Drift)
	}
	if !errors.Is(execErr, apply.ErrDrift) {
		t.Fatalf("Execute() error = %v, want ErrDrift", execErr)
	}
	if !strings.Contains(f.tree()["agents/planner.md"], "edited by hand") {
		t.Error("nothing may be overwritten before the user decides")
	}
}

func TestDiscardEditsOverwritesManualChanges(t *testing.T) {
	f := newFx(t, nil)
	f.use("solo", apply.Options{})
	testutil.WriteFiles(t, f.p.ClaudeDir, map[string]string{"agents/planner.md": agentMD("planner", "edited by hand")})

	f.use("bug-hunt", apply.Options{Edits: apply.EditsDiscard})

	if has(f.tree(), "agents/planner.md") {
		t.Error("planner.md belongs to solo and should be gone")
	}
	f.use("initial", apply.Options{Edits: apply.EditsDiscard})
	if !reflect.DeepEqual(f.tree(), f.origin) {
		t.Error("restore after a discard must still be exact")
	}
}

func TestSaveEditsStoresManualChangesInTheCurrentPreset(t *testing.T) {
	f := newFx(t, nil)
	f.use("solo", apply.Options{})
	edited := agentMD("planner", "edited by hand")
	testutil.WriteFiles(t, f.p.ClaudeDir, map[string]string{"agents/planner.md": edited})
	if err := os.Remove(filepath.Join(f.p.AgentsDir(), "tester.md")); err != nil {
		t.Fatal(err)
	}

	f.use("bug-hunt", apply.Options{Edits: apply.EditsSave})

	saved, err := f.store.Load("solo")
	if err != nil {
		t.Fatal(err)
	}
	var planner string
	for _, a := range saved.Agents {
		if a.Name == "planner" {
			planner = string(a.Data)
		}
	}
	if planner != edited {
		t.Errorf("solo/planner.md = %q, want the hand-edited version", planner)
	}
	if !reflect.DeepEqual(saved.Preset.Agents, []string{"planner", "reviewer"}) {
		t.Errorf("solo agents = %v, want the deleted tester dropped", saved.Preset.Agents)
	}
}

func TestSavingEditsIntoThePresetThatIsEquippedKeepsThemInTheFile(t *testing.T) {
	f := newFx(t, nil)
	f.use("solo", apply.Options{})
	edited := agentMD("planner", "edited by hand")
	testutil.WriteFiles(t, f.p.ClaudeDir, map[string]string{"agents/planner.md": edited})
	changed := strings.Replace(f.tree()["settings.json"], `"model": "sonnet"`, `"model": "opus"`, 1)
	testutil.WriteFiles(t, f.p.ClaudeDir, map[string]string{"settings.json": changed})

	f.use("solo", apply.Options{Edits: apply.EditsSave})

	if got := f.tree()["agents/planner.md"]; got != edited {
		t.Errorf("planner.md = %q: saving an edit into the preset must not overwrite it in the file", got)
	}
	if got := f.settingsString("model"); got != "opus" {
		t.Errorf("model = %q, want the hand-edited opus to stay", got)
	}
	saved, err := f.store.Load("solo")
	if err != nil {
		t.Fatal(err)
	}
	if saved.Preset.Settings.Model != "opus" {
		t.Errorf("the preset's model = %q, want opus", saved.Preset.Settings.Model)
	}
	plan, err := f.e.PlanUse("solo", apply.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Drift) != 0 {
		t.Errorf("after saving there is nothing left to drift: %+v", plan.Drift)
	}
}

func TestSettingsDriftIsDetectedByValueNotByFormatting(t *testing.T) {
	f := newFx(t, nil)
	f.use("solo", apply.Options{})

	// Claude Code reformats the file; the values are unchanged.
	compact := f.tree()["settings.json"]
	compact = gjson.Parse(compact).Raw
	compact = strings.Join(strings.Fields(compact), " ")
	testutil.WriteFiles(t, f.p.ClaudeDir, map[string]string{"settings.json": compact})
	plan, err := f.e.PlanUse("bug-hunt", apply.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Drift) != 0 {
		t.Fatalf("reformatting is not drift: %+v", plan.Drift)
	}

	testutil.WriteFiles(t, f.p.ClaudeDir, map[string]string{"settings.json": strings.Replace(compact, `"sonnet"`, `"opus"`, 1)})
	plan, err = f.e.PlanUse("bug-hunt", apply.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Drift) != 1 || plan.Drift[0].Path != "settings.json:model" || plan.Drift[0].Kind != apply.DriftModified {
		t.Fatalf("Drift = %+v", plan.Drift)
	}
}

func TestRestoreAfterClaudeReformattedSettingsKeepsForeignChanges(t *testing.T) {
	f := newFx(t, nil)
	f.use("solo", apply.Options{})
	// Claude Code (or the user) changes a foreign key and reformats.
	changed := strings.Replace(f.tree()["settings.json"], `"theme": "dark"`, `"theme": "light"`, 1)
	testutil.WriteFiles(t, f.p.ClaudeDir, map[string]string{"settings.json": changed})

	f.use("initial", apply.Options{})

	if got := f.settingsString("theme"); got != "light" {
		t.Errorf("theme = %q: a foreign change must survive restore", got)
	}
	if got := f.settingsString("model"); got != "opus" {
		t.Errorf("model = %q, want Initial's opus", got)
	}
	if got := f.settingsString("effortLevel"); got != "medium" {
		t.Errorf("effortLevel = %q, want Initial's medium", got)
	}
}

// ---- undo, backups, journal ----

func TestUndoReturnsToThePreviousStateAndCanBeUndone(t *testing.T) {
	f := newFx(t, nil)
	f.use("solo", apply.Options{})
	afterSolo := f.tree()
	f.use("bug-hunt", apply.Options{})
	afterBug := f.tree()

	f.undo("", apply.Options{})

	if !reflect.DeepEqual(f.tree(), afterSolo) {
		t.Fatalf("undo did not return to the state after solo\n got: %v\nwant: %v", f.tree(), afterSolo)
	}
	if got := f.state().ActivePreset(); got != "solo" {
		t.Errorf("active after undo = %q", got)
	}
	f.undo("", apply.Options{})
	if !reflect.DeepEqual(f.tree(), afterBug) {
		t.Fatal("undoing the undo must return to the state after bug-hunt")
	}
}

func TestUndoToAnEarlierPoint(t *testing.T) {
	f := newFx(t, nil)
	first := f.use("solo", apply.Options{})
	f.use("bug-hunt", apply.Options{})
	f.use("extra", apply.Options{})

	f.undo(first.BackupID, apply.Options{})

	if !reflect.DeepEqual(f.tree(), f.origin) {
		t.Fatalf("undo --to the first operation must return to the original tree\n got: %v", f.tree())
	}
	if got := f.state().ActivePreset(); got != "initial" {
		t.Errorf("active = %q", got)
	}
}

func TestUndoWithoutHistoryHasNothingToDo(t *testing.T) {
	f := newFx(t, nil)

	_, err := f.e.PlanUndo("", apply.Options{})

	if !errors.Is(err, apply.ErrNothingToUndo) {
		t.Fatalf("PlanUndo() error = %v, want ErrNothingToUndo", err)
	}
}

func TestUndoToAnUnknownBackup(t *testing.T) {
	f := newFx(t, nil)
	f.use("solo", apply.Options{})

	_, err := f.e.PlanUndo("2000-01-01T00-00-00", apply.Options{})

	if !errors.Is(err, apply.ErrNoBackup) {
		t.Fatalf("PlanUndo() error = %v, want ErrNoBackup", err)
	}
}

func TestUndoKeepsForeignFilesAddedSinceTheBackup(t *testing.T) {
	f := newFx(t, nil)
	f.use("solo", apply.Options{})
	testutil.WriteFiles(t, f.p.ClaudeDir, map[string]string{"agents/later.md": agentMD("later", "added after the backup")})

	f.undo("", apply.Options{})

	if !has(f.tree(), "agents/later.md") {
		t.Error("undo deleted a file squad never wrote")
	}
}

func TestBackupHoldsAgentsSettingsAndPreviousState(t *testing.T) {
	f := newFx(t, nil)
	f.use("solo", apply.Options{})
	res := f.use("bug-hunt", apply.Options{})

	dir := filepath.Join(f.p.BackupsDir(), res.BackupID)
	tree := testutil.Tree(t, dir)

	for _, want := range []string{"agents/planner.md", "agents/reviewer.md", "settings.json", "state.json", "backup.json"} {
		if !has(tree, want) {
			t.Errorf("backup lacks %s: %v", want, keys(tree))
		}
	}
	if !strings.Contains(tree["settings.json"], `"sonnet"`) {
		t.Errorf("the backup must hold the settings from before the operation:\n%s", tree["settings.json"])
	}
	if !strings.Contains(tree["state.json"], `"solo"`) {
		t.Errorf("the backup must hold the previous state:\n%s", tree["state.json"])
	}
}

func keys(m map[string]string) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestOnlyTheTwentyNewestBackupsAreKept(t *testing.T) {
	f := newFx(t, nil)

	for i := 0; i < 13; i++ {
		f.use("solo", apply.Options{})
		f.use("bug-hunt", apply.Options{})
	}

	backups, err := f.e.Backups()
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 20 {
		t.Fatalf("%d backups kept, want 20", len(backups))
	}
	if backups[0].ID >= backups[19].ID {
		t.Errorf("Backups() must be oldest first: %s … %s", backups[0].ID, backups[19].ID)
	}
	journal, _ := f.e.Journal()
	if len(journal) != 26 {
		t.Errorf("journal has %d entries, want all 26 operations", len(journal))
	}
}

func TestJournalRecordsEveryOperation(t *testing.T) {
	f := newFx(t, nil)
	a := f.use("solo", apply.Options{})
	b := f.use("initial", apply.Options{})
	c := f.undo("", apply.Options{})

	journal, err := f.e.Journal()

	if err != nil {
		t.Fatal(err)
	}
	if len(journal) != 3 {
		t.Fatalf("journal = %+v", journal)
	}
	want := []struct{ command, preset, backup string }{
		{"use", "solo", a.BackupID},
		{"restore", "initial", b.BackupID},
		{"undo", "solo", c.BackupID},
	}
	for i, w := range want {
		e := journal[i]
		if e.Command != w.command || e.Preset != w.preset || e.Backup != w.backup || e.Time.IsZero() {
			t.Errorf("entry %d = %+v, want %+v", i, e, w)
		}
	}
}

func TestJournalOfAFreshHomeIsEmpty(t *testing.T) {
	f := newFx(t, nil)

	journal, err := f.e.Journal()

	if err != nil || len(journal) != 0 {
		t.Fatalf("Journal() = %v, %v", journal, err)
	}
}

func TestPermissionsChangesAreSummarisedInThePlan(t *testing.T) {
	f := newFx(t, nil)

	plan, err := f.e.PlanUse("solo", apply.Options{})

	if err != nil {
		t.Fatal(err)
	}
	for _, op := range plan.Settings {
		if op.Key != "permissions" {
			continue
		}
		if op.Old != "default · 1 allow · 0 ask · 1 deny" || op.New != "acceptEdits · 1 allow · 1 ask · 0 deny" {
			t.Fatalf("permissions op = %+v, want one-line summaries", op)
		}
		return
	}
	t.Fatal("no permissions op in the plan")
}

func TestResolveGivesTheBundleAndSettingsWithInitialFallbacks(t *testing.T) {
	f := newFx(t, nil)

	b, resolved, err := f.e.Resolve("bug-hunt")

	if err != nil {
		t.Fatal(err)
	}
	if b.Preset.Name != "bug-hunt" || len(b.Agents) != 2 {
		t.Errorf("bundle = %+v", b.Preset)
	}
	want := map[string]string{
		"model":       `"haiku"`,
		"agent":       `"debugger"`,
		"effortLevel": `"medium"`,
	}
	for key, raw := range want {
		if resolved[key] != raw {
			t.Errorf("resolved[%s] = %q, want %q", key, resolved[key], raw)
		}
	}
	if !strings.Contains(resolved["permissions"], `"defaultMode": "default"`) {
		t.Errorf("permissions = %q, want Initial's text", resolved["permissions"])
	}
}

func TestResolveFailsForUnknownAndBrokenPresets(t *testing.T) {
	f := newFx(t, nil)

	if _, _, err := f.e.Resolve("nope"); err == nil || !strings.Contains(err.Error(), "nope") {
		t.Errorf("unknown preset: %v", err)
	}
	path := filepath.Join(f.p.PresetDir("solo"), "agents", "planner.md")
	if err := os.WriteFile(path, []byte(agentMD("somebody-else", "x")), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.e.Resolve("solo"); err == nil || !strings.Contains(err.Error(), "planner") {
		t.Errorf("mismatched agent: %v", err)
	}
}

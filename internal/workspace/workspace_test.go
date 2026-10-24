package workspace_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/EdrenaVosh/squadup/internal/apply"
	"github.com/EdrenaVosh/squadup/internal/initial"
	"github.com/EdrenaVosh/squadup/internal/models"
	"github.com/EdrenaVosh/squadup/internal/paths"
	"github.com/EdrenaVosh/squadup/internal/preset"
	"github.com/EdrenaVosh/squadup/internal/testutil"
	"github.com/EdrenaVosh/squadup/internal/workspace"
)

var now = time.Date(2026, 10, 24, 9, 30, 0, 0, time.UTC)

func agentMD(name, body string) string {
	return "---\nname: " + name + "\ndescription: " + name + " agent. Use when " + name + " is needed.\ntools: Read\n---\n" + body + "\n"
}

// setup captures a foreign Claude config as Initial and stores one preset,
// "solo", with two agents and a main agent.
func setup(t *testing.T) workspace.Workspace {
	t.Helper()
	p := paths.Paths{ClaudeDir: filepath.Join(t.TempDir(), "claude"), Home: filepath.Join(t.TempDir(), "squadup")}
	testutil.WriteFiles(t, p.ClaudeDir, map[string]string{
		"agents/reviewer.md":    agentMD("reviewer", "Initial reviewer."),
		"agents/team/tester.md": agentMD("tester", "Initial tester."),
		"agents/broken.md":      "no frontmatter",
		"settings.json":         "{\n  \"model\": \"opus\",\n  \"agent\": \"reviewer\",\n  \"effortLevel\": \"high\",\n  \"permissions\": {\"defaultMode\": \"acceptEdits\", \"allow\": [\"Bash(git status)\"]}\n}\n",
	})
	if _, err := initial.Capture(p, now); err != nil {
		t.Fatal(err)
	}
	main := "planner"
	err := preset.Store{Dir: p.PresetsDir()}.Save(preset.Bundle{
		Preset: preset.Preset{
			Schema: 1, Name: "solo", Title: "solo", Description: "Solo work", Mode: preset.ModeReplace, Main: &main,
			Agents:   []string{"planner", "tester"},
			Settings: preset.Settings{Model: "sonnet"},
		},
		Agents: []preset.AgentFile{
			{Name: "planner", Data: []byte(agentMD("planner", "Plan it."))},
			{Name: "tester", Data: []byte(agentMD("tester", "Solo tester."))},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return workspace.Workspace{Paths: p, Now: func() time.Time { return now }}
}

func TestEntriesListPresetsThenInitialAndMarkTheActiveOne(t *testing.T) {
	w := setup(t)

	entries, err := w.Entries()

	if err != nil || len(entries) != 2 {
		t.Fatalf("Entries() = %+v, %v", entries, err)
	}
	solo, init := entries[0], entries[1]
	if solo.Name != "solo" || solo.Agents != 2 || solo.Active || solo.Initial {
		t.Errorf("solo = %+v", solo)
	}
	if init.Name != "initial" || !init.Initial || !init.Active || init.Agents != 2 || init.CapturedAt == nil {
		t.Errorf("initial = %+v: the broken agent is not counted", init)
	}
	if err := apply.SaveState(w.Paths, apply.State{Active: "solo"}); err != nil {
		t.Fatal(err)
	}
	entries, _ = w.Entries()
	if !entries[0].Active || entries[1].Active {
		t.Errorf("entries after equipping = %+v", entries)
	}
}

func TestEntriesToleratesAnUnreadablePreset(t *testing.T) {
	w := setup(t)
	testutil.WriteFiles(t, filepath.Join(w.Paths.PresetsDir(), "broken"), map[string]string{"preset.yaml": "schema: [oops"})

	entries, err := w.Entries()

	if err != nil || len(entries) != 3 || entries[0].Name != "broken" || entries[0].Error == "" {
		t.Fatalf("Entries() = %+v, %v", entries, err)
	}
}

// ---- new ----

func TestNewCreatesABlankPreset(t *testing.T) {
	w := setup(t)

	b, err := w.New("fresh")

	if err != nil {
		t.Fatal(err)
	}
	if b.Preset.Name != "fresh" || b.Preset.Mode != preset.ModeReplace || b.Preset.Schema != 1 || len(b.Preset.Agents) != 0 {
		t.Errorf("preset = %+v", b.Preset)
	}
	loaded, err := w.Store().Load("fresh")
	if err != nil || loaded.Preset.Title != "fresh" {
		t.Errorf("Load() = %+v, %v", loaded, err)
	}
}

func TestNewRejectsBadReservedAndTakenNames(t *testing.T) {
	w := setup(t)

	for name, want := range map[string]error{
		"Bad Name": workspace.ErrInvalidName,
		"../x":     workspace.ErrInvalidName,
		"initial":  workspace.ErrReservedName,
	} {
		if _, err := w.New(name); !errors.Is(err, want) {
			t.Errorf("New(%q) error = %v, want %v", name, err, want)
		}
	}
	if _, err := w.New("solo"); !errors.Is(err, preset.ErrExists) {
		t.Errorf("New(solo) error = %v, want ErrExists", err)
	}
}

// ---- clone ----

func TestCloneCopiesAPresetByteForByte(t *testing.T) {
	w := setup(t)

	skipped, err := w.Clone("solo", "copy")

	if err != nil || len(skipped) != 0 {
		t.Fatalf("Clone() = %v, %v", skipped, err)
	}
	orig, _ := w.Store().Load("solo")
	got, err := w.Store().Load("copy")
	if err != nil {
		t.Fatal(err)
	}
	if got.Preset.Name != "copy" || got.Preset.Title != "copy" || got.Preset.Description != "Solo work" {
		t.Errorf("preset = %+v", got.Preset)
	}
	if !reflect.DeepEqual(got.Agents, orig.Agents) || *got.Preset.Main != "planner" || got.Preset.Settings.Model != "sonnet" {
		t.Errorf("the copy differs from the original: %+v", got)
	}
}

func TestCloneOfInitialMakesAnEditableCopyAndLeavesInitialAlone(t *testing.T) {
	w := setup(t)
	before := testutil.Tree(t, w.Paths.InitialDir())

	skipped, err := w.Clone("initial", "mine")

	if err != nil {
		t.Fatal(err)
	}
	if len(skipped) != 1 || skipped[0].Path != "broken.md" {
		t.Errorf("skipped = %+v, want the broken agent reported", skipped)
	}
	got, err := w.Store().Load("mine")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Preset.Agents, []string{"reviewer", "tester"}) || got.Preset.Settings.Model != "opus" ||
		got.Preset.Settings.EffortLevel != "high" || *got.Preset.Main != "reviewer" || got.Preset.Settings.Permissions.DefaultMode != "acceptEdits" {
		t.Errorf("clone = %+v", got.Preset)
	}
	if string(got.Agents[1].Data) != agentMD("tester", "Initial tester.") {
		t.Errorf("tester.md = %q", got.Agents[1].Data)
	}
	if !reflect.DeepEqual(testutil.Tree(t, w.Paths.InitialDir()), before) {
		t.Error("Initial changed")
	}
}

func TestCloneRejectsUnknownSourcesAndBadNames(t *testing.T) {
	w := setup(t)

	if _, err := w.Clone("nope", "x"); !errors.Is(err, preset.ErrNotFound) {
		t.Errorf("unknown source: %v", err)
	}
	if _, err := w.Clone("solo", "Bad"); !errors.Is(err, workspace.ErrInvalidName) {
		t.Errorf("bad name: %v", err)
	}
	if _, err := w.Clone("solo", "initial"); !errors.Is(err, workspace.ErrReservedName) {
		t.Errorf("reserved name: %v", err)
	}
	if _, err := w.Clone("solo", "solo"); !errors.Is(err, preset.ErrExists) {
		t.Errorf("taken name: %v", err)
	}
}

// ---- rename ----

func TestRenameMovesThePresetAndKeepsItsContent(t *testing.T) {
	w := setup(t)

	if err := w.Rename("solo", "alpha"); err != nil {
		t.Fatal(err)
	}

	if w.Store().Exists("solo") {
		t.Error("the old folder is still there")
	}
	got, err := w.Store().Load("alpha")
	if err != nil || got.Preset.Name != "alpha" || got.Preset.Title != "alpha" || len(got.Agents) != 2 || got.Preset.Description != "Solo work" {
		t.Fatalf("Load() = %+v, %v", got, err)
	}
}

func TestRenameOfTheEquippedPresetUpdatesTheState(t *testing.T) {
	w := setup(t)
	manifest := map[string]string{"agents/planner.md": "abc"}
	if err := apply.SaveState(w.Paths, apply.State{Active: "solo", Manifest: manifest}); err != nil {
		t.Fatal(err)
	}

	if err := w.Rename("solo", "alpha"); err != nil {
		t.Fatal(err)
	}

	st, _ := apply.LoadState(w.Paths)
	if st.Active != "alpha" || !reflect.DeepEqual(st.Manifest, manifest) {
		t.Errorf("state = %+v", st)
	}
}

func TestRenameRefusals(t *testing.T) {
	w := setup(t)
	if _, err := w.New("other"); err != nil {
		t.Fatal(err)
	}

	if err := w.Rename("initial", "x"); !errors.Is(err, workspace.ErrProtected) {
		t.Errorf("renaming Initial: %v", err)
	}
	if err := w.Rename("nope", "x"); !errors.Is(err, preset.ErrNotFound) {
		t.Errorf("unknown: %v", err)
	}
	if err := w.Rename("solo", "other"); !errors.Is(err, preset.ErrExists) {
		t.Errorf("taken: %v", err)
	}
	if err := w.Rename("solo", "initial"); !errors.Is(err, workspace.ErrReservedName) {
		t.Errorf("reserved: %v", err)
	}
	if err := w.Rename("solo", "Bad Name"); !errors.Is(err, workspace.ErrInvalidName) {
		t.Errorf("invalid: %v", err)
	}
	if !w.Store().Exists("solo") {
		t.Error("a refused rename must leave the preset where it was")
	}
}

// ---- remove ----

func TestRemoveDeletesAPreset(t *testing.T) {
	w := setup(t)

	wasActive, err := w.Remove("solo", false)

	if err != nil || wasActive || w.Store().Exists("solo") {
		t.Fatalf("Remove() = %v, %v", wasActive, err)
	}
}

func TestRemoveRefusesInitialAndTheEquippedPresetWithoutForce(t *testing.T) {
	w := setup(t)
	if err := apply.SaveState(w.Paths, apply.State{Active: "solo"}); err != nil {
		t.Fatal(err)
	}

	if _, err := w.Remove("initial", true); !errors.Is(err, workspace.ErrProtected) {
		t.Errorf("removing Initial: %v", err)
	}
	if _, err := w.Remove("solo", false); !errors.Is(err, workspace.ErrEquipped) {
		t.Errorf("removing the equipped preset: %v", err)
	}
	if !w.Store().Exists("solo") {
		t.Fatal("a refused removal must keep the preset")
	}
	wasActive, err := w.Remove("solo", true)
	if err != nil || !wasActive || w.Store().Exists("solo") {
		t.Errorf("forced removal = %v, %v", wasActive, err)
	}
	if _, err := w.Remove("nope", false); !errors.Is(err, preset.ErrNotFound) {
		t.Errorf("unknown: %v", err)
	}
}

func TestMutationsRefuseWhileAnotherSquadHoldsTheLock(t *testing.T) {
	w := setup(t)
	testutil.WriteFiles(t, w.Paths.Home, map[string]string{".lock": "pid 1"})

	_, newErr := w.New("x")
	renameErr := w.Rename("solo", "y")
	_, removeErr := w.Remove("solo", false)

	for name, err := range map[string]error{"new": newErr, "rename": renameErr, "remove": removeErr} {
		if err == nil || !strings.Contains(err.Error(), ".lock") {
			t.Errorf("%s: error = %v, want it to name the lock", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(w.Paths.PresetsDir(), "x")); err == nil {
		t.Error("nothing may be written while locked")
	}
}

// ---- save ----

func TestSaveReplacesAnExistingPreset(t *testing.T) {
	w := setup(t)
	b, _ := w.Store().Load("solo")
	b.Preset.Description = "Changed"

	if err := w.Save(*b); err != nil {
		t.Fatal(err)
	}

	got, _ := w.Store().Load("solo")
	if got.Preset.Description != "Changed" {
		t.Errorf("description = %q", got.Preset.Description)
	}
}

// ---- card ----

func richAgent(name, extra string) string {
	return "---\nname: " + name + "\ndescription: " + name + " agent. Use when " + name + " is needed.\ntools: Read\n" + extra + "---\nBody of " + name + ".\n"
}

func TestCardDescribesAPreset(t *testing.T) {
	w := setup(t)
	perms := &preset.Permissions{DefaultMode: "acceptEdits", Allow: []string{"Bash(npm run *)"}, Deny: []string{"Read(./.env)"}}
	err := w.Store().Save(preset.Bundle{
		Preset: preset.Preset{
			Schema: 1, Name: "rich", Title: "Rich Team", Description: "A team", Mode: preset.ModeExtend,
			Agents:   []string{"alpha", "beta"},
			Settings: preset.Settings{Model: "opus", EffortLevel: "high", Permissions: perms},
		},
		Agents: []preset.AgentFile{
			{Name: "alpha", Data: []byte(richAgent("alpha", "model: sonnet\neffort: low\npermissionMode: plan\ncolor: cyan\n"))},
			{Name: "beta", Data: []byte(richAgent("beta", ""))},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	c, err := w.Card("rich")

	if err != nil {
		t.Fatal(err)
	}
	if c.Name != "rich" || c.Title != "Rich Team" || c.Mode != preset.ModeExtend || c.Model != "opus" || c.Effort != "high" {
		t.Errorf("card = %+v", c)
	}
	if c.Permissions != "acceptEdits · 1 allow · 0 ask · 1 deny" || c.Main != "" {
		t.Errorf("permissions = %q, main = %q", c.Permissions, c.Main)
	}
	want := []workspace.AgentInfo{
		{Name: "alpha", Model: "sonnet", Effort: "low", PermissionMode: "plan", Color: "cyan"},
		{Name: "beta"},
	}
	if !reflect.DeepEqual(c.Agents, want) {
		t.Errorf("agents = %+v, want %+v", c.Agents, want)
	}
	if c.SessionTokens == 0 {
		t.Error("the cost estimate is missing")
	}
}

func TestCardCountsLintFindings(t *testing.T) {
	w := setup(t)
	err := w.Store().Save(preset.Bundle{
		Preset: preset.Preset{Schema: 1, Name: "messy", Mode: preset.ModeReplace, Agents: []string{"zed"}},
		Agents: []preset.AgentFile{{Name: "zed", Data: []byte(richAgent("zed", "model: gpt-5\n"))}},
	})
	if err != nil {
		t.Fatal(err)
	}

	c, err := w.Card("messy")

	if err != nil || c.Errors != 1 {
		t.Fatalf("card = %+v, %v: the bad model is one error", c, err)
	}
}

func TestCardOfInitialReadsTheSnapshot(t *testing.T) {
	w := setup(t)

	c, err := w.Card("initial")

	if err != nil {
		t.Fatal(err)
	}
	if !c.Initial || c.Model != "opus" || c.Effort != "high" || c.Main != "reviewer" || c.Permissions != "acceptEdits · 1 allow · 0 ask · 0 deny" {
		t.Errorf("card = %+v", c)
	}
	names := []string{}
	for _, a := range c.Agents {
		names = append(names, a.Name)
	}
	if !reflect.DeepEqual(names, []string{"reviewer", "tester"}) {
		t.Errorf("agents = %v: the broken one is left out", names)
	}
	if c.Errors != 1 {
		t.Errorf("Errors = %d, want the unreadable agent counted", c.Errors)
	}
}

func TestCardOfAnUnknownOrBrokenPreset(t *testing.T) {
	w := setup(t)
	testutil.WriteFiles(t, filepath.Join(w.Paths.PresetsDir(), "broken"), map[string]string{"preset.yaml": "schema: [oops"})

	if _, err := w.Card("nope"); !errors.Is(err, preset.ErrNotFound) {
		t.Errorf("unknown: %v", err)
	}
	c, err := w.Card("broken")
	if err != nil || c.Error == "" || c.Errors == 0 {
		t.Errorf("broken = %+v, %v: the card must say what is wrong instead of failing", c, err)
	}
}

// ---- capture ----

func TestCaptureStoresTheLiveSetupAsAPreset(t *testing.T) {
	w := setup(t)
	testutil.WriteFiles(t, w.Paths.ClaudeDir, map[string]string{"agents/extra.md": agentMD("extra", "Added later.")})

	skipped, err := w.Capture("live")

	if err != nil {
		t.Fatal(err)
	}
	if len(skipped) != 1 || skipped[0].Path != "broken.md" {
		t.Errorf("skipped = %+v", skipped)
	}
	got, err := w.Store().Load("live")
	if err != nil || !reflect.DeepEqual(got.Preset.Agents, []string{"extra", "reviewer", "tester"}) {
		t.Fatalf("Load() = %+v, %v", got, err)
	}
	if _, err := w.Capture("live"); !errors.Is(err, preset.ErrExists) {
		t.Errorf("capturing onto an existing name: %v", err)
	}
	if _, err := w.Capture("initial"); !errors.Is(err, workspace.ErrReservedName) {
		t.Errorf("capturing as initial: %v", err)
	}
}

// ---- probing models ----

func TestProbeModelsStoresTheResultsInTheCache(t *testing.T) {
	w := setup(t)
	testutil.FakeClaude(t)
	t.Setenv("FAKE_CLAUDE_UNAVAILABLE", "fable")

	cache, err := w.ProbeModels(context.Background())

	if err != nil {
		t.Fatal(err)
	}
	if !cache.Results["opus"].OK || cache.Results["opus"].ID != "claude-opus-5-5" || cache.Results["fable"].OK {
		t.Errorf("results = %+v", cache.Results)
	}
	if cache.ClaudeVersion != "2.1.0 (Claude Code)" || cache.AuthMethod != "claude.ai" || !cache.ProbedAt.Equal(now) {
		t.Errorf("cache = %+v", cache)
	}
	saved, err := models.LoadCache(w.Paths)
	if err != nil || !reflect.DeepEqual(saved.Results, cache.Results) {
		t.Errorf("saved = %+v, %v", saved, err)
	}
}

func TestProbeModelsExplainsWhatIsMissing(t *testing.T) {
	w := setup(t)

	testutil.HideClaude(t)
	if _, err := w.ProbeModels(context.Background()); !errors.Is(err, workspace.ErrNoClaude) {
		t.Errorf("without claude: %v", err)
	}
	testutil.FakeClaude(t)
	t.Setenv("FAKE_CLAUDE_LOGGED_IN", "0")
	if _, err := w.ProbeModels(context.Background()); !errors.Is(err, workspace.ErrNotLoggedIn) {
		t.Errorf("logged out: %v", err)
	}
}

func importBundle(name string) preset.Bundle {
	return preset.Bundle{
		Preset: preset.Preset{Schema: 1, Name: name, Title: name, Mode: preset.ModeReplace, Agents: []string{"scout"}},
		Agents: []preset.AgentFile{{Name: "scout", Data: []byte(agentMD("scout", "Scout."))}},
	}
}

func TestImportStoresAPresetUnderItsOwnName(t *testing.T) {
	w := setup(t)

	res, err := w.Import(importBundle("fresh"), workspace.ImportOptions{})

	if err != nil || res.Name != "fresh" || res.Renamed || res.Replaced {
		t.Fatalf("Import = %+v, %v", res, err)
	}
	if !w.Store().Exists("fresh") {
		t.Error("the preset was not stored")
	}
	if state, _ := apply.LoadState(w.Paths); state.Active != "" {
		t.Errorf("import must not equip anything, active = %q", state.Active)
	}
}

func TestImportPicksANewNameOnCollision(t *testing.T) {
	w := setup(t)

	res, err := w.Import(importBundle("solo"), workspace.ImportOptions{})
	if err != nil || res.Name != "solo-2" || !res.Renamed {
		t.Fatalf("Import = %+v, %v", res, err)
	}
	res, err = w.Import(importBundle("solo"), workspace.ImportOptions{})
	if err != nil || res.Name != "solo-3" {
		t.Fatalf("second Import = %+v, %v", res, err)
	}
	got, _ := w.Store().LoadPreset("solo-2")
	if got.Name != "solo-2" {
		t.Errorf("the stored preset is still called %q", got.Name)
	}
	if kept, _ := w.Store().LoadPreset("solo"); kept.Description != "Solo work" {
		t.Error("the existing preset was touched")
	}
}

func TestImportNeverTakesTheReservedName(t *testing.T) {
	w := setup(t)
	res, err := w.Import(importBundle("initial"), workspace.ImportOptions{})
	if err != nil || res.Name != "initial-2" {
		t.Fatalf("Import = %+v, %v", res, err)
	}
}

func TestImportAsGivesTheName(t *testing.T) {
	w := setup(t)

	res, err := w.Import(importBundle("fresh"), workspace.ImportOptions{As: "mine"})
	if err != nil || res.Name != "mine" || !res.Renamed {
		t.Fatalf("Import = %+v, %v", res, err)
	}
	if _, err := w.Import(importBundle("fresh"), workspace.ImportOptions{As: "solo"}); !errors.Is(err, preset.ErrExists) {
		t.Errorf("--as onto an existing preset: %v", err)
	}
	if _, err := w.Import(importBundle("fresh"), workspace.ImportOptions{As: "Not Valid"}); !errors.Is(err, workspace.ErrInvalidName) {
		t.Errorf("--as with a bad name: %v", err)
	}
}

func TestImportForceReplacesThePreset(t *testing.T) {
	w := setup(t)

	res, err := w.Import(importBundle("solo"), workspace.ImportOptions{Force: true})

	if err != nil || res.Name != "solo" || !res.Replaced {
		t.Fatalf("Import = %+v, %v", res, err)
	}
	got, _ := w.Store().Load("solo")
	if len(got.Agents) != 1 || got.Agents[0].Name != "scout" {
		t.Errorf("solo was not replaced: %+v", got.Agents)
	}
}

func TestImportForceOnANewNameJustStores(t *testing.T) {
	w := setup(t)
	res, err := w.Import(importBundle("fresh"), workspace.ImportOptions{Force: true})
	if err != nil || res.Replaced || res.Name != "fresh" {
		t.Fatalf("Import = %+v, %v", res, err)
	}
}

func TestNewFromTemplateStoresACopyUnderTheGivenName(t *testing.T) {
	w := setup(t)

	if err := w.NewFromTemplate("bug-hunt", "my-hunt"); err != nil {
		t.Fatal(err)
	}

	b, err := w.Store().Load("my-hunt")
	if err != nil {
		t.Fatal(err)
	}
	if b.Preset.Title != "Bug Hunt" || len(b.Agents) != 2 || b.Preset.Name != "my-hunt" {
		t.Errorf("preset = %+v, %d agents", b.Preset, len(b.Agents))
	}
	if err := w.NewFromTemplate("bug-hunt", "my-hunt"); !errors.Is(err, preset.ErrExists) {
		t.Errorf("second copy: %v", err)
	}
	if err := w.NewFromTemplate("bug-hunt", "initial"); !errors.Is(err, workspace.ErrReservedName) {
		t.Errorf("reserved name: %v", err)
	}
	if err := w.NewFromTemplate("nope", "x"); err == nil {
		t.Error("unknown template accepted")
	}
}

func TestFindAgentLooksInTheNamedPresetThenTheEquippedOneThenEverywhere(t *testing.T) {
	w := setup(t)

	// Named preset.
	name, data, err := w.FindAgent("solo", "planner")
	if err != nil || name != "solo" || !strings.Contains(string(data), "Plan it.") {
		t.Fatalf("named: %q, %v", name, err)
	}
	if _, _, err := w.FindAgent("solo", "ghost"); !errors.Is(err, workspace.ErrNoAgent) {
		t.Errorf("missing agent in a named preset: %v", err)
	}
	if _, _, err := w.FindAgent("nope", "planner"); !errors.Is(err, preset.ErrNotFound) {
		t.Errorf("missing preset: %v", err)
	}

	// Nothing equipped: a unique match anywhere is fine.
	if name, _, err := w.FindAgent("", "planner"); err != nil || name != "solo" {
		t.Errorf("unique: %q, %v", name, err)
	}
	if _, _, err := w.FindAgent("", "ghost"); !errors.Is(err, workspace.ErrNoAgent) {
		t.Errorf("unknown: %v", err)
	}

	// Two presets have a tester: ambiguous until one is equipped.
	if _, err := w.Import(importBundle("other"), workspace.ImportOptions{}); err != nil {
		t.Fatal(err)
	}
	other := preset.Bundle{
		Preset: preset.Preset{Schema: 1, Name: "second", Mode: preset.ModeReplace, Agents: []string{"tester"}},
		Agents: []preset.AgentFile{{Name: "tester", Data: []byte(agentMD("tester", "Second tester."))}},
	}
	if err := w.Store().Save(other); err != nil {
		t.Fatal(err)
	}
	_, _, err = w.FindAgent("", "tester")
	var amb *workspace.AmbiguousAgentError
	if !errors.As(err, &amb) || len(amb.Presets) != 2 {
		t.Fatalf("ambiguous: %v", err)
	}
	if err := apply.SaveState(w.Paths, apply.State{Active: "second"}); err != nil {
		t.Fatal(err)
	}
	if name, data, err := w.FindAgent("", "tester"); err != nil || name != "second" || !strings.Contains(string(data), "Second tester.") {
		t.Errorf("equipped: %q, %v", name, err)
	}
}

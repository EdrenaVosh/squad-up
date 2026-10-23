package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/EdrenaVosh/squadup/internal/apply"
	"github.com/EdrenaVosh/squadup/internal/fsx"
	"github.com/EdrenaVosh/squadup/internal/paths"
	"github.com/EdrenaVosh/squadup/internal/testutil"
)

func (e *env) paths() paths.Paths { return paths.Paths{ClaudeDir: e.claudeDir, Home: e.home} }

// ---- list ----

func TestListShowsInitialAsActiveBeforeAnythingIsApplied(t *testing.T) {
	e := newEnv(t)
	e.writeSetup()

	r := e.run("list")

	if !strings.Contains(r.stdout, "your original setup · captured 2026-10-24") {
		t.Errorf("stdout lacks the Initial label:\n%s", r.stdout)
	}
	if !strings.Contains(r.stdout, "● initial") {
		t.Errorf("Initial must be marked active:\n%s", r.stdout)
	}
	if !strings.Contains(r.stdout, "No presets yet") {
		t.Errorf("stdout lacks the empty-state hint:\n%s", r.stdout)
	}
}

func TestListShowsPresetsAboveInitialAndMarksTheActiveOne(t *testing.T) {
	e := newEnv(t)
	e.writeSetup()
	e.run("capture", "mine")
	e.run("capture", "alpha")
	if err := apply.SaveState(e.paths(), apply.State{Active: "mine"}); err != nil {
		t.Fatal(err)
	}

	r := e.run("ls")

	if r.code != 0 {
		t.Fatalf("exit code = %d\n%s", r.code, r.stderr)
	}
	lines := strings.Split(r.stdout, "\n")
	idx := func(sub string) int {
		for i, l := range lines {
			if strings.Contains(l, sub) {
				return i
			}
		}
		t.Fatalf("no line contains %q:\n%s", sub, r.stdout)
		return -1
	}
	if idx("alpha") >= idx("mine") || idx("mine") >= idx("initial") {
		t.Errorf("order must be alphabetical presets, then Initial:\n%s", r.stdout)
	}
	if !strings.Contains(lines[idx("mine")], "●") || !strings.Contains(lines[idx("alpha")], "○") || !strings.Contains(lines[idx("initial")], "◇") {
		t.Errorf("markers are wrong:\n%s", r.stdout)
	}
	if !strings.Contains(lines[idx("mine")], "3 agents") {
		t.Errorf("agent count missing:\n%s", r.stdout)
	}
}

func TestListJSON(t *testing.T) {
	e := newEnv(t)
	e.writeSetup()
	e.run("capture", "mine")

	r := e.run("list", "--json")

	var got []struct {
		Name       string `json:"name"`
		Title      string `json:"title"`
		Agents     int    `json:"agents"`
		Active     bool   `json:"active"`
		Initial    bool   `json:"initial"`
		CapturedAt string `json:"capturedAt"`
	}
	if err := json.Unmarshal([]byte(r.stdout), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, r.stdout)
	}
	if len(got) != 2 || got[0].Name != "mine" || got[0].Agents != 3 || got[0].Active || got[0].Initial {
		t.Errorf("first entry = %+v", got)
	}
	if got[1].Name != "initial" || !got[1].Initial || !got[1].Active || got[1].CapturedAt != "2026-10-24T09:30:00Z" {
		t.Errorf("initial entry = %+v", got[1])
	}
}

func TestListToleratesAnUnreadablePreset(t *testing.T) {
	e := newEnv(t)
	e.writeSetup()
	e.run("capture", "mine")
	testutil.WriteFiles(t, filepath.Join(e.home, "presets", "broken"), map[string]string{"preset.yaml": "schema: [oops"})

	r := e.run("list")

	if r.code != 0 {
		t.Fatalf("exit code = %d: one broken preset must not hide the rest\n%s", r.code, r.stderr)
	}
	if !strings.Contains(r.stdout, "mine") || !strings.Contains(r.stdout, "broken") || !strings.Contains(r.stdout, "unreadable") {
		t.Errorf("stdout = %s", r.stdout)
	}
}

// ---- capture ----

func TestCaptureSavesCurrentSetupAsPreset(t *testing.T) {
	e := newEnv(t)
	e.writeSetup()

	r := e.run("capture", "mine")

	if r.code != 0 {
		t.Fatalf("exit code = %d\n%s", r.code, r.stderr)
	}
	if !strings.Contains(r.stdout, `Captured preset "mine": 3 agents (docs-writer · reviewer · tester)`) {
		t.Errorf("stdout = %s", r.stdout)
	}
	if !strings.Contains(r.stdout, "squad use mine") {
		t.Errorf("stdout lacks the next step:\n%s", r.stdout)
	}
	got, err := os.ReadFile(filepath.Join(e.home, "presets", "mine", "agents", "tester.md"))
	if err != nil || string(got) != testerMD {
		t.Errorf("tester.md = %q, %v", got, err)
	}
	yaml, _ := os.ReadFile(filepath.Join(e.home, "presets", "mine", "preset.yaml"))
	if !strings.Contains(string(yaml), "model: opus") {
		t.Errorf("preset.yaml lacks the model:\n%s", yaml)
	}
}

func TestCaptureReportsSkippedAgents(t *testing.T) {
	e := newEnv(t)
	testutil.WriteFiles(t, e.claudeDir, map[string]string{
		"agents/good.md":   reviewerMD,
		"agents/broken.md": "no frontmatter",
	})

	r := e.run("capture", "mine")

	if r.code != 0 {
		t.Fatalf("exit code = %d\n%s", r.code, r.stderr)
	}
	if !strings.Contains(r.stderr, "Skipped broken.md") || !strings.Contains(r.stdout, "1 agent") {
		t.Errorf("stdout:\n%s\nstderr:\n%s", r.stdout, r.stderr)
	}
}

func TestCaptureRejectsBadNamesWithExitCode2(t *testing.T) {
	e := newEnv(t)

	for _, name := range []string{"Bad Name", "../x", "initial", "a_b"} {
		r := e.run("capture", name)
		if r.code != 2 {
			t.Errorf("capture %q: exit code = %d, want 2\n%s", name, r.code, r.stderr)
		}
		if !strings.Contains(r.stderr, "Fix:") {
			t.Errorf("capture %q: stderr lacks a fix hint:\n%s", name, r.stderr)
		}
	}
}

func TestCaptureRefusesToOverwriteAPreset(t *testing.T) {
	e := newEnv(t)
	e.writeSetup()
	e.run("capture", "mine")

	r := e.run("capture", "mine")

	if r.code != 1 || !strings.Contains(r.stderr, "already exists") {
		t.Fatalf("exit code = %d, stderr:\n%s", r.code, r.stderr)
	}
}

func TestCaptureRequiresAName(t *testing.T) {
	e := newEnv(t)

	r := e.run("capture")

	if r.code != 1 {
		t.Fatalf("exit code = %d, want 1", r.code)
	}
}

// ---- status ----

func TestStatusShowsActivePresetClaudeAndNoDrift(t *testing.T) {
	e := newEnv(t)
	e.writeSetup()
	testutil.FakeClaude(t)
	t.Setenv("FAKE_CLAUDE_VERSION", "2.7.1 (Claude Code)")

	r := e.run("status")

	if r.code != 0 {
		t.Fatalf("exit code = %d\n%s", r.code, r.stderr)
	}
	for _, want := range []string{
		"Active preset  initial · your original setup",
		"Drift          none",
		"Claude         2.7.1 (Claude Code) · firstParty",
		"Config dir     " + e.claudeDir,
		"Squadup home   " + e.home,
	} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("stdout lacks %q:\n%s", want, r.stdout)
		}
	}
}

func TestStatusReportsManualEdits(t *testing.T) {
	e := newEnv(t)
	e.writeSetup()
	testutil.FakeClaude(t)
	e.run("list")
	testutil.WriteFiles(t, e.claudeDir, map[string]string{"agents/changed.md": "edited by hand"})
	err := apply.SaveState(e.paths(), apply.State{Active: "mine", Manifest: map[string]string{
		"agents/changed.md": fsx.SHA256Hex([]byte("as written")),
		"agents/gone.md":    fsx.SHA256Hex([]byte("was here")),
	}})
	if err != nil {
		t.Fatal(err)
	}

	r := e.run("status")

	for _, want := range []string{"Active preset  mine", "Manual edits detected", "agents/changed.md (modified)", "agents/gone.md (missing)"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("stdout lacks %q:\n%s", want, r.stdout)
		}
	}
}

func TestStatusWithoutClaudeStillWorks(t *testing.T) {
	e := newEnv(t)
	testutil.EmptyPath(t)

	r := e.run("status")

	if r.code != 0 {
		t.Fatalf("exit code = %d\n%s", r.code, r.stderr)
	}
	if !strings.Contains(r.stdout, "not found — run squad doctor") {
		t.Errorf("stdout = %s", r.stdout)
	}
}

func TestStatusWarnsWhenAnthropicModelOverridesPresets(t *testing.T) {
	e := newEnv(t)
	testutil.FakeClaude(t)
	t.Setenv("ANTHROPIC_MODEL", "sonnet")

	r := e.run("status")

	if !strings.Contains(r.stdout, "ANTHROPIC_MODEL is set (sonnet): it overrides the model of every preset") {
		t.Errorf("stdout lacks the warning:\n%s", r.stdout)
	}
}

func TestStatusJSON(t *testing.T) {
	e := newEnv(t)
	testutil.FakeClaude(t)

	r := e.run("status", "--json")

	var got struct {
		Active string `json:"active"`
		Drift  []any  `json:"drift"`
		Claude struct {
			Found    bool   `json:"found"`
			Version  string `json:"version"`
			Provider string `json:"provider"`
			LoggedIn bool   `json:"loggedIn"`
		} `json:"claude"`
		ClaudeDir string `json:"claudeDir"`
		Home      string `json:"home"`
	}
	if err := json.Unmarshal([]byte(r.stdout), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, r.stdout)
	}
	if got.Active != "initial" || got.Drift == nil || len(got.Drift) != 0 {
		t.Errorf("active/drift = %q, %v", got.Active, got.Drift)
	}
	if !got.Claude.Found || got.Claude.Version != "2.1.0 (Claude Code)" || got.Claude.Provider != "firstParty" || !got.Claude.LoggedIn {
		t.Errorf("claude = %+v", got.Claude)
	}
	if got.ClaudeDir != e.claudeDir || got.Home != e.home {
		t.Errorf("dirs = %q, %q", got.ClaudeDir, got.Home)
	}
}

// ---- initial ----

func TestInitialShowsTheSavedSnapshot(t *testing.T) {
	e := newEnv(t)
	e.writeSetup()

	r := e.run("initial")

	if r.code != 0 {
		t.Fatalf("exit code = %d\n%s", r.code, r.stderr)
	}
	for _, want := range []string{
		"◇ Initial — your original setup, captured 2026-10-24 09:30 UTC",
		"  Agents       3   docs-writer · reviewer · tester",
		"  Model        opus",
	} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("stdout lacks %q:\n%s", want, r.stdout)
		}
	}
}

func TestRecaptureNeedsConfirmationWithoutATerminal(t *testing.T) {
	e := newEnv(t)
	e.writeSetup()
	e.run("list")

	r := e.run("initial", "--recapture")

	if r.code != 1 || !strings.Contains(r.stderr, "--yes") {
		t.Fatalf("exit code = %d, stderr:\n%s", r.code, r.stderr)
	}
}

func TestRecaptureWithYesReplacesInitialAndKeepsTheOldOne(t *testing.T) {
	e := newEnv(t)
	e.writeSetup()
	e.run("list")
	testutil.WriteFiles(t, e.claudeDir, map[string]string{"settings.json": `{"model": "haiku"}`})

	r := e.run("initial", "--recapture", "--yes")

	if r.code != 0 {
		t.Fatalf("exit code = %d\n%s", r.code, r.stderr)
	}
	if !strings.Contains(r.stdout, "Recaptured Initial") || !strings.Contains(r.stdout, filepath.Join(e.home, "backups")) {
		t.Errorf("stdout = %s", r.stdout)
	}
	shown := e.run("initial")
	if !strings.Contains(shown.stdout, "Model        haiku") {
		t.Errorf("Initial was not replaced:\n%s", shown.stdout)
	}
	entries, _ := os.ReadDir(filepath.Join(e.home, "backups"))
	if len(entries) != 1 {
		t.Errorf("backups = %v, want exactly the old snapshot", entries)
	}
}

func TestRecaptureAsksOnATerminal(t *testing.T) {
	e := newEnv(t)
	e.writeSetup()
	e.run("list")
	testutil.WriteFiles(t, e.claudeDir, map[string]string{"settings.json": `{"model": "haiku"}`})

	declined := e.runWith("n\n", true, "initial", "--recapture")
	if declined.code != 3 || !strings.Contains(declined.stderr, "Cancelled") {
		t.Fatalf("declined: exit code = %d, stderr:\n%s", declined.code, declined.stderr)
	}
	if got := e.run("initial").stdout; !strings.Contains(got, "Model        opus") {
		t.Fatalf("declining must keep Initial:\n%s", got)
	}

	accepted := e.runWith("y\n", true, "initial", "--recapture")
	if accepted.code != 0 || !strings.Contains(accepted.stderr, "Replace Initial with your current setup?") {
		t.Fatalf("accepted: exit code = %d, stderr:\n%s", accepted.code, accepted.stderr)
	}
	if got := e.run("initial").stdout; !strings.Contains(got, "Model        haiku") {
		t.Fatalf("accepting must replace Initial:\n%s", got)
	}
}

func TestRecaptureIsRefusedWhileAPresetIsEquipped(t *testing.T) {
	e := newEnv(t)
	e.writeSetup()
	e.run("list")
	if err := apply.SaveState(e.paths(), apply.State{Active: "mine"}); err != nil {
		t.Fatal(err)
	}

	r := e.run("initial", "--recapture", "--yes")

	if r.code != 1 || !strings.Contains(r.stderr, "squad restore") {
		t.Fatalf("exit code = %d, stderr:\n%s", r.code, r.stderr)
	}
}

package initial_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/EdrenaVosh/squadup/internal/initial"
	"github.com/EdrenaVosh/squadup/internal/paths"
	"github.com/EdrenaVosh/squadup/internal/preset"
	"github.com/EdrenaVosh/squadup/internal/testutil"
)

const settingsJSON = `{
  "theme": "dark",
  "model": "opus",
  "env": { "SECRET": "keep-out" },
  "permissions": {
    "defaultMode": "acceptEdits",
    "allow": ["Bash(npm run *)", "Bash(git status)", "Bash(git diff *)", "Read(./docs/**)"],
    "ask": ["Bash(git push *)"],
    "deny": ["Read(./.env)", "Read(./.env.*)"]
  },
  "effortLevel": "high"
}
`

const reviewer = "---\r\nname: reviewer\r\ndescription: Reviews diffs. Use after any code change.\r\ncolor: green\r\n---\r\nBe strict.\r\n"

var captureTime = time.Date(2026, 10, 24, 9, 30, 0, 0, time.UTC)

func fixture(t *testing.T, files map[string]string) paths.Paths {
	t.Helper()
	p := paths.Paths{ClaudeDir: filepath.Join(t.TempDir(), "claude"), Home: filepath.Join(t.TempDir(), "squadup")}
	testutil.WriteFiles(t, p.ClaudeDir, files)
	return p
}

func foreignSetup() map[string]string {
	return map[string]string{
		"agents/reviewer.md":       reviewer,
		"agents/team/tester.md":    "---\nname: tester\ndescription: Writes tests. Use when coverage is missing.\n---\nTest it.",
		"agents/docs-writer.md":    "---\nname: docs-writer\ndescription: Updates docs. Use after changes.\n---\n",
		"agents/notes.txt":         "not an agent",
		"agents/.DS_Store":         "junk",
		"settings.json":            settingsJSON,
		"projects/x/session.jsonl": "{}",
	}
}

func TestCaptureCopiesAgentsByteForByteAndLeavesConfigUntouched(t *testing.T) {
	p := fixture(t, foreignSetup())
	before := testutil.Tree(t, p.ClaudeDir)

	if _, err := initial.Capture(p, captureTime); err != nil {
		t.Fatalf("Capture() error = %v", err)
	}

	if after := testutil.Tree(t, p.ClaudeDir); !reflect.DeepEqual(before, after) {
		t.Fatal("Capture() modified the Claude config directory")
	}
	for rel, want := range map[string]string{
		"agents/reviewer.md":    reviewer,
		"agents/team/tester.md": foreignSetup()["agents/team/tester.md"],
		"agents/docs-writer.md": foreignSetup()["agents/docs-writer.md"],
	} {
		got, err := os.ReadFile(filepath.Join(p.InitialDir(), filepath.FromSlash(rel)))
		if err != nil || string(got) != want {
			t.Errorf("%s = %q, %v; want %q", rel, got, err, want)
		}
	}
	if _, err := os.Stat(filepath.Join(p.InitialDir(), "agents", "notes.txt")); err == nil {
		t.Error("non-markdown files must not be captured")
	}
}

func TestCaptureRecordsManagedKeysAndOriginalSettings(t *testing.T) {
	p := fixture(t, foreignSetup())
	if _, err := initial.Capture(p, captureTime); err != nil {
		t.Fatal(err)
	}

	meta, err := initial.Load(p)

	if err != nil {
		t.Fatal(err)
	}
	if meta.Managed["model"] != `"opus"` || meta.Managed["effortLevel"] != `"high"` {
		t.Errorf("Managed = %v", meta.Managed)
	}
	if _, ok := meta.Managed["agent"]; ok {
		t.Error("absent key `agent` must stay absent")
	}
	if !strings.HasPrefix(meta.Managed["permissions"], "{\n    \"defaultMode\"") {
		t.Errorf("permissions raw = %q, want original formatting", meta.Managed["permissions"])
	}
	data, existed, err := initial.OriginalSettings(p)
	if err != nil || !existed || string(data) != settingsJSON {
		t.Errorf("OriginalSettings() = %q, %v, %v", data, existed, err)
	}
	if !meta.SettingsFileExisted || !meta.AgentsDirExisted || !meta.CapturedAt.Equal(captureTime) {
		t.Errorf("meta = %+v", meta)
	}
}

func TestCaptureOfCleanSetup(t *testing.T) {
	p := paths.Paths{ClaudeDir: filepath.Join(t.TempDir(), "missing"), Home: filepath.Join(t.TempDir(), "squadup")}

	meta, err := initial.Capture(p, captureTime)

	if err != nil {
		t.Fatal(err)
	}
	if meta.AgentsDirExisted || meta.SettingsFileExisted {
		t.Errorf("meta = %+v, want both existence flags false", meta)
	}
	s := meta.Summary()
	if !s.Clean || len(s.AgentNames) != 0 || s.Model != "" || s.Permissions != "" {
		t.Errorf("Summary() = %+v, want a clean summary", s)
	}
	if _, existed, _ := initial.OriginalSettings(p); existed {
		t.Error("no settings.json existed, none must be stored")
	}
}

func TestCaptureRecordsExistingEmptyAgentsDir(t *testing.T) {
	p := fixture(t, nil)
	if err := os.MkdirAll(p.AgentsDir(), 0o755); err != nil {
		t.Fatal(err)
	}

	meta, err := initial.Capture(p, captureTime)

	if err != nil {
		t.Fatal(err)
	}
	if !meta.AgentsDirExisted || meta.SettingsFileExisted {
		t.Errorf("meta = %+v", meta)
	}
	if !meta.Summary().Clean {
		t.Error("an empty agents dir with no settings is still a clean setup")
	}
}

func TestCaptureKeepsBrokenAgentsByteForByteAndFlagsThem(t *testing.T) {
	files := map[string]string{
		"agents/good.md":        "---\nname: good\ndescription: Use when needed.\n---\nok",
		"agents/broken-yaml.md": "---\nname: [oops\n---\nbody",
		"agents/no-front.md":    "plain text\n",
		"agents/bad-name.md":    "---\nname: Bad_Name\ndescription: d\n---\nbody",
	}
	p := fixture(t, files)

	meta, err := initial.Capture(p, captureTime)

	if err != nil {
		t.Fatal(err)
	}
	for rel, want := range files {
		got, err := os.ReadFile(filepath.Join(p.InitialDir(), filepath.FromSlash(rel)))
		if err != nil || string(got) != want {
			t.Errorf("%s not preserved: %q, %v", rel, got, err)
		}
	}
	s := meta.Summary()
	if s.Issues != 3 || !reflect.DeepEqual(s.AgentNames, []string{"good"}) || s.Clean {
		t.Errorf("Summary() = %+v, want 3 issues, only `good` listed, not clean", s)
	}
}

func TestCaptureSurvivesInvalidSettingsJSON(t *testing.T) {
	p := fixture(t, map[string]string{"settings.json": `{"model": `})

	meta, err := initial.Capture(p, captureTime)

	if err != nil {
		t.Fatalf("Capture() error = %v: Initial must store a broken config as it is", err)
	}
	data, existed, _ := initial.OriginalSettings(p)
	if !existed || string(data) != `{"model": ` {
		t.Errorf("OriginalSettings() = %q, %v", data, existed)
	}
	if !meta.SettingsInvalid || meta.Summary().Issues != 1 {
		t.Errorf("meta = %+v, summary = %+v", meta, meta.Summary())
	}
}

func TestCaptureWritesPresetViewOfTheSetup(t *testing.T) {
	p := fixture(t, foreignSetup())
	if _, err := initial.Capture(p, captureTime); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(p.InitialDir(), "preset.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := preset.Unmarshal(data)

	if err != nil {
		t.Fatalf("preset.yaml does not parse: %v\n%s", err, data)
	}
	if got.Name != "initial" || got.Mode != "replace" || got.Settings.Model != "opus" || got.Settings.EffortLevel != "high" {
		t.Errorf("preset = %+v", got)
	}
	if !reflect.DeepEqual(got.Agents, []string{"docs-writer", "reviewer", "tester"}) {
		t.Errorf("agents = %v", got.Agents)
	}
	if got.Settings.Permissions == nil || len(got.Settings.Permissions.Allow) != 4 {
		t.Errorf("permissions = %+v", got.Settings.Permissions)
	}
	if err := got.Validate(); err != nil {
		t.Errorf("Validate() = %v", err)
	}
}

func TestSummaryDescribesTheSetup(t *testing.T) {
	p := fixture(t, foreignSetup())
	meta, err := initial.Capture(p, captureTime)
	if err != nil {
		t.Fatal(err)
	}

	s := meta.Summary()

	want := initial.Summary{
		AgentNames:  []string{"docs-writer", "reviewer", "tester"},
		Model:       "opus",
		Effort:      "high",
		Permissions: "acceptEdits · 4 allow · 1 ask · 2 deny",
	}
	if !reflect.DeepEqual(s, want) {
		t.Fatalf("Summary() = %+v\nwant %+v", s, want)
	}
}

func TestSummaryReportsMainAgent(t *testing.T) {
	p := fixture(t, map[string]string{"settings.json": `{"agent": "reviewer"}`})
	meta, err := initial.Capture(p, captureTime)
	if err != nil {
		t.Fatal(err)
	}

	if got := meta.Summary().Main; got != "reviewer" {
		t.Fatalf("Main = %q", got)
	}
}

func TestCaptureRefusesWhenInitialExists(t *testing.T) {
	p := fixture(t, foreignSetup())
	if _, err := initial.Capture(p, captureTime); err != nil {
		t.Fatal(err)
	}

	_, err := initial.Capture(p, captureTime)

	if !errors.Is(err, initial.ErrExists) {
		t.Fatalf("Capture() error = %v, want ErrExists", err)
	}
}

func TestLoadBeforeCaptureReportsNotCaptured(t *testing.T) {
	p := fixture(t, nil)

	_, err := initial.Load(p)

	if !errors.Is(err, initial.ErrNotCaptured) || initial.Exists(p) {
		t.Fatalf("Load() error = %v, Exists = %v", err, initial.Exists(p))
	}
}

func TestCaptureLeavesNoTempDirsAndMakesFilesReadOnly(t *testing.T) {
	p := fixture(t, foreignSetup())
	if _, err := initial.Capture(p, captureTime); err != nil {
		t.Fatal(err)
	}

	entries, _ := os.ReadDir(p.PresetsDir())
	if len(entries) != 1 || entries[0].Name() != "initial" {
		t.Errorf("presets dir = %v, want only initial", entries)
	}
	if runtime.GOOS == "windows" {
		return
	}
	info, err := os.Stat(filepath.Join(p.InitialDir(), "agents", "reviewer.md"))
	if err != nil || info.Mode().Perm() != 0o400 {
		t.Errorf("reviewer.md mode = %v, %v; want 0400", info.Mode().Perm(), err)
	}
}

func TestRecaptureBacksUpOldInitialAndTakesNewSnapshot(t *testing.T) {
	p := fixture(t, foreignSetup())
	if _, err := initial.Capture(p, captureTime); err != nil {
		t.Fatal(err)
	}
	testutil.WriteFiles(t, p.ClaudeDir, map[string]string{"settings.json": `{"model": "haiku"}`})

	backup, meta, err := initial.Recapture(p, captureTime.Add(time.Hour))

	if err != nil {
		t.Fatalf("Recapture() error = %v", err)
	}
	if meta.Summary().Model != "haiku" {
		t.Errorf("new Initial model = %q", meta.Summary().Model)
	}
	old, err := os.ReadFile(filepath.Join(backup, "initial.json"))
	if err != nil || !strings.Contains(string(old), `\"opus\"`) {
		t.Errorf("backup %s does not hold the previous snapshot: %v", backup, err)
	}
	if !strings.HasPrefix(backup, p.BackupsDir()) {
		t.Errorf("backup %s is outside %s", backup, p.BackupsDir())
	}
}

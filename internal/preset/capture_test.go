package preset_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/EdrenaVosh/squadup/internal/paths"
	"github.com/EdrenaVosh/squadup/internal/preset"
	"github.com/EdrenaVosh/squadup/internal/testutil"
)

const liveSettings = `{
  "theme": "dark",
  "model": "opus",
  "agent": "reviewer",
  "permissions": {
    "defaultMode": "acceptEdits",
    "allow": ["Bash(npm run *)"],
    "deny": ["Read(./.env)"],
    "disableBypassPermissionsMode": "disable"
  },
  "effortLevel": "high"
}
`

func configWith(t *testing.T, files map[string]string) paths.Paths {
	t.Helper()
	p := paths.Paths{ClaudeDir: filepath.Join(t.TempDir(), "claude"), Home: filepath.Join(t.TempDir(), "squadup")}
	testutil.WriteFiles(t, p.ClaudeDir, files)
	return p
}

func TestFromConfigBuildsPresetFromTheLiveSetup(t *testing.T) {
	reviewer := "---\r\nname: reviewer\r\ndescription: Reviews. Use after changes.\r\n---\r\nBe strict.\r\n"
	p := configWith(t, map[string]string{
		"agents/team/tester.md": "---\nname: tester\ndescription: Tests. Use when coverage drops.\n---\nTest.",
		"agents/reviewer.md":    reviewer,
		"settings.json":         liveSettings,
	})

	b, skipped, err := preset.FromConfig(p, "mine")

	if err != nil || len(skipped) != 0 {
		t.Fatalf("FromConfig() = %v, %v", skipped, err)
	}
	if err := b.Preset.Validate(); err != nil {
		t.Fatalf("Validate() = %v", err)
	}
	pr := b.Preset
	if pr.Name != "mine" || pr.Mode != preset.ModeReplace || pr.Settings.Model != "opus" || pr.Settings.EffortLevel != "high" {
		t.Errorf("preset = %+v", pr)
	}
	if pr.Main == nil || *pr.Main != "reviewer" {
		t.Errorf("Main = %v, want reviewer", pr.Main)
	}
	if !reflect.DeepEqual(pr.Agents, []string{"reviewer", "tester"}) {
		t.Errorf("Agents = %v", pr.Agents)
	}
	if perms := pr.Settings.Permissions; perms == nil || perms.DefaultMode != "acceptEdits" || perms.Extra["disableBypassPermissionsMode"] != "disable" {
		t.Errorf("permissions = %+v", perms)
	}
	if len(b.Agents) != 2 || b.Agents[0].Name != "reviewer" || string(b.Agents[0].Data) != reviewer {
		t.Errorf("agents = %q, want exact bytes of reviewer.md first", b.Agents)
	}
}

func TestFromConfigSkipsInvalidAndDuplicateAgentsWithReasons(t *testing.T) {
	p := configWith(t, map[string]string{
		"agents/a-good.md":  "---\nname: good\ndescription: Use when needed.\n---\nok",
		"agents/b-dup.md":   "---\nname: good\ndescription: Use when needed.\n---\nsecond",
		"agents/broken.md":  "---\nname: [oops\n---\n",
		"agents/nofront.md": "plain",
		"agents/Bad.md":     "---\nname: Bad_Name\ndescription: d\n---\n",
	})

	b, skipped, err := preset.FromConfig(p, "mine")

	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(b.Preset.Agents, []string{"good"}) || string(b.Agents[0].Data) != "---\nname: good\ndescription: Use when needed.\n---\nok" {
		t.Errorf("bundle = %+v, want only the first `good`", b)
	}
	if len(skipped) != 4 {
		t.Fatalf("skipped = %+v, want 4 entries", skipped)
	}
	for _, s := range skipped {
		if s.Path == "" || s.Reason == "" {
			t.Errorf("skipped entry %+v lacks path or reason", s)
		}
	}
	found := false
	for _, s := range skipped {
		if strings.Contains(s.Reason, "duplicate") {
			found = true
		}
	}
	if !found {
		t.Errorf("no skipped entry explains the duplicate name: %+v", skipped)
	}
}

func TestFromConfigOfEmptyConfigIsAValidEmptyPreset(t *testing.T) {
	p := configWith(t, nil)

	b, skipped, err := preset.FromConfig(p, "empty")

	if err != nil || len(skipped) != 0 {
		t.Fatalf("FromConfig() = %v, %v", skipped, err)
	}
	if err := b.Preset.Validate(); err != nil {
		t.Fatalf("Validate() = %v", err)
	}
	if len(b.Agents) != 0 || b.Preset.Main != nil || b.Preset.Settings.Model != "" {
		t.Errorf("bundle = %+v", b)
	}
}

func TestFromConfigFailsOnInvalidSettingsJSONNamingTheFile(t *testing.T) {
	p := configWith(t, map[string]string{"settings.json": `{"model": `})

	_, _, err := preset.FromConfig(p, "mine")

	if err == nil || !strings.Contains(err.Error(), p.SettingsFile()) {
		t.Fatalf("FromConfig() error = %v, want it to name %s", err, p.SettingsFile())
	}
}

func TestLoadPresetReadsOnlyPresetYAML(t *testing.T) {
	dir := t.TempDir()
	store := preset.Store{Dir: dir}
	if err := store.Save(bundle()); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(dir, "solo-indie", "agents")); err != nil {
		t.Fatal(err)
	}

	got, err := store.LoadPreset("solo-indie")

	if err != nil || got.Name != "solo-indie" || len(got.Agents) != 3 {
		t.Fatalf("LoadPreset() = %+v, %v", got, err)
	}
}

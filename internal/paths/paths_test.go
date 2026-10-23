package paths_test

import (
	"path/filepath"
	"testing"

	"github.com/EdrenaVosh/squadup/internal/paths"
)

func setHome(t *testing.T, home string) {
	t.Helper()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
}

func TestResolveDefaultsToHomeDirectories(t *testing.T) {
	home := t.TempDir()
	setHome(t, home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("SQUADUP_HOME", "")

	got, err := paths.Resolve("", "")

	if err != nil {
		t.Fatal(err)
	}
	if got.ClaudeDir != filepath.Join(home, ".claude") || got.Home != filepath.Join(home, ".squadup") {
		t.Fatalf("Resolve() = %+v", got)
	}
}

func TestResolvePrefersEnvOverDefault(t *testing.T) {
	setHome(t, t.TempDir())
	claudeDir, squadHome := t.TempDir(), t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)
	t.Setenv("SQUADUP_HOME", squadHome)

	got, err := paths.Resolve("", "")

	if err != nil {
		t.Fatal(err)
	}
	if got.ClaudeDir != claudeDir || got.Home != squadHome {
		t.Fatalf("Resolve() = %+v", got)
	}
}

func TestResolvePrefersFlagsOverEnv(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	t.Setenv("SQUADUP_HOME", t.TempDir())
	claudeDir, squadHome := t.TempDir(), t.TempDir()

	got, err := paths.Resolve(claudeDir, squadHome)

	if err != nil {
		t.Fatal(err)
	}
	if got.ClaudeDir != claudeDir || got.Home != squadHome {
		t.Fatalf("Resolve() = %+v", got)
	}
}

func TestResolveMakesRelativeFlagsAbsolute(t *testing.T) {
	got, err := paths.Resolve("rel/claude", "rel/home")

	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(got.ClaudeDir) || !filepath.IsAbs(got.Home) {
		t.Fatalf("Resolve() = %+v, want absolute paths", got)
	}
}

func TestDerivedLocations(t *testing.T) {
	p := paths.Paths{ClaudeDir: filepath.FromSlash("/c"), Home: filepath.FromSlash("/h")}

	cases := map[string]struct{ got, want string }{
		"AgentsDir":    {p.AgentsDir(), "/c/agents"},
		"SettingsFile": {p.SettingsFile(), "/c/settings.json"},
		"PresetsDir":   {p.PresetsDir(), "/h/presets"},
		"PresetDir":    {p.PresetDir("solo"), "/h/presets/solo"},
		"InitialDir":   {p.InitialDir(), "/h/presets/initial"},
		"StateFile":    {p.StateFile(), "/h/state.json"},
		"ConfigFile":   {p.ConfigFile(), "/h/config.json"},
		"BackupsDir":   {p.BackupsDir(), "/h/backups"},
		"CacheDir":     {p.CacheDir(), "/h/cache"},
		"LockFile":     {p.LockFile(), "/h/.lock"},
		"LogFile":      {p.LogFile(), "/h/log.jsonl"},
	}
	for name, c := range cases {
		if c.got != filepath.FromSlash(c.want) {
			t.Errorf("%s = %q, want %q", name, c.got, filepath.FromSlash(c.want))
		}
	}
}

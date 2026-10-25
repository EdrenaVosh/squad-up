package cli_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

func (e *env) settings() string {
	e.t.Helper()
	data, err := os.ReadFile(filepath.Join(e.claudeDir, "settings.json"))
	if err != nil {
		e.t.Fatal(err)
	}
	return string(data)
}

func TestStatuslineEnableAndDisableRoundTrip(t *testing.T) {
	e := newEnv(t)
	e.writeSetup()
	e.addPresets()
	before := e.settings()

	r := e.run("statusline", "enable")
	if r.code != 0 {
		t.Fatalf("enable: %d %s", r.code, r.stderr)
	}
	got := gjson.Get(e.settings(), "statusLine.command").String()
	if !strings.Contains(got, " statusline") || gjson.Get(e.settings(), "statusLine.type").String() != "command" {
		t.Errorf("statusLine = %s", gjson.Get(e.settings(), "statusLine").Raw)
	}
	for _, want := range []string{"enabled", "squad statusline disable"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("stdout should mention %q:\n%s", want, r.stdout)
		}
	}
	if r := e.run("statusline", "enable"); r.code != 0 || !strings.Contains(r.stdout, "already") {
		t.Errorf("second enable: %d %q", r.code, r.stdout)
	}
	if r := e.run("statusline", "status"); r.code != 0 || !strings.Contains(r.stdout, "enabled") {
		t.Errorf("status: %d %q", r.code, r.stdout)
	}

	r = e.run("statusline", "disable")
	if r.code != 0 || !strings.Contains(r.stdout, "removed") && !strings.Contains(r.stdout, "back") {
		t.Fatalf("disable: %d %q %q", r.code, r.stdout, r.stderr)
	}
	if after := e.settings(); after != before {
		t.Errorf("settings.json is not what it was:\n%s", after)
	}
	if r := e.run("statusline", "status"); !strings.Contains(r.stdout, "not enabled") {
		t.Errorf("status after disable: %q", r.stdout)
	}
}

func TestStatuslineDisableWhenNeverEnabled(t *testing.T) {
	e := newEnv(t)
	e.writeSetup()

	r := e.run("statusline", "disable")

	if r.code != 1 || !strings.Contains(r.stderr, "not enabled") || !strings.Contains(r.stderr, "Fix:") {
		t.Errorf("%d %q", r.code, r.stderr)
	}
}

func TestStatuslineEnableKeepsTheUsersOwnLine(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the user's line is a shell command")
	}
	e := newEnv(t)
	t.Setenv("NO_COLOR", "1")
	e.writeSetup()
	settings := strings.Replace(settingsJSON, `"theme": "dark",`, `"theme": "dark",`+"\n  "+`"statusLine": {"type": "command", "command": "printf 'my line'"},`, 1)
	if err := os.WriteFile(filepath.Join(e.claudeDir, "settings.json"), []byte(settings), 0o600); err != nil {
		t.Fatal(err)
	}
	e.addPresets()
	if r := e.run("use", "solo", "--yes"); r.code != 0 {
		t.Fatalf("use: %s", r.stderr)
	}

	r := e.run("statusline", "enable")
	if r.code != 0 || !strings.Contains(r.stdout, "printf 'my line'") {
		t.Fatalf("enable: %d %q", r.code, r.stdout)
	}

	hook := e.runWith(`{"model":{"display_name":"Opus"}}`, false, "statusline")
	if hook.code != 0 || strings.TrimSpace(hook.stdout) != "my line ◆ solo" {
		t.Errorf("the line = %q (stderr %q)", hook.stdout, hook.stderr)
	}

	if r := e.run("statusline", "disable"); r.code != 0 {
		t.Fatalf("disable: %s", r.stderr)
	}
	if got := gjson.Get(e.settings(), "statusLine.command").String(); got != "printf 'my line'" {
		t.Errorf("the user's line is not back: %q", got)
	}
}

func TestStatuslineCommandShowsThePresetAndStaysQuiet(t *testing.T) {
	e := newEnv(t)
	e.addPresets()

	// A fresh home: it must not capture Initial or print a welcome.
	r := e.run("statusline")
	if r.code != 0 || r.stdout != "" || r.stderr != "" {
		t.Errorf("fresh: %d %q %q", r.code, r.stdout, r.stderr)
	}
	if _, err := os.Stat(filepath.Join(e.home, "presets", "initial")); err == nil {
		t.Error("the status line command captured Initial")
	}

	e.writeSetup()
	if r := e.run("use", "solo", "--yes"); r.code != 0 {
		t.Fatalf("use: %s", r.stderr)
	}
	r = e.run("statusline")
	if r.code != 0 || !strings.Contains(r.stdout, "◆ solo") {
		t.Errorf("%d %q", r.code, r.stdout)
	}

	t.Setenv("NO_COLOR", "1")
	if r := e.run("statusline"); strings.TrimSpace(r.stdout) != "◆ solo" {
		t.Errorf("NO_COLOR: %q", r.stdout)
	}
	t.Setenv("SQUAD_UP_RUN_PRESET", "bug-hunt")
	if r := e.run("statusline"); strings.TrimSpace(r.stdout) != "◆ bug-hunt (run)" {
		t.Errorf("run preset: %q", r.stdout)
	}
}

func TestStatuslineSurvivesUseAndDoesNotBlockRestore(t *testing.T) {
	e := newEnv(t)
	e.writeSetup()
	e.addPresets()
	if r := e.run("statusline", "enable"); r.code != 0 {
		t.Fatalf("enable: %s", r.stderr)
	}

	if r := e.run("use", "solo", "--yes"); r.code != 0 {
		t.Fatalf("use: %d %s", r.code, r.stderr)
	}
	if !gjson.Get(e.settings(), "statusLine.command").Exists() {
		t.Error("`squad use` removed the status line")
	}
	if r := e.run("status"); strings.Contains(r.stdout, "Manual edits") {
		t.Errorf("the status line counts as a manual edit:\n%s", r.stdout)
	}

	if r := e.run("restore", "--yes"); r.code != 0 {
		t.Fatalf("restore: %d %s", r.code, r.stderr)
	}
	if r := e.run("statusline", "status"); !strings.Contains(r.stdout, "enabled") {
		t.Errorf("status after restore: %q", r.stdout)
	}
	if r := e.run("statusline", "enable"); r.code != 0 {
		t.Fatalf("enable again: %s", r.stderr)
	}
	if r := e.run("statusline", "disable"); r.code != 0 {
		t.Fatalf("disable: %s", r.stderr)
	}
}

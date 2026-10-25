package statusline_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/tidwall/gjson"

	"github.com/EdrenaVosh/squad-up/internal/apply"
	"github.com/EdrenaVosh/squad-up/internal/paths"
	"github.com/EdrenaVosh/squad-up/internal/statusline"
)

var now = time.Date(2026, 10, 24, 9, 30, 0, 0, time.UTC)

const ours = "'/opt/squad' statusline"

func setup(t *testing.T, settings string) paths.Paths {
	t.Helper()
	p := paths.Paths{ClaudeDir: filepath.Join(t.TempDir(), "claude"), Home: filepath.Join(t.TempDir(), "squad-up")}
	if settings != "" {
		if err := os.MkdirAll(p.ClaudeDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p.SettingsFile(), []byte(settings), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return p
}

func settingsOf(t *testing.T, p paths.Paths) string {
	t.Helper()
	data, err := os.ReadFile(p.SettingsFile())
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

const plain = "{\n  \"theme\": \"dark\",\n  \"model\": \"opus\",\n  \"permissions\": {\n    \"allow\": [\"Read\"]\n  }\n}\n"

const withLine = "{\n  \"theme\": \"dark\",\n  \"statusLine\": {\n    \"type\": \"command\",\n    \"command\": \"~/.claude/line.sh\",\n    \"padding\": 2\n  },\n  \"model\": \"opus\"\n}\n"

func TestEnableAddsOurCommandAndDisableRemovesItByteForByte(t *testing.T) {
	p := setup(t, plain)

	out, err := statusline.Enable(p, ours, now)
	if err != nil || out != statusline.Enabled {
		t.Fatalf("Enable = %v, %v", out, err)
	}
	got := settingsOf(t, p)
	if gjson.Get(got, "statusLine.type").String() != "command" || gjson.Get(got, "statusLine.command").String() != ours {
		t.Errorf("statusLine = %s", gjson.Get(got, "statusLine").Raw)
	}
	if gjson.Get(got, "theme").String() != "dark" || gjson.Get(got, "model").String() != "opus" || !gjson.Get(got, "permissions.allow.0").Exists() {
		t.Errorf("other keys changed:\n%s", got)
	}

	out, err = statusline.Disable(p)
	if err != nil || out != statusline.Restored {
		t.Fatalf("Disable = %v, %v", out, err)
	}
	if after := settingsOf(t, p); after != plain {
		t.Errorf("settings.json is not what it was:\n%s", after)
	}
	if rec, err := statusline.Load(p); err != nil || rec != nil {
		t.Errorf("the record should be gone: %v, %v", rec, err)
	}
}

func TestEnableKeepsTheUsersLineAndItsOptions(t *testing.T) {
	p := setup(t, withLine)

	if _, err := statusline.Enable(p, ours, now); err != nil {
		t.Fatal(err)
	}

	got := settingsOf(t, p)
	if gjson.Get(got, "statusLine.command").String() != ours || gjson.Get(got, "statusLine.padding").Int() != 2 {
		t.Errorf("statusLine = %s", gjson.Get(got, "statusLine").Raw)
	}
	rec, err := statusline.Load(p)
	if err != nil || rec == nil || gjson.GetBytes(rec.Original, "command").String() != "~/.claude/line.sh" {
		t.Fatalf("record = %+v, %v", rec, err)
	}

	if _, err := statusline.Disable(p); err != nil {
		t.Fatal(err)
	}
	if after := settingsOf(t, p); after != withLine {
		t.Errorf("settings.json is not what it was:\n%s", after)
	}
}

func TestEnableTwiceChangesNothingAndRefreshesAMovedBinary(t *testing.T) {
	p := setup(t, withLine)
	if _, err := statusline.Enable(p, ours, now); err != nil {
		t.Fatal(err)
	}
	first := settingsOf(t, p)

	if out, err := statusline.Enable(p, ours, now); err != nil || out != statusline.AlreadyEnabled || settingsOf(t, p) != first {
		t.Errorf("second Enable = %v, %v", out, err)
	}

	moved := "'/usr/local/bin/squad' statusline"
	if out, err := statusline.Enable(p, moved, now); err != nil || out != statusline.Refreshed {
		t.Fatalf("Enable with a new path = %v, %v", out, err)
	}
	if gjson.Get(settingsOf(t, p), "statusLine.command").String() != moved {
		t.Error("the command was not refreshed")
	}
	if _, err := statusline.Disable(p); err != nil {
		t.Fatal(err)
	}
	if after := settingsOf(t, p); after != withLine {
		t.Errorf("the original was lost after a refresh:\n%s", after)
	}
}

func TestDisableLeavesALineTheUserChangedAfterwards(t *testing.T) {
	p := setup(t, plain)
	if _, err := statusline.Enable(p, ours, now); err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(settingsOf(t, p), ours, "my-own-line", 1)
	if err := os.WriteFile(p.SettingsFile(), []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := statusline.Disable(p)

	if err != nil || out != statusline.NotOurs {
		t.Fatalf("Disable = %v, %v", out, err)
	}
	if settingsOf(t, p) != changed {
		t.Error("a line that is not ours was touched")
	}
	if rec, _ := statusline.Load(p); rec != nil {
		t.Error("the stale record should be dropped")
	}
}

func TestDisableWhenNeverEnabled(t *testing.T) {
	p := setup(t, plain)
	if _, err := statusline.Disable(p); !errors.Is(err, statusline.ErrNotEnabled) {
		t.Errorf("err = %v", err)
	}
	if settingsOf(t, p) != plain {
		t.Error("settings.json changed")
	}
}

func TestEnableAfterTheSettingsWereRestoredStartsFresh(t *testing.T) {
	p := setup(t, withLine)
	if _, err := statusline.Enable(p, ours, now); err != nil {
		t.Fatal(err)
	}
	// `squad restore` puts the original file back, our line gone.
	if err := os.WriteFile(p.SettingsFile(), []byte(withLine), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := statusline.Enable(p, ours, now)

	if err != nil || out != statusline.Enabled {
		t.Fatalf("Enable = %v, %v", out, err)
	}
	if _, err := statusline.Disable(p); err != nil {
		t.Fatal(err)
	}
	if settingsOf(t, p) != withLine {
		t.Error("the user's line was not kept")
	}
}

func TestEnableRefusesBrokenSettingsAndCreatesMissingOnes(t *testing.T) {
	p := setup(t, "{ not json")
	if _, err := statusline.Enable(p, ours, now); err == nil || !strings.Contains(err.Error(), "settings.json") {
		t.Errorf("err = %v", err)
	}
	if settingsOf(t, p) != "{ not json" {
		t.Error("broken settings were rewritten")
	}

	q := setup(t, "")
	if _, err := statusline.Enable(q, ours, now); err != nil {
		t.Fatal(err)
	}
	if gjson.Get(settingsOf(t, q), "statusLine.command").String() != ours {
		t.Errorf("settings = %s", settingsOf(t, q))
	}
	if _, err := statusline.Disable(q); err != nil {
		t.Fatal(err)
	}
	if gjson.Get(settingsOf(t, q), "statusLine").Exists() {
		t.Error("statusLine is still there")
	}
}

func equip(t *testing.T, p paths.Paths, name string) {
	t.Helper()
	if err := apply.SaveState(p, apply.State{Active: name}); err != nil {
		t.Fatal(err)
	}
}

func render(t *testing.T, p paths.Paths, in statusline.Input) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return statusline.Render(ctx, p, in)
}

func TestRenderShowsTheEquippedPreset(t *testing.T) {
	p := setup(t, plain)
	if got := render(t, p, statusline.Input{}); got != "" {
		t.Errorf("nothing equipped: %q", got)
	}
	equip(t, p, "solo")
	if got := render(t, p, statusline.Input{}); got != "◆ solo" {
		t.Errorf("got %q", got)
	}
	equip(t, p, "initial")
	if got := render(t, p, statusline.Input{}); got != "" {
		t.Errorf("initial: %q", got)
	}
}

func TestRenderPrefersThePresetOfThisSession(t *testing.T) {
	p := setup(t, plain)
	equip(t, p, "solo")

	got := render(t, p, statusline.Input{RunPreset: "bug-hunt"})

	if got != "◆ bug-hunt (run)" {
		t.Errorf("got %q", got)
	}
}

func TestRenderColorsTheSegmentOnRequest(t *testing.T) {
	p := setup(t, plain)
	equip(t, p, "solo")
	got := render(t, p, statusline.Input{Color: true})
	if !strings.Contains(got, "\x1b[") || !strings.Contains(got, "◆ solo") {
		t.Errorf("got %q", got)
	}
}

func enableWith(t *testing.T, line string) paths.Paths {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the user's line is a shell command")
	}
	p := setup(t, "{\n  \"statusLine\": {\"type\": \"command\", \"command\": "+quote(line)+"}\n}\n")
	if _, err := statusline.Enable(p, ours, now); err != nil {
		t.Fatal(err)
	}
	equip(t, p, "solo")
	return p
}

func quote(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

func TestRenderRunsTheUsersLineAndAppendsToItsLastLine(t *testing.T) {
	p := enableWith(t, `printf 'main line\nsecond line'`)

	got := render(t, p, statusline.Input{})

	if got != "main line\nsecond line ◆ solo" {
		t.Errorf("got %q", got)
	}
}

func TestRenderPassesStdinToTheUsersLine(t *testing.T) {
	p := enableWith(t, `cat`)

	got := render(t, p, statusline.Input{Stdin: []byte(`{"model":"opus"}`)})

	if got != `{"model":"opus"} ◆ solo` {
		t.Errorf("got %q", got)
	}
}

func TestRenderSurvivesAFailingOrSlowLine(t *testing.T) {
	failing := enableWith(t, `exit 3`)
	if got := render(t, failing, statusline.Input{}); got != "◆ solo" {
		t.Errorf("failing: %q", got)
	}

	slow := enableWith(t, `sleep 30; echo late`)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	start := time.Now()
	got := statusline.Render(ctx, slow, statusline.Input{})
	if got != "◆ solo" || time.Since(start) > 5*time.Second {
		t.Errorf("slow: %q after %v", got, time.Since(start))
	}
}

func TestRenderWithoutARecordJustShowsThePreset(t *testing.T) {
	p := setup(t, plain)
	equip(t, p, "solo")
	if got := render(t, p, statusline.Input{Stdin: []byte("{}")}); got != "◆ solo" {
		t.Errorf("got %q", got)
	}
}

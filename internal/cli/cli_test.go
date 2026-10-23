package cli_test

import (
	"strings"
	"testing"

	"github.com/EdrenaVosh/squadup/internal/testutil"
	"github.com/EdrenaVosh/squadup/internal/version"
)

func TestVersionFlagPrintsBuildInfo(t *testing.T) {
	e := newEnv(t)
	version.Version, version.Commit, version.Date = "0.1.0", "abc1234", "2026-10-24T00:00:00Z"

	r := e.run("--version")

	if r.code != 0 {
		t.Fatalf("exit code = %d, want 0", r.code)
	}
	if want := "squad 0.1.0 (commit abc1234, built 2026-10-24T00:00:00Z)\n"; r.stdout != want {
		t.Fatalf("stdout = %q, want %q", r.stdout, want)
	}
}

func TestDoctorReportsHealthyClaude(t *testing.T) {
	e := newEnv(t)
	bin := testutil.FakeClaude(t)
	t.Setenv("FAKE_CLAUDE_VERSION", "2.7.1 (Claude Code)")

	r := e.run("doctor")

	if r.code != 0 {
		t.Fatalf("exit code = %d, want 0\nstdout:\n%s\nstderr:\n%s", r.code, r.stdout, r.stderr)
	}
	for _, want := range []string{bin, "2.7.1 (Claude Code)", "logged in via claude.ai", e.claudeDir} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("stdout lacks %q:\n%s", want, r.stdout)
		}
	}
}

func TestDoctorSkipsConfigDirWhenClaudeDoesNotReportIt(t *testing.T) {
	e := newEnv(t)
	testutil.FakeClaude(t)
	t.Setenv("CLAUDE_CONFIG_DIR", "") // the stub echoes this variable; squad gets the flag instead

	r := e.run("doctor", "--claude-dir", e.claudeDir)

	if r.code != 0 {
		t.Fatalf("exit code = %d, want 0", r.code)
	}
	if strings.Contains(r.stdout, "config dir") {
		t.Errorf("stdout prints an empty config dir line:\n%s", r.stdout)
	}
}

func TestDoctorFailsWithFixWhenClaudeIsMissing(t *testing.T) {
	e := newEnv(t)
	testutil.EmptyPath(t)

	r := e.run("doctor")

	if r.code != 1 {
		t.Fatalf("exit code = %d, want 1", r.code)
	}
	if !strings.Contains(r.stdout, "not found on PATH") {
		t.Errorf("stdout lacks the failed check:\n%s", r.stdout)
	}
	if !strings.Contains(r.stderr, "install Claude Code") {
		t.Errorf("stderr lacks a fix hint:\n%s", r.stderr)
	}
	if strings.Contains(r.stderr, "goroutine") {
		t.Errorf("stderr leaks a stack trace:\n%s", r.stderr)
	}
}

func TestDoctorFailsWithFixWhenLoggedOut(t *testing.T) {
	e := newEnv(t)
	testutil.FakeClaude(t)
	t.Setenv("FAKE_CLAUDE_LOGGED_IN", "0")

	r := e.run("doctor")

	if r.code != 1 {
		t.Fatalf("exit code = %d, want 1", r.code)
	}
	if !strings.Contains(r.stdout, "not logged in") {
		t.Errorf("stdout lacks the failed check:\n%s", r.stdout)
	}
	if !strings.Contains(r.stderr, "claude auth login") {
		t.Errorf("stderr lacks a fix hint:\n%s", r.stderr)
	}
}

func TestUnknownCommandExitsWithErrorAndHelpHint(t *testing.T) {
	e := newEnv(t)

	r := e.run("bogus")

	if r.code != 1 {
		t.Fatalf("exit code = %d, want 1", r.code)
	}
	if !strings.Contains(r.stderr, `unknown command "bogus"`) {
		t.Errorf("stderr = %q", r.stderr)
	}
}

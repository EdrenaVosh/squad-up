package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/EdrenaVosh/squadup/internal/cli"
	"github.com/EdrenaVosh/squadup/internal/testutil"
	"github.com/EdrenaVosh/squadup/internal/version"
)

func run(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = cli.Execute(args, &out, &errOut)
	return out.String(), errOut.String(), code
}

func TestVersionFlagPrintsBuildInfo(t *testing.T) {
	version.Version, version.Commit, version.Date = "0.1.0", "abc1234", "2026-10-24T00:00:00Z"

	stdout, _, code := run(t, "--version")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if want := "squad 0.1.0 (commit abc1234, built 2026-10-24T00:00:00Z)\n"; stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestDoctorReportsHealthyClaude(t *testing.T) {
	bin := testutil.FakeClaude(t)
	t.Setenv("FAKE_CLAUDE_VERSION", "2.7.1 (Claude Code)")
	t.Setenv("CLAUDE_CONFIG_DIR", "/tmp/cfg")

	stdout, stderr, code := run(t, "doctor")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	for _, want := range []string{bin, "2.7.1 (Claude Code)", "logged in via claude.ai", "/tmp/cfg"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout lacks %q:\n%s", want, stdout)
		}
	}
}

func TestDoctorSkipsConfigDirWhenClaudeDoesNotReportIt(t *testing.T) {
	testutil.FakeClaude(t)
	t.Setenv("CLAUDE_CONFIG_DIR", "")

	stdout, _, code := run(t, "doctor")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.Contains(stdout, "config dir") {
		t.Errorf("stdout prints an empty config dir line:\n%s", stdout)
	}
}

func TestDoctorFailsWithFixWhenClaudeIsMissing(t *testing.T) {
	testutil.EmptyPath(t)

	stdout, stderr, code := run(t, "doctor")

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stdout, "not found on PATH") {
		t.Errorf("stdout lacks the failed check:\n%s", stdout)
	}
	if !strings.Contains(stderr, "install Claude Code") {
		t.Errorf("stderr lacks a fix hint:\n%s", stderr)
	}
	if strings.Contains(stderr, "goroutine") {
		t.Errorf("stderr leaks a stack trace:\n%s", stderr)
	}
}

func TestDoctorFailsWithFixWhenLoggedOut(t *testing.T) {
	testutil.FakeClaude(t)
	t.Setenv("FAKE_CLAUDE_LOGGED_IN", "0")

	stdout, stderr, code := run(t, "doctor")

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stdout, "not logged in") {
		t.Errorf("stdout lacks the failed check:\n%s", stdout)
	}
	if !strings.Contains(stderr, "claude auth login") {
		t.Errorf("stderr lacks a fix hint:\n%s", stderr)
	}
}

func TestUnknownCommandExitsWithErrorAndHelpHint(t *testing.T) {
	_, stderr, code := run(t, "bogus")

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, `unknown command "bogus"`) {
		t.Errorf("stderr = %q", stderr)
	}
}

package claude_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/EdrenaVosh/squadup/internal/claude"
	"github.com/EdrenaVosh/squadup/internal/testutil"
)

func TestFindReturnsErrNotFoundWhenClaudeIsNotOnPath(t *testing.T) {
	testutil.EmptyPath(t)

	_, err := claude.Find()

	if !errors.Is(err, claude.ErrNotFound) {
		t.Fatalf("Find() error = %v, want ErrNotFound", err)
	}
}

func TestFindLocatesClaudeOnPath(t *testing.T) {
	want := testutil.FakeClaude(t)

	got, err := claude.Find()

	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}
	if got != want {
		t.Fatalf("Find() = %q, want %q", got, want)
	}
}

func TestVersionTrimsOutput(t *testing.T) {
	bin := testutil.FakeClaude(t)
	t.Setenv("FAKE_CLAUDE_VERSION", "2.7.1 (Claude Code)")

	got, err := claude.Client{Bin: bin}.Version(context.Background())

	if err != nil {
		t.Fatalf("Version() error = %v", err)
	}
	if got != "2.7.1 (Claude Code)" {
		t.Fatalf("Version() = %q", got)
	}
}

func TestAuthReportsLoggedInAccount(t *testing.T) {
	bin := testutil.FakeClaude(t)
	t.Setenv("CLAUDE_CONFIG_DIR", "/tmp/cfg")

	got, err := claude.Client{Bin: bin}.Auth(context.Background())

	if err != nil {
		t.Fatalf("Auth() error = %v", err)
	}
	if !got.LoggedIn || got.AuthMethod != "claude.ai" || got.APIProvider != "firstParty" || got.ConfigDirectory != "/tmp/cfg" {
		t.Fatalf("Auth() = %+v", got)
	}
}

func TestAuthParsesLoggedOutStatusDespiteNonZeroExit(t *testing.T) {
	bin := testutil.FakeClaude(t)
	t.Setenv("FAKE_CLAUDE_LOGGED_IN", "0")

	got, err := claude.Client{Bin: bin}.Auth(context.Background())

	if err != nil {
		t.Fatalf("Auth() error = %v, want nil: logged out is a status, not a failure", err)
	}
	if got.LoggedIn || got.AuthMethod != "none" {
		t.Fatalf("Auth() = %+v", got)
	}
}

func TestProbeReturnsTheResolvedModelID(t *testing.T) {
	bin := testutil.FakeClaude(t)
	t.Setenv("FAKE_CLAUDE_ID_OPUS", "claude-opus-5-5-20261001")

	got, err := claude.Client{Bin: bin}.Probe(context.Background(), "opus")

	if err != nil {
		t.Fatalf("Probe() error = %v", err)
	}
	if got != "claude-opus-5-5-20261001" {
		t.Fatalf("Probe() = %q: the helper model in modelUsage must not win", got)
	}
}

func TestProbeSendsAMinimalNonPersistentRequest(t *testing.T) {
	bin := testutil.FakeClaude(t)
	argsFile := filepath.Join(t.TempDir(), "args")
	t.Setenv("FAKE_CLAUDE_ARGS_FILE", argsFile)

	if _, err := (claude.Client{Bin: bin}).Probe(context.Background(), "sonnet"); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(argsFile)
	args := strings.Split(strings.TrimSpace(string(data)), "\x1f")
	want := []string{"-p", "Reply with OK", "--model", "sonnet", "--output-format", "json", "--no-session-persistence"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %q, want %q", args, want)
	}
}

func TestProbeExplainsAnUnavailableModel(t *testing.T) {
	bin := testutil.FakeClaude(t)
	t.Setenv("FAKE_CLAUDE_UNAVAILABLE", "opus")

	_, err := claude.Client{Bin: bin}.Probe(context.Background(), "opus")

	if err == nil || !strings.Contains(err.Error(), "issue with the selected model") {
		t.Fatalf("Probe() error = %v, want the reason claude gave", err)
	}
}

func TestProbeGivesUpWhenTheContextExpires(t *testing.T) {
	bin := testutil.FakeClaude(t)
	t.Setenv("FAKE_CLAUDE_SLEEP_MS", "3000")
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := claude.Client{Bin: bin}.Probe(ctx, "haiku")

	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("Probe() error = %v, want a timeout", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Errorf("Probe() took %v: the process was not stopped", time.Since(start))
	}
}

package claude_test

import (
	"context"
	"errors"
	"testing"

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

package version_test

import (
	"testing"

	"github.com/EdrenaVosh/squad-up/internal/version"
)

func TestStringShowsVersionCommitAndDate(t *testing.T) {
	version.Version, version.Commit, version.Date = "0.1.0", "abc1234", "2026-10-24T00:00:00Z"

	got := version.String()

	want := "squad 0.1.0 (commit abc1234, built 2026-10-24T00:00:00Z)"
	if got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

func TestDefaultsIdentifyDevelopmentBuild(t *testing.T) {
	version.Version, version.Commit, version.Date = "dev", "none", "unknown"

	got := version.String()

	want := "squad dev (commit none, built unknown)"
	if got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

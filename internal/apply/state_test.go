package apply_test

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/EdrenaVosh/squadup/internal/apply"
	"github.com/EdrenaVosh/squadup/internal/fsx"
	"github.com/EdrenaVosh/squadup/internal/paths"
	"github.com/EdrenaVosh/squadup/internal/testutil"
)

func newPaths(t *testing.T) paths.Paths {
	t.Helper()
	return paths.Paths{ClaudeDir: filepath.Join(t.TempDir(), "claude"), Home: filepath.Join(t.TempDir(), "squadup")}
}

func TestLoadStateWithoutFileMeansNothingApplied(t *testing.T) {
	s, err := apply.LoadState(newPaths(t))

	if err != nil {
		t.Fatal(err)
	}
	if s.Active != "" || len(s.Manifest) != 0 || s.ActivePreset() != "initial" {
		t.Fatalf("state = %+v, ActivePreset = %q", s, s.ActivePreset())
	}
}

func TestSaveStateThenLoadRoundTrips(t *testing.T) {
	p := newPaths(t)
	want := apply.State{
		Schema:    1,
		Active:    "solo-indie",
		AppliedAt: time.Date(2026, 10, 24, 9, 30, 0, 0, time.UTC),
		BackupID:  "2026-10-24T09-30-00",
		Manifest:  map[string]string{"agents/planner.md": "abc123"},
	}

	if err := apply.SaveState(p, want); err != nil {
		t.Fatal(err)
	}
	got, err := apply.LoadState(p)

	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadState() = %+v, %v; want %+v", got, err, want)
	}
	if got.ActivePreset() != "solo-indie" {
		t.Errorf("ActivePreset() = %q", got.ActivePreset())
	}
}

func TestLoadStateOfCorruptFileNamesFileAndFix(t *testing.T) {
	p := newPaths(t)
	testutil.WriteFiles(t, p.Home, map[string]string{"state.json": "{not json"})

	_, err := apply.LoadState(p)

	if err == nil || !strings.Contains(err.Error(), p.StateFile()) {
		t.Fatalf("LoadState() error = %v, want it to name %s", err, p.StateFile())
	}
}

func TestDriftReportsModifiedAndMissingFilesOnly(t *testing.T) {
	p := newPaths(t)
	testutil.WriteFiles(t, p.ClaudeDir, map[string]string{
		"agents/same.md":    "same",
		"agents/changed.md": "edited by hand",
		"agents/foreign.md": "not ours",
	})
	state := apply.State{Active: "x", Manifest: map[string]string{
		"agents/same.md":    fsx.SHA256Hex([]byte("same")),
		"agents/changed.md": fsx.SHA256Hex([]byte("as written")),
		"agents/gone.md":    fsx.SHA256Hex([]byte("was here")),
	}}

	got, err := apply.Drift(p, state)

	if err != nil {
		t.Fatal(err)
	}
	want := []apply.DriftItem{
		{Path: "agents/changed.md", Kind: apply.DriftModified},
		{Path: "agents/gone.md", Kind: apply.DriftMissing},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Drift() = %+v, want %+v", got, want)
	}
}

func TestDriftWithEmptyManifestIsClean(t *testing.T) {
	got, err := apply.Drift(newPaths(t), apply.State{})

	if err != nil || len(got) != 0 {
		t.Fatalf("Drift() = %+v, %v", got, err)
	}
}

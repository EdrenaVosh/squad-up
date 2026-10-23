package preset_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/EdrenaVosh/squadup/internal/preset"
)

func bundle() preset.Bundle {
	p := sample()
	return preset.Bundle{
		Preset: p,
		Agents: []preset.AgentFile{
			{Name: "planner", Data: []byte("---\r\nname: planner\r\ndescription: d\r\n---\r\nPlan it.\r\n")},
			{Name: "reviewer", Data: []byte("---\nname: reviewer\ndescription: d\n---\nReview it.")},
			{Name: "tester", Data: []byte("---\nname: tester\ndescription: d\n---\n")},
		},
	}
}

func TestSaveThenLoadRoundTripsPresetAndAgentBytes(t *testing.T) {
	store := preset.Store{Dir: t.TempDir()}
	want := bundle()

	if err := store.Save(want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	got, err := store.Load("solo-indie")

	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !reflect.DeepEqual(got.Preset, want.Preset) {
		t.Errorf("preset = %+v, want %+v", got.Preset, want.Preset)
	}
	if !reflect.DeepEqual(got.Agents, want.Agents) {
		t.Errorf("agents changed:\n got: %q\nwant: %q", got.Agents, want.Agents)
	}
}

func TestSaveWritesDocumentedLayout(t *testing.T) {
	dir := t.TempDir()
	store := preset.Store{Dir: dir}

	if err := store.Save(bundle()); err != nil {
		t.Fatal(err)
	}

	for _, rel := range []string{"solo-indie/preset.yaml", "solo-indie/agents/planner.md", "solo-indie/agents/reviewer.md", "solo-indie/agents/tester.md"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
			t.Errorf("missing %s: %v", rel, err)
		}
	}
}

func TestSaveRefusesToOverwrite(t *testing.T) {
	store := preset.Store{Dir: t.TempDir()}
	if err := store.Save(bundle()); err != nil {
		t.Fatal(err)
	}

	err := store.Save(bundle())

	if !errors.Is(err, preset.ErrExists) {
		t.Fatalf("Save() error = %v, want ErrExists", err)
	}
}

func TestSaveRejectsInvalidPresetAndWritesNothing(t *testing.T) {
	dir := t.TempDir()
	b := bundle()
	b.Preset.Name = "../escape"

	err := preset.Store{Dir: dir}.Save(b)

	if err == nil {
		t.Fatal("Save() accepted an invalid name")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("Save() left files behind: %v", entries)
	}
}

func TestSaveRejectsAgentListMismatch(t *testing.T) {
	b := bundle()
	b.Agents = b.Agents[:2] // preset.yaml still lists "tester"

	err := preset.Store{Dir: t.TempDir()}.Save(b)

	if err == nil || !strings.Contains(err.Error(), "tester") {
		t.Fatalf("Save() error = %v, want a mismatch error naming tester", err)
	}
}

func TestLoadMissingPreset(t *testing.T) {
	_, err := preset.Store{Dir: t.TempDir()}.Load("nope")

	if !errors.Is(err, preset.ErrNotFound) {
		t.Fatalf("Load() error = %v, want ErrNotFound", err)
	}
}

func TestLoadNamesTheMissingAgentFile(t *testing.T) {
	dir := t.TempDir()
	store := preset.Store{Dir: dir}
	if err := store.Save(bundle()); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "solo-indie", "agents", "tester.md")); err != nil {
		t.Fatal(err)
	}

	_, err := store.Load("solo-indie")

	if err == nil || !strings.Contains(err.Error(), "tester.md") {
		t.Fatalf("Load() error = %v, want it to name tester.md", err)
	}
}

func TestLoadRejectsTraversalNames(t *testing.T) {
	_, err := preset.Store{Dir: t.TempDir()}.Load("../etc")

	if err == nil || errors.Is(err, preset.ErrNotFound) {
		t.Fatalf("Load() error = %v, want a name validation error", err)
	}
}

func TestNamesAreSortedAndSkipHiddenAndFiles(t *testing.T) {
	dir := t.TempDir()
	store := preset.Store{Dir: dir}
	for _, name := range []string{"zeta", "alpha"} {
		b := bundle()
		b.Preset.Name = name
		if err := store.Save(b); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, ".initial.tmp"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "stray.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := store.Names()

	if err != nil || !reflect.DeepEqual(got, []string{"alpha", "zeta"}) {
		t.Fatalf("Names() = %v, %v", got, err)
	}
}

func TestNamesOfMissingDirIsEmpty(t *testing.T) {
	got, err := preset.Store{Dir: filepath.Join(t.TempDir(), "absent")}.Names()

	if err != nil || len(got) != 0 {
		t.Fatalf("Names() = %v, %v", got, err)
	}
}

package workspace_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/EdrenaVosh/squad-up/internal/preset"
	"github.com/EdrenaVosh/squad-up/internal/workspace"
)

func TestSlugMakesAFileSafeIDFromAnyTitle(t *testing.T) {
	cases := map[string]string{
		"Cube creator":           "cube-creator",
		"  My  Team!!  ":         "my-team",
		"Bug Hunter #1 (v2)":     "bug-hunter-1-v2",
		"Куб Креатор":            "kub-kreator",
		"Щука & Ёж":              "schuka-ezh",
		"★ ☠ 🚀":                  "preset",
		"":                       "preset",
		"---":                    "preset",
		"already-a-slug":         "already-a-slug",
		"CamelCaseTitle":         "camelcasetitle",
		"a/b\\c:d":               "a-b-c-d",
		strings.Repeat("x", 100): strings.Repeat("x", 64),
	}
	for title, want := range cases {
		got := workspace.Slug(title)
		if got != want {
			t.Errorf("Slug(%q) = %q, want %q", title, got, want)
		}
		if !preset.ValidName(got) {
			t.Errorf("Slug(%q) = %q is not a valid id", title, got)
		}
	}
}

func TestATitleMayHaveCapitalsSpacesAndSymbols(t *testing.T) {
	for _, ok := range []string{"Cube creator", "Bug: \"Hunter\" #1 — ☠️ [v2] 'x'", "Куб", "日本語", "a"} {
		if err := workspace.CheckTitle(ok); err != nil {
			t.Errorf("%q refused: %v", ok, err)
		}
	}
	for bad, why := range map[string]string{"": "enter a name", "   ": "enter a name", "two\nlines": "control", strings.Repeat("я", 81): "at most 80"} {
		if err := workspace.CheckTitle(bad); err == nil || !strings.Contains(err.Error(), why) {
			t.Errorf("%q: err = %v, want %q", bad, err, why)
		}
	}
}

func TestFreeIDAddsANumberWhenTheIDIsTaken(t *testing.T) {
	w := setup(t) // has the preset "solo"

	if got := w.FreeID("Solo"); got != "solo-2" {
		t.Errorf("FreeID(Solo) = %q", got)
	}
	if got := w.FreeID("Initial"); got != "initial-2" {
		t.Errorf("FreeID(Initial) = %q: initial is reserved", got)
	}
	if got := w.FreeID("Something new"); got != "something-new" {
		t.Errorf("FreeID = %q", got)
	}
	long := strings.Repeat("x", 80)
	if _, err := w.New(workspace.Slug(long)); err != nil {
		t.Fatal(err)
	}
	if got := w.FreeID(long); len(got) > 64 || !strings.HasSuffix(got, "-2") || !preset.ValidName(got) {
		t.Errorf("FreeID(long) = %q", got)
	}
}

func TestIDForKeepsAnIDAndMakesOneFromATitle(t *testing.T) {
	w := setup(t)

	if id, title := w.IDFor("my-id"); id != "my-id" || title != "my-id" {
		t.Errorf("a valid id: %q, %q", id, title)
	}
	if id, title := w.IDFor("  Cube creator  "); id != "cube-creator" || title != "Cube creator" {
		t.Errorf("a title: %q, %q", id, title)
	}
}

func TestATitleWithSymbolsSurvivesTheFile(t *testing.T) {
	w := setup(t)
	title := "Bug: \"Hunter\" #1 — ☠️ [v2] 'x' | {y}"
	id, _ := w.IDFor(title)
	if _, err := w.New(id); err != nil {
		t.Fatal(err)
	}

	if err := w.SetTitle(id, title); err != nil {
		t.Fatal(err)
	}

	b, err := w.Store().Load(id)
	if err != nil || b.Preset.Title != title {
		t.Fatalf("title = %q, %v", b.Preset.Title, err)
	}
	card, err := w.Card(id)
	if err != nil || card.Title != title {
		t.Errorf("card title = %q, %v", card.Title, err)
	}
	if err := w.SetTitle("initial", "x"); err == nil {
		t.Error("Initial's title was changed")
	}
	if err := w.SetTitle(id, ""); err == nil {
		t.Error("an empty title was accepted")
	}
}

func TestRenameToChangesTheTitleAndMovesTheIDOnlyWhenTheWordsChange(t *testing.T) {
	w := setup(t)
	if _, err := w.New("cube-creator"); err != nil {
		t.Fatal(err)
	}

	// the same words in a nicer form: the id stays, the title changes
	id, err := w.RenameTo("cube-creator", "Cube Creator!")
	if err != nil || id != "cube-creator" {
		t.Fatalf("same words: %q, %v", id, err)
	}
	if b, _ := w.Store().Load("cube-creator"); b.Preset.Title != "Cube Creator!" {
		t.Errorf("title = %q", b.Preset.Title)
	}

	// different words: a new id and the typed title
	id, err = w.RenameTo("cube-creator", "Cube Maker ★")
	if err != nil || id != "cube-maker" || w.Store().Exists("cube-creator") {
		t.Fatalf("new words: %q, %v", id, err)
	}
	if b, _ := w.Store().Load("cube-maker"); b.Preset.Title != "Cube Maker ★" || b.Preset.Name != "cube-maker" {
		t.Errorf("preset = %+v", b.Preset)
	}

	// a valid id is used as it is
	if id, err = w.RenameTo("cube-maker", "plain-id"); err != nil || id != "plain-id" {
		t.Errorf("valid id: %q, %v", id, err)
	}
	if _, err = w.RenameTo("plain-id", "initial"); !errors.Is(err, workspace.ErrReservedName) {
		t.Errorf("reserved: %v", err)
	}
	if _, err = w.RenameTo("initial", "x"); !errors.Is(err, workspace.ErrProtected) {
		t.Errorf("Initial: %v", err)
	}
}

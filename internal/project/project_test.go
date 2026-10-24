package project_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/EdrenaVosh/squadup/internal/project"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFindWalksUpToTheNearestFile(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, ".squadup"), "outer\n")
	write(t, filepath.Join(root, "app", ".squadup"), "# the app team\nsolo-indie\n")
	deep := filepath.Join(root, "app", "src", "pkg")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}

	pin, err := project.Find(deep)
	if err != nil || pin == nil {
		t.Fatalf("Find = %v, %v", pin, err)
	}
	if pin.Preset != "solo-indie" || pin.File != filepath.Join(root, "app", ".squadup") {
		t.Errorf("pin = %+v", pin)
	}

	other := filepath.Join(root, "docs")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	if pin, _ := project.Find(other); pin == nil || pin.Preset != "outer" {
		t.Errorf("docs should find the outer file, got %+v", pin)
	}
}

func TestFindReturnsNilWhenNothingIsPinned(t *testing.T) {
	pin, err := project.Find(t.TempDir())
	if err != nil || pin != nil {
		t.Errorf("Find = %v, %v", pin, err)
	}
}

func TestFindReportsABrokenFileWithItsPath(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, ".squadup"), "Not A Name\n")

	_, err := project.Find(root)

	if err == nil || !strings.Contains(err.Error(), filepath.Join(root, ".squadup")) {
		t.Fatalf("err = %v", err)
	}
}

func TestParseTakesTheFirstLineThatIsNotAComment(t *testing.T) {
	for in, want := range map[string]string{
		"solo\n":                  "solo",
		"  solo  \r\n":            "solo",
		"# note\n\nsolo\nignored": "solo",
		"solo":                    "solo",
	} {
		got, err := project.Parse([]byte(in))
		if err != nil || got != want {
			t.Errorf("Parse(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "# only a comment\n", "../x\n", "initial\n", "two words\n"} {
		if got, err := project.Parse([]byte(in)); err == nil {
			t.Errorf("Parse(%q) = %q, want an error", in, got)
		}
	}
}

func TestWriteAndRemove(t *testing.T) {
	dir := t.TempDir()

	if err := project.Write(dir, "bug-hunt"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".squadup"))
	if string(data) != "bug-hunt\n" {
		t.Errorf("file = %q", data)
	}
	if err := project.Write(dir, "Bad Name"); err == nil {
		t.Error("an invalid name was written")
	}

	removed, err := project.Remove(dir)
	if err != nil || !removed {
		t.Fatalf("Remove = %v, %v", removed, err)
	}
	if removed, err := project.Remove(dir); err != nil || removed {
		t.Errorf("second Remove = %v, %v", removed, err)
	}
}

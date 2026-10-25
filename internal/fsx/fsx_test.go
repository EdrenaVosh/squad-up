package fsx_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/EdrenaVosh/squad-up/internal/fsx"
)

func TestWriteFileAtomicCreatesParentsAndWritesContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a", "b", "file.txt")

	if err := fsx.WriteFileAtomic(path, []byte("hello"), 0o600); err != nil {
		t.Fatalf("WriteFileAtomic() error = %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil || string(got) != "hello" {
		t.Fatalf("ReadFile() = %q, %v", got, err)
	}
}

func TestWriteFileAtomicReplacesExistingFileAndLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := fsx.WriteFileAtomic(path, []byte("new"), 0o600); err != nil {
		t.Fatalf("WriteFileAtomic() error = %v", err)
	}

	got, _ := os.ReadFile(path)
	if string(got) != "new" {
		t.Fatalf("content = %q, want new", got)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("directory has %d entries, want only the target file: %v", len(entries), entries)
	}
}

func TestWriteFileAtomicAppliesPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix permission bits")
	}
	path := filepath.Join(t.TempDir(), "secret.json")

	if err := fsx.WriteFileAtomic(path, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestSHA256HexOfKnownInput(t *testing.T) {
	got := fsx.SHA256Hex([]byte("abc"))

	want := "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if got != want {
		t.Fatalf("SHA256Hex() = %s, want %s", got, want)
	}
}

func TestSHA256FileMatchesSHA256Hex(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(path, []byte("abc"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := fsx.SHA256File(path)

	if err != nil || got != fsx.SHA256Hex([]byte("abc")) {
		t.Fatalf("SHA256File() = %s, %v", got, err)
	}
}

func TestReadFileIfExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	data, ok, err := fsx.ReadFileIfExists(path)
	if err != nil || !ok || string(data) != "x" {
		t.Fatalf("existing: %q, %v, %v", data, ok, err)
	}

	data, ok, err = fsx.ReadFileIfExists(filepath.Join(dir, "missing"))
	if err != nil || ok || data != nil {
		t.Fatalf("missing: %q, %v, %v", data, ok, err)
	}
}

// The safety fuse: a test binary must never write into the developer's real
// ~/.claude or ~/.squad-up.
func TestWritesIntoRealConfigPanicInTests(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	for name, op := range map[string]func(path string) error{
		"WriteFileAtomic": func(p string) error { return fsx.WriteFileAtomic(p, []byte("x"), 0o600) },
		"MkdirAll":        func(p string) error { return fsx.MkdirAll(p, 0o700) },
		"Remove":          func(p string) error { return fsx.Remove(p) },
		"RemoveAll":       func(p string) error { return fsx.RemoveAll(p) },
		"Rename":          func(p string) error { return fsx.Rename(p, p+".moved") },
	} {
		for _, root := range []string{".claude", ".squad-up"} {
			t.Run(name+"/"+root, func(t *testing.T) {
				defer func() {
					if recover() == nil {
						t.Fatalf("%s into ~/%s did not panic", name, root)
					}
				}()
				_ = op(filepath.Join(home, root, "agents", "x.md"))
			})
		}
	}
}

func TestWritingThroughASymlinkUpdatesTheTargetAndKeepsTheLink(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "dotfiles", "settings.json")
	if err := os.MkdirAll(filepath.Dir(real), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(real, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, link); err != nil {
		t.Skip("symlinks are not available:", err)
	}

	if err := fsx.WriteFileAtomic(link, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}

	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("the link was replaced by a regular file: %v, %v", info, err)
	}
	if got, _ := os.ReadFile(real); string(got) != "new" {
		t.Errorf("the file behind the link = %q, want it updated", got)
	}
}

package agentfile_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/EdrenaVosh/squadup/internal/agentfile"
	"github.com/EdrenaVosh/squadup/internal/testutil"
)

func TestScanDirFindsMarkdownRecursivelyInPathOrder(t *testing.T) {
	root := filepath.Join(t.TempDir(), "agents")
	testutil.WriteFiles(t, root, map[string]string{
		"b.md":           "b",
		"a.md":           "a",
		"sub/deep/c.md":  "c",
		"sub/d.md":       "d",
		"notes.txt":      "skip",
		".DS_Store":      "skip",
		"sub/README.txt": "skip",
	})

	files, existed, err := agentfile.ScanDir(root)

	if err != nil || !existed {
		t.Fatalf("ScanDir() = %v, %v", existed, err)
	}
	var got []string
	for _, f := range files {
		got = append(got, f.Path+"="+string(f.Data))
	}
	want := []string{"a.md=a", "b.md=b", "sub/d.md=d", "sub/deep/c.md=c"}
	if len(got) != len(want) {
		t.Fatalf("ScanDir() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ScanDir() = %v, want %v", got, want)
		}
	}
}

func TestScanDirOfMissingOrNonDirectory(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "agents")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	for name, path := range map[string]string{"missing": filepath.Join(dir, "nope"), "a file": file} {
		files, existed, err := agentfile.ScanDir(path)
		if err != nil || existed || len(files) != 0 {
			t.Errorf("%s: ScanDir() = %v, %v, %v", name, files, existed, err)
		}
	}
}

func TestScanDirEmptyExistingDir(t *testing.T) {
	root := t.TempDir()

	files, existed, err := agentfile.ScanDir(root)

	if err != nil || !existed || len(files) != 0 {
		t.Fatalf("ScanDir() = %v, %v, %v", files, existed, err)
	}
}

func TestScanDirFollowsFileSymlinksAndSkipsDirectoryAndDanglingOnes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	outside := t.TempDir()
	testutil.WriteFiles(t, outside, map[string]string{"real.md": "linked content", "dir/inner.md": "inner"})
	root := t.TempDir()
	for name, target := range map[string]string{
		"link.md":     filepath.Join(outside, "real.md"),
		"dirlink.md":  filepath.Join(outside, "dir"),
		"dangling.md": filepath.Join(outside, "gone.md"),
	} {
		if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}

	files, _, err := agentfile.ScanDir(root)

	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Path != "link.md" || string(files[0].Data) != "linked content" {
		t.Fatalf("ScanDir() = %+v, want only link.md", files)
	}
}

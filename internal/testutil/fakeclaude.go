// Package testutil holds helpers shared by tests only.
package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

var (
	buildOnce sync.Once
	builtBin  string
	buildErr  error
)

// FakeClaude puts a stub `claude` binary first on PATH for the duration of the
// test and returns the path to it. The stub is compiled once per test process.
func FakeClaude(t *testing.T) string {
	t.Helper()

	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "fakeclaude-")
		if err != nil {
			buildErr = err
			return
		}
		name := "claude"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		builtBin = filepath.Join(dir, name)
		cmd := exec.Command("go", "build", "-o", builtBin, "./testdata/fakeclaude")
		cmd.Dir = moduleRoot()
		if out, err := cmd.CombinedOutput(); err != nil {
			buildErr = &buildError{err: err, output: string(out)}
		}
	})
	if buildErr != nil {
		t.Fatalf("build fakeclaude: %v", buildErr)
	}

	t.Setenv("PATH", filepath.Dir(builtBin)+string(os.PathListSeparator)+os.Getenv("PATH"))
	return builtBin
}

// HideClaude removes every PATH entry that holds a claude executable for the
// duration of the test, so that a test never starts the developer's real
// Claude Code. Other tools stay available.
func HideClaude(t *testing.T) {
	t.Helper()
	var kept []string
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		found := false
		for _, name := range []string{"claude", "claude.exe"} {
			if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
				found = true
			}
		}
		if !found {
			kept = append(kept, dir)
		}
	}
	t.Setenv("PATH", strings.Join(kept, string(os.PathListSeparator)))
}

// EmptyPath makes `claude` unfindable for the duration of the test.
func EmptyPath(t *testing.T) {
	t.Helper()
	t.Setenv("PATH", t.TempDir())
}

type buildError struct {
	err    error
	output string
}

func (e *buildError) Error() string { return e.err.Error() + ": " + e.output }

func moduleRoot() string {
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "."
		}
		dir = parent
	}
}

// Package testutil holds helpers shared by tests only.
package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

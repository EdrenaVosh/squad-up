package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/EdrenaVosh/squadup/internal/cli"
)

func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){
		"squad": func() { os.Exit(cli.Execute(os.Args[1:], os.Stdout, os.Stderr)) },
	})
}

// TestScripts runs the end-to-end scenarios in testdata/script. Every script
// gets its own Claude config dir and squadup home inside its work dir.
func TestScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: filepath.Join("..", "..", "testdata", "script"),
		Setup: func(env *testscript.Env) error {
			home := filepath.Join(env.WorkDir, "home")
			env.Setenv("HOME", home)
			env.Setenv("USERPROFILE", home)
			env.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(env.WorkDir, "claude"))
			env.Setenv("SQUADUP_HOME", filepath.Join(env.WorkDir, "squadup"))
			env.Setenv("ANTHROPIC_MODEL", "")
			env.Setenv("PATH", fakeClaudeDir()+string(os.PathListSeparator)+withoutClaude(env.Getenv("PATH")))
			return nil
		},
		Cmds: map[string]func(ts *testscript.TestScript, neg bool, args []string){
			"copytree": cmdCopyTree,
			"treeeq":   cmdTreeEqual,
		},
	})
}

// copytree SRC DST copies a directory tree.
func cmdCopyTree(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) != 2 {
		ts.Fatalf("usage: copytree src dst")
	}
	src, dst := ts.MkAbs(args[0]), ts.MkAbs(args[1])
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	ts.Check(err)
}

// treeeq A B succeeds when both directories hold the same files with the
// same bytes; with ! it succeeds when they differ.
func cmdTreeEqual(ts *testscript.TestScript, neg bool, args []string) {
	if len(args) != 2 {
		ts.Fatalf("usage: [!] treeeq a b")
	}
	diff := compareTrees(ts, ts.MkAbs(args[0]), ts.MkAbs(args[1]))
	switch {
	case neg && len(diff) == 0:
		ts.Fatalf("%s and %s are identical", args[0], args[1])
	case !neg && len(diff) > 0:
		ts.Fatalf("%s and %s differ:\n  %s", args[0], args[1], strings.Join(diff, "\n  "))
	}
}

func readTree(ts *testscript.TestScript, root string) map[string][]byte {
	files := map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		files[filepath.ToSlash(rel)] = data
		return nil
	})
	ts.Check(err)
	return files
}

func compareTrees(ts *testscript.TestScript, a, b string) []string {
	left, right := readTree(ts, a), readTree(ts, b)
	var diff []string
	for path, data := range left {
		other, ok := right[path]
		switch {
		case !ok:
			diff = append(diff, "only in first: "+path)
		case !bytes.Equal(data, other):
			diff = append(diff, fmt.Sprintf("content differs: %s", path))
		}
	}
	for path := range right {
		if _, ok := left[path]; !ok {
			diff = append(diff, "only in second: "+path)
		}
	}
	sort.Strings(diff)
	return diff
}

// withoutClaude drops every PATH entry that holds a claude executable, so
// scripts never start the developer's real Claude Code.
func withoutClaude(path string) string {
	var kept []string
	for _, dir := range filepath.SplitList(path) {
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
	return strings.Join(kept, string(os.PathListSeparator))
}

var (
	fakeOnce sync.Once
	fakeDir  string
)

// fakeClaudeDir builds testdata/fakeclaude once and returns the folder with
// the resulting `claude`, which scripts find first on PATH.
func fakeClaudeDir() string {
	fakeOnce.Do(func() {
		dir, err := os.MkdirTemp("", "squad-fakeclaude-")
		if err != nil {
			panic(err)
		}
		name := "claude"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		cmd := exec.Command("go", "build", "-o", filepath.Join(dir, name), "./testdata/fakeclaude")
		cmd.Dir = filepath.Join("..", "..")
		if out, err := cmd.CombinedOutput(); err != nil {
			panic(fmt.Sprintf("build fakeclaude: %v\n%s", err, out))
		}
		fakeDir = dir
	})
	return fakeDir
}

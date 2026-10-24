package testutil

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// FakeGH puts a stub `gh` alone on PATH for the duration of the test, so that
// the real GitHub CLI is never reached. The stub records its arguments in the
// first file returned and what it read on stdin in the second, lists the gist
// files named by FAKE_GH_FILES, serves FAKE_GH_CONTENT as a gist's content,
// and answers `gist create` with a fixed address.
func FakeGH(t *testing.T) (argsLog, stdinFile string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the gh stub is a shell script")
	}
	dir := t.TempDir()
	argsLog = filepath.Join(dir, "args.log")
	stdinFile = filepath.Join(dir, "stdin")
	script := `#!/bin/sh
echo "$*" >> "$FAKE_GH_LOG"
case "$1 $2" in
"gist view")
  case "$*" in
  *--files*) printf '%s\n' "$FAKE_GH_FILES" ;;
  *) /bin/cat "$FAKE_GH_CONTENT" ;;
  esac ;;
"gist create")
  /bin/cat > "$FAKE_GH_STDIN"
  echo "https://gist.github.com/me/abc123" ;;
*) echo "unexpected: $*" >&2; exit 2 ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	content := filepath.Join(dir, "content")
	if err := os.WriteFile(content, []byte("gist body"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("FAKE_GH_LOG", argsLog)
	t.Setenv("FAKE_GH_STDIN", stdinFile)
	t.Setenv("FAKE_GH_FILES", "notes.txt\nsolo.squad.json")
	t.Setenv("FAKE_GH_CONTENT", content)
	return argsLog, stdinFile
}

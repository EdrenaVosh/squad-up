// Package editor starts the user's text editor on a file.
package editor

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// Command returns the command that opens path in the editor named by $VISUAL
// or $EDITOR (which may carry arguments, as in "code -w"), or in vi, or in
// notepad on Windows. The caller connects it to the terminal and runs it.
func Command(path string) *exec.Cmd {
	spec := os.Getenv("VISUAL")
	if spec == "" {
		spec = os.Getenv("EDITOR")
	}
	if spec == "" {
		spec = "vi"
		if runtime.GOOS == "windows" {
			spec = "notepad"
		}
	}
	fields := strings.Fields(spec)
	return exec.Command(fields[0], append(fields[1:], path)...)
}

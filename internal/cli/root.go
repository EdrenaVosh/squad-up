// Package cli wires the squad command line.
package cli

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/EdrenaVosh/squadup/internal/version"
)

// ExitError ends the process with Code. Err says what happened and Fix says
// how to repair it; a nil Err means the details were already printed.
type ExitError struct {
	Code int
	Err  error
	Fix  string
}

func (e *ExitError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("exit status %d", e.Code)
	}
	return e.Err.Error()
}

func (e *ExitError) Unwrap() error { return e.Err }

// Execute runs squad with args and returns the process exit code.
func Execute(args []string, stdout, stderr io.Writer) int {
	root := newRoot(stdout, stderr)
	root.SetArgs(args)

	err := root.Execute()
	if err == nil {
		return 0
	}

	var exitErr *ExitError
	if errors.As(err, &exitErr) {
		if exitErr.Err != nil {
			printf(stderr, "Error: %v\n", exitErr.Err)
		}
		if exitErr.Fix != "" {
			printf(stderr, "Fix: %s\n", exitErr.Fix)
		}
		return exitErr.Code
	}

	printf(stderr, "Error: %v\nRun 'squad --help' for usage.\n", err)
	return 1
}

// printf writes terminal output; a failed write to stdout/stderr has no
// useful recovery, so the error is dropped on purpose.
func printf(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

func newRoot(stdout, stderr io.Writer) *cobra.Command {
	root := &cobra.Command{
		Use:           "squad",
		Short:         "Switch Claude Code agent teams with one command",
		Version:       version.String(),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetVersionTemplate("{{.Version}}\n")
	root.AddCommand(newDoctor())
	return root
}

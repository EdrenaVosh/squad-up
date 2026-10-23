// Package cli wires the squad command line.
package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/EdrenaVosh/squadup/internal/initial"
	"github.com/EdrenaVosh/squadup/internal/paths"
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

// IO is everything squad touches in the outside world, so tests can drive it.
type IO struct {
	In          io.Reader
	Out, Err    io.Writer
	Interactive bool             // whether In is a person at a terminal
	Now         func() time.Time // defaults to time.Now
}

// Execute runs squad on the real terminal and returns the exit code.
func Execute(args []string, stdout, stderr io.Writer) int {
	return Run(args, IO{In: os.Stdin, Out: stdout, Err: stderr, Interactive: isTerminal(os.Stdin)})
}

// isTerminal reports whether f is a character device other than the null
// device. It avoids a terminal dependency until the TUI brings one.
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	null, err := os.Stat(os.DevNull)
	return err != nil || !os.SameFile(info, null)
}

// Run executes squad with args and returns the process exit code.
func Run(args []string, streams IO) int {
	a := &app{io: streams}
	root := a.newRoot()
	root.SetArgs(args)

	err := root.Execute()
	if err == nil {
		return 0
	}

	var exitErr *ExitError
	if errors.As(err, &exitErr) {
		if exitErr.Err != nil {
			printf(streams.Err, "Error: %v\n", exitErr.Err)
		}
		if exitErr.Fix != "" {
			printf(streams.Err, "Fix: %s\n", exitErr.Fix)
		}
		return exitErr.Code
	}

	printf(streams.Err, "Error: %v\nRun 'squad --help' for usage.\n", err)
	return 1
}

// printf writes terminal output; a failed write to stdout/stderr has no
// useful recovery, so the error is dropped on purpose.
func printf(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

type app struct {
	io        IO
	claudeDir string // --claude-dir
	home      string // --home
	yes       bool
	noColor   bool
	paths     paths.Paths
}

func (a *app) now() time.Time {
	if a.io.Now != nil {
		return a.io.Now()
	}
	return time.Now()
}

func (a *app) newRoot() *cobra.Command {
	root := &cobra.Command{
		Use:               "squad",
		Short:             "Switch Claude Code agent teams with one command",
		Version:           version.String(),
		SilenceUsage:      true,
		SilenceErrors:     true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error { return a.prepare(cmd) },
	}
	root.SetOut(a.io.Out)
	root.SetErr(a.io.Err)
	root.SetVersionTemplate("{{.Version}}\n")

	flags := root.PersistentFlags()
	flags.StringVar(&a.claudeDir, "claude-dir", "", "Claude Code config directory (default: $CLAUDE_CONFIG_DIR or ~/.claude)")
	flags.StringVar(&a.home, "home", "", "squadup data directory (default: $SQUADUP_HOME or ~/.squadup)")
	flags.BoolVarP(&a.yes, "yes", "y", false, "skip confirmations")
	flags.BoolVar(&a.noColor, "no-color", false, "disable colors (NO_COLOR is also respected)")

	root.AddCommand(newDoctor(), a.newList(), a.newStatus(), a.newCapture(), a.newInitial())
	return root
}

// prepare resolves the locations and, on the very first run, captures Initial
// before any command does its work.
func (a *app) prepare(cmd *cobra.Command) error {
	if skipsPrepare(cmd) {
		return nil
	}
	p, err := paths.Resolve(a.claudeDir, a.home)
	if err != nil {
		return &ExitError{Code: 1, Err: err, Fix: "pass --claude-dir and --home explicitly"}
	}
	a.paths = p

	if initial.Exists(p) {
		return nil
	}
	meta, err := initial.Capture(p, a.now())
	if errors.Is(err, initial.ErrExists) {
		return nil
	}
	if err != nil {
		return &ExitError{
			Code: 1,
			Err:  fmt.Errorf("save your original setup: %w", err),
			Fix:  fmt.Sprintf("make sure %s is readable and %s is writable", p.ClaudeDir, p.Home),
		}
	}
	renderWelcome(a.io.Err, meta)
	return nil
}

// skipsPrepare is true for help and shell completion, which must never
// create files.
func skipsPrepare(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		switch c.Name() {
		case "help", "completion", cobra.ShellCompRequestCmd, cobra.ShellCompNoDescRequestCmd:
			return true
		}
	}
	return false
}

// confirm asks a yes/no question on stderr and reads the answer from stdin.
func (a *app) confirm(prompt string) (bool, error) {
	printf(a.io.Err, "%s", prompt)
	line, err := bufio.NewReader(a.io.In).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}

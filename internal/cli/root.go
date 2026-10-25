// Package cli wires the squad command line.
package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"

	"github.com/EdrenaVosh/squad-up/internal/initial"
	"github.com/EdrenaVosh/squad-up/internal/paths"
	"github.com/EdrenaVosh/squad-up/internal/tui"
	"github.com/EdrenaVosh/squad-up/internal/version"
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
	Terminal    bool             // whether Out is a terminal
	Now         func() time.Time // defaults to time.Now
	HTTP        *http.Client     // for https imports; nil means a default client
	// TUI opens the terminal interface; nil means the real one.
	TUI func(env tui.Env, in io.Reader, out io.Writer) error
}

// Execute runs squad on the real terminal and returns the exit code.
func Execute(args []string, stdout, stderr io.Writer) int {
	return Run(args, IO{
		In:          os.Stdin,
		Out:         stdout,
		Err:         stderr,
		Interactive: term.IsTerminal(os.Stdin.Fd()),
		Terminal:    term.IsTerminal(os.Stdout.Fd()),
	})
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
	welcome   *initial.Meta // set on the very first run
	input     *bufio.Reader // shared by every prompt so that buffered answers are not lost
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
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error { return a.prepare(cmd, args) },
		RunE: func(cmd *cobra.Command, _ []string) error {
			if a.canOpenInterface() {
				return a.openInterface(tui.Start{})
			}
			return cmd.Help()
		},
	}
	root.SetOut(a.io.Out)
	root.SetErr(a.io.Err)
	root.SetVersionTemplate("{{.Version}}\n")

	flags := root.PersistentFlags()
	flags.StringVar(&a.claudeDir, "claude-dir", "", "Claude Code config directory (default: $CLAUDE_CONFIG_DIR or ~/.claude)")
	flags.StringVar(&a.home, "home", "", "squad-up data directory (default: $SQUAD_UP_HOME or ~/.squad-up)")
	flags.BoolVarP(&a.yes, "yes", "y", false, "skip confirmations")
	flags.BoolVar(&a.noColor, "no-color", false, "disable colors (NO_COLOR is also respected)")

	root.AddCommand(newDoctor(), a.newList(), a.newStatus(), a.newCapture(), a.newInitial(),
		a.newUse(), a.newRestore(), a.newUndo(), a.newLog(), a.newRun(), a.newModels(), a.newLint(),
		a.newNew(), a.newClone(), a.newRename(), a.newRm(), a.newEdit(), a.newExport(), a.newImport(), a.newTemplates(), a.newCard(), a.newPin(), a.newFlow(), a.newUpdate(), a.newUsage(), a.newStatusline())
	return root
}

// prepare resolves the locations and, on the very first run, captures Initial
// before any command does its work.
func (a *app) prepare(cmd *cobra.Command, args []string) error {
	if skipsPrepare(cmd) {
		return nil
	}
	p, err := paths.Resolve(a.claudeDir, a.home)
	if err != nil {
		return &ExitError{Code: 1, Err: err, Fix: "pass --claude-dir and --home explicitly"}
	}
	a.paths = p
	if isStatuslineLine(cmd) {
		return nil // run by Claude Code on every redraw: no first-run capture, no welcome
	}

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
	a.welcome = meta
	if !cmd.HasParent() && len(args) == 0 && a.canOpenInterface() {
		return nil // the interface shows the welcome itself
	}
	renderWelcome(a.io.Err, meta)
	return nil
}

// canOpenInterface is true when a person sits at a color terminal. Pipes,
// NO_COLOR, CI and a dumb terminal get plain text instead.
func (a *app) canOpenInterface() bool {
	return a.io.Interactive && a.useColor() && os.Getenv("CI") == ""
}

// useColor is true when the output is a color terminal.
func (a *app) useColor() bool {
	return a.io.Terminal && !a.noColor && os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb"
}

// openInterface runs the terminal interface, starting on the given screen.
func (a *app) openInterface(start tui.Start) error {
	env := tui.Env{Paths: a.paths, Now: a.now, Start: start, Animate: os.Getenv("SQUAD_UP_NO_ANIM") == "", Live: true, Cwd: start.Cwd}
	if start.Kind == tui.StartHome {
		env.Welcome = a.welcome
	}
	run := a.io.TUI
	if run == nil {
		run = tui.Run
	}
	if err := run(env, a.io.In, a.io.Out); err != nil {
		return &ExitError{Code: 1, Err: fmt.Errorf("the terminal interface stopped: %w", err), Fix: "use the commands instead; `squad --help` lists them"}
	}
	return nil
}

// isStatuslineLine is true for `squad statusline` itself, the command that
// Claude Code runs to draw its status line.
func isStatuslineLine(cmd *cobra.Command) bool {
	return cmd.Name() == "statusline" && cmd.HasParent() && !cmd.Parent().HasParent()
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

// ask prints a prompt on stderr and returns the line typed in answer. At the
// end of input it returns what was read so far, without an error.
func (a *app) ask(prompt string) (string, error) {
	printf(a.io.Err, "%s", prompt)
	if a.input == nil {
		a.input = bufio.NewReader(a.io.In)
	}
	line, err := a.input.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return line, nil
}

// confirm asks a yes/no question; only "y" or "yes" means yes.
func (a *app) confirm(prompt string) (bool, error) {
	line, err := a.ask(prompt)
	if err != nil {
		return false, err
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}

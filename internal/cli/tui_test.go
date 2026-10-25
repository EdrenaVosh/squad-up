package cli_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/EdrenaVosh/squad-up/internal/cli"
	"github.com/EdrenaVosh/squad-up/internal/tui"
)

// tuiRun runs squad as if on a terminal, with a fake interface that records
// how it was opened.
type tuiRun struct {
	result
	opened int
	env    tui.Env
}

func (e *env) runOnTerminal(tuiErr error, args ...string) tuiRun {
	e.t.Helper()
	var out, errOut bytes.Buffer
	var got tuiRun
	code := cli.Run(args, cli.IO{
		In:          strings.NewReader(""),
		Out:         &out,
		Err:         &errOut,
		Interactive: true,
		Terminal:    true,
		Now:         func() time.Time { return fixedNow },
		TUI: func(env tui.Env, _ io.Reader, _ io.Writer) error {
			got.opened++
			got.env = env
			return tuiErr
		},
	})
	got.result = result{stdout: out.String(), stderr: errOut.String(), code: code}
	return got
}

func TestBareSquadOpensTheInterfaceOnATerminal(t *testing.T) {
	e := ready(t)
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("NO_COLOR", "")
	t.Setenv("CI", "")

	r := e.runOnTerminal(nil)

	if r.code != 0 || r.opened != 1 {
		t.Fatalf("exit code = %d, opened %d times\n%s", r.code, r.opened, r.stderr)
	}
	if r.env.Paths.ClaudeDir != e.claudeDir || r.env.Paths.Home != e.home {
		t.Errorf("env paths = %+v", r.env.Paths)
	}
	if r.env.Welcome != nil {
		t.Error("Initial was captured earlier: there is no welcome screen")
	}
	if r.env.Start.Kind != tui.StartHome {
		t.Errorf("start = %+v", r.env.Start)
	}
}

func TestTheFirstRunGoesToTheWelcomeScreenInsteadOfPrintingText(t *testing.T) {
	e := newEnv(t)
	e.writeSetup()
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("NO_COLOR", "")
	t.Setenv("CI", "")

	r := e.runOnTerminal(nil)

	if r.opened != 1 || r.env.Welcome == nil {
		t.Fatalf("opened %d times, welcome = %v", r.opened, r.env.Welcome)
	}
	if got := r.env.Welcome.Summary().AgentNames; len(got) != 3 {
		t.Errorf("welcome agents = %v", got)
	}
	if strings.Contains(r.stderr, "Welcome to squad-up") {
		t.Errorf("the text summary must not be printed when the interface shows it:\n%s", r.stderr)
	}
}

func TestBareSquadWithoutATerminalPrintsHelp(t *testing.T) {
	e := ready(t)

	r := e.run()

	if r.code != 0 || !strings.Contains(r.stdout, "Usage:") || !strings.Contains(r.stdout, "squad [command]") {
		t.Fatalf("exit code = %d\n%s%s", r.code, r.stdout, r.stderr)
	}
}

func TestNoColorCiAndDumbTerminalsGetText(t *testing.T) {
	for name, set := range map[string]func(*testing.T){
		"NO_COLOR": func(t *testing.T) { t.Setenv("NO_COLOR", "1") },
		"CI":       func(t *testing.T) { t.Setenv("CI", "true") },
		"TERM":     func(t *testing.T) { t.Setenv("TERM", "dumb") },
	} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			e.writeSetup()
			t.Setenv("TERM", "xterm-256color")
			t.Setenv("NO_COLOR", "")
			t.Setenv("CI", "")
			set(t)

			r := e.runOnTerminal(nil)

			if r.opened != 0 {
				t.Fatal("the interface must not open")
			}
			if !strings.Contains(r.stdout, "Usage:") || !strings.Contains(r.stderr, "Welcome to squad-up") {
				t.Errorf("text mode prints help and the summary:\nstdout:\n%s\nstderr:\n%s", r.stdout, r.stderr)
			}
			if strings.Contains(r.stdout+r.stderr, "\x1b[") {
				t.Error("text mode must not print ANSI sequences")
			}
		})
	}
}

func TestTheNoColorFlagKeepsTheInterfaceClosed(t *testing.T) {
	e := ready(t)
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("NO_COLOR", "")
	t.Setenv("CI", "")

	r := e.runOnTerminal(nil, "--no-color")

	if r.opened != 0 || !strings.Contains(r.stdout, "Usage:") {
		t.Fatalf("opened %d times\n%s", r.opened, r.stdout)
	}
}

func TestAnInterfaceFailureIsReportedWithAFix(t *testing.T) {
	e := ready(t)
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("NO_COLOR", "")
	t.Setenv("CI", "")

	r := e.runOnTerminal(errors.New("could not open the terminal"))

	if r.code != 1 || !strings.Contains(r.stderr, "could not open the terminal") || !strings.Contains(r.stderr, "Fix:") {
		t.Fatalf("exit code = %d\n%s", r.code, r.stderr)
	}
}

func TestNewWithoutANameOpensTheCreateDialogOnATerminal(t *testing.T) {
	e := ready(t)
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("NO_COLOR", "")
	t.Setenv("CI", "")

	r := e.runOnTerminal(nil, "new")

	if r.opened != 1 || r.env.Start.Kind != tui.StartNew {
		t.Fatalf("opened %d times, start = %+v\n%s", r.opened, r.env.Start, r.stderr)
	}
}

func TestEditOpensTheEditorOfThePresetOrOfOneAgent(t *testing.T) {
	e := ready(t)
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("NO_COLOR", "")
	t.Setenv("CI", "")

	preset := e.runOnTerminal(nil, "edit", "solo")
	if preset.opened != 1 || preset.env.Start != (tui.Start{Kind: tui.StartEdit, Preset: "solo"}) {
		t.Errorf("edit solo: opened %d, start = %+v\n%s", preset.opened, preset.env.Start, preset.stderr)
	}
	agent := e.runOnTerminal(nil, "edit", "solo", "planner")
	if agent.env.Start != (tui.Start{Kind: tui.StartEdit, Preset: "solo", Agent: "planner"}) {
		t.Errorf("edit solo planner: start = %+v", agent.env.Start)
	}
	if missing := e.runOnTerminal(nil, "edit", "solo", "ghost"); missing.opened != 0 || missing.code != 1 {
		t.Errorf("edit solo ghost: opened %d, exit code %d", missing.opened, missing.code)
	}
}

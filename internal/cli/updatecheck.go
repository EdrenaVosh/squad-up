package cli

import (
	"context"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/EdrenaVosh/squad-up/internal/tui"
	"github.com/EdrenaVosh/squad-up/internal/updatecheck"
	"github.com/EdrenaVosh/squad-up/internal/version"
)

const (
	autoCheckOff = "squad update --auto-check off"
	autoCheckOn  = "squad update --auto-check on"
	// checkNotice tells, once, that squad asks GitHub and how to stop it.
	checkNotice = "squad looks for a new release on GitHub once a day. Turn it off: " + autoCheckOff
	// checkNote is the same for the footer of the interface, short enough
	// for 80 columns.
	checkNote = "squad checks for updates daily · turn off: " + autoCheckOff
	// announceWait is how long a command waits at the end for a check that
	// is still running; a slower answer is used on the next run.
	announceWait = time.Second
)

func (a *app) checker() updatecheck.Checker {
	return updatecheck.Checker{File: a.paths.UpdateCheckFile(), Current: version.Version, HTTP: a.io.HTTP, Now: a.io.Now}
}

// checksUpdates is true when a person runs squad at a color terminal: never
// in a pipe, a script or CI, and never for the status line, which Claude Code
// runs on every redraw.
func (a *app) checksUpdates(cmd *cobra.Command) bool {
	return a.useColor() && os.Getenv("CI") == "" && !isStatuslineLine(cmd)
}

// startUpdateCheck asks GitHub in the background, at most once a day, while
// the command does its work. The interface and squad update ask for
// themselves.
func (a *app) startUpdateCheck(cmd *cobra.Command) {
	if !a.checksUpdates(cmd) || !cmd.HasParent() || cmd.Name() == "update" {
		return
	}
	done := make(chan struct{})
	a.updateDone = done
	go func() {
		defer close(done)
		a.checker().Refresh(context.Background())
	}()
}

// announceUpdate prints a line about a newer release after a command, and
// the first time, how to turn the check off.
func (a *app) announceUpdate() {
	if a.updateDone == nil {
		return
	}
	finished := true
	select {
	case <-a.updateDone:
	case <-time.After(announceWait):
		finished = false
	}
	c := a.checker()
	if latest := c.Known(); latest != "" {
		printf(a.io.Err, "\n\033[1;38;2;247;107;21m⬆ squad %s is out (you have %s). Run: squad update\033[0m\n", latest, version.Version)
	}
	// while the check still writes the file, the note waits for the next run
	if finished && c.Enabled() && c.MarkNoticed() {
		printf(a.io.Err, "\n\033[2m%s\033[0m\n", checkNotice)
	}
}

// interfaceUpdates gives the interface its background check and, the first
// time, the note about it.
func (a *app) interfaceUpdates(env *tui.Env) {
	c := a.checker()
	env.CheckUpdate = func() *tui.Update {
		c.Refresh(context.Background())
		if latest := c.Known(); latest != "" {
			return &tui.Update{Latest: latest, Current: version.Version}
		}
		return nil
	}
	if c.Enabled() && c.MarkNoticed() {
		env.Note = checkNote
	}
}

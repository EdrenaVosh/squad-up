package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/EdrenaVosh/squadup/internal/apply"
	"github.com/EdrenaVosh/squadup/internal/initial"
	"github.com/EdrenaVosh/squadup/internal/tui"
)

func (a *app) newInitial() *cobra.Command {
	var recapture bool
	cmd := &cobra.Command{
		Use:   "initial",
		Short: "Show your original setup, or capture it again",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			meta, err := initial.Load(a.paths)
			if err != nil {
				return &ExitError{Code: 1, Err: err, Fix: "run any squad command once to capture it"}
			}
			if !recapture {
				renderInitial(a.io.Out, meta)
				return nil
			}
			return a.recapture()
		},
	}
	cmd.Flags().BoolVar(&recapture, "recapture", false, "replace Initial with the current setup (the old one is kept in backups)")
	return cmd
}

func (a *app) recapture() error {
	state, err := apply.LoadState(a.paths)
	if err != nil {
		return &ExitError{Code: 1, Err: err, Fix: "fix or delete the state file; files squad wrote stay in place"}
	}
	if active := state.ActivePreset(); active != "initial" {
		return &ExitError{
			Code: 1,
			Err:  fmt.Errorf("preset %q is equipped, so %s no longer matches your original setup", active, a.paths.ClaudeDir),
			Fix:  "run `squad restore` first, then recapture",
		}
	}

	if !a.yes {
		if !a.io.Interactive {
			return &ExitError{
				Code: 1,
				Err:  errors.New("recapturing replaces the saved Initial snapshot and needs confirmation"),
				Fix:  "re-run with --yes to confirm",
			}
		}
		ok, err := a.confirm(tui.RecapturePrompt)
		if err != nil {
			return &ExitError{Code: 1, Err: fmt.Errorf("read the answer: %w", err)}
		}
		if !ok {
			printf(a.io.Err, "%s\n", tui.Cancelled)
			return &ExitError{Code: 3}
		}
	}

	backup, meta, err := initial.Recapture(a.paths, a.now())
	if err != nil {
		return &ExitError{Code: 1, Err: err, Fix: "check the permissions of " + a.paths.Home}
	}
	printf(a.io.Out, "Recaptured Initial from your current setup. The previous snapshot is saved in %s.\n\n", backup)
	renderInitial(a.io.Out, meta)
	return nil
}

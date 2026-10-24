package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/EdrenaVosh/squadup/internal/preset"
)

func (a *app) newFlow() *cobra.Command {
	var gate, fix string
	var rounds int
	var order, off bool
	cmd := &cobra.Command{
		Use:   "flow <preset>",
		Short: "Show or set how a preset's agents work as a team, and the review loop",
		Long: "A flow makes the agents a team: the main agent hands work down the slots in order. With --gate and --fix the agent\n" +
			"named by --gate checks the result and the one named by --fix gets its defects back, for up to --rounds rounds (3 by default).\n" +
			"Without flags the flow is shown. Agents share a tiny notes file in the project (.squad/notes.md); `squad edit --isolate` opts one out.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			w := a.workspace()
			p, err := w.Store().LoadPreset(name)
			if err != nil {
				return libraryError(err, cloneInitialFix)
			}
			changing := gate != "" || fix != "" || order || off || cmd.Flags().Changed("rounds")
			if !changing {
				printFlow(a, *p)
				return nil
			}
			if off && (gate != "" || fix != "" || order) {
				return &ExitError{Code: 2, Err: errors.New("--off cannot be combined with the other flags"), Fix: "use `squad flow <preset> --off` on its own"}
			}

			var next *preset.Flow
			if !off {
				next = &preset.Flow{Gate: gate, Fix: fix, Rounds: rounds}
				if p.Flow != nil && !cmd.Flags().Changed("rounds") {
					next.Rounds = p.Flow.Rounds
				}
				if gate == "" && fix == "" && p.Flow != nil {
					next.Gate, next.Fix = p.Flow.Gate, p.Flow.Fix
				}
			}
			if err := w.SetFlow(name, next); err != nil {
				var verr *ExitError
				if errors.As(err, &verr) {
					return err
				}
				if strings.Contains(err.Error(), "flow") {
					return &ExitError{Code: 1, Err: err, Fix: "`squad lint " + name + "` shows the agents of the preset"}
				}
				return libraryError(err, cloneInitialFix)
			}
			updated, err := w.Store().LoadPreset(name)
			if err != nil {
				return libraryError(err, cloneInitialFix)
			}
			printFlow(a, *updated)
			return nil
		},
	}
	cmd.Flags().StringVar(&gate, "gate", "", "the agent that checks the work last, e.g. reviewer")
	cmd.Flags().StringVar(&fix, "fix", "", "the agent that gets the gate's defects back, e.g. developer")
	cmd.Flags().IntVar(&rounds, "rounds", 0, fmt.Sprintf("how many times defects may be sent back (default %d, at most %d)", preset.DefaultRounds, preset.MaxRounds))
	cmd.Flags().BoolVar(&order, "order", false, "work in slot order, without a review loop")
	cmd.Flags().BoolVar(&off, "off", false, "remove the flow")
	return cmd
}

func printFlow(a *app, p preset.Preset) {
	if p.Flow == nil {
		printf(a.io.Out, "%s has no flow: its agents are separate helpers.\n", p.Name)
		return
	}
	printf(a.io.Out, "Order   %s\n", strings.Join(p.Agents, " → "))
	if p.Flow.Loops() {
		printf(a.io.Out, "Loop    %s checks, %s fixes · up to %s\n", p.Flow.Gate, p.Flow.Fix, plural(p.Flow.MaxRounds(), "round"))
	} else {
		printf(a.io.Out, "Loop    none\n")
	}
	if p.Main == nil {
		printf(a.io.Out, "\nNo main agent: `squad run %s` passes the flow to Claude Code; an equipped preset needs a main agent to carry it.\n", p.Name)
	}
}

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/EdrenaVosh/squadup/internal/claude"
	"github.com/EdrenaVosh/squadup/internal/runner"
	"github.com/EdrenaVosh/squadup/internal/tui"
)

func (a *app) newRun() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "run [preset] [-- claude arguments]",
		Short: "Start claude with a preset for this session only; nothing is written to ~/.claude",
		Long: "Without a preset name, the one pinned by the nearest .squadup file (see `squad pin`) runs.\n" +
			"Starts claude with the preset's agents (--agents) and settings (--settings) for one session.\n" +
			"Nothing is written to your Claude config. The agents are added to your own agents in ~/.claude/agents,\n" +
			"they do not replace them: Claude Code offers no way to hide those.\n" +
			"Everything after -- goes to claude, after squad's own flags, so it can override them.",
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			names, extra := args, []string(nil)
			if dash := cmd.ArgsLenAtDash(); dash >= 0 {
				names, extra = args[:dash], args[dash:]
			}
			if len(names) == 0 {
				pin, err := a.projectPin()
				if err != nil {
					return err
				}
				if pin == nil {
					return &ExitError{Code: 1, Err: errors.New("which preset should run?"), Fix: "use `squad run <preset> [-- claude arguments]`, or pin one to this project with `squad pin <preset>`; `squad list` shows the presets"}
				}
				printf(a.io.Err, "Using %q, pinned by %s\n", pin.Preset, pin.File)
				names = []string{pin.Preset}
			}
			if len(names) != 1 {
				return &ExitError{Code: 1, Err: errors.New("which preset should run?"), Fix: "use `squad run <preset> [-- claude arguments]`; `squad list` shows the presets"}
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			return a.runPreset(ctx, names[0], extra, dryRun)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be started and stop")
	return cmd
}

func (a *app) runPreset(ctx context.Context, name string, extra []string, dryRun bool) error {
	if name == "initial" {
		return &ExitError{Code: 1, Err: errors.New("initial is your own setup, there is nothing to add to it"), Fix: "run claude directly"}
	}
	bundle, resolved, err := a.engine().Resolve(name)
	if err != nil {
		return planError(err)
	}
	spec, err := runner.Build(*bundle, resolved)
	if err != nil {
		return &ExitError{Code: 1, Err: err, Fix: "shorten the prompts of the preset, or equip it with `squad use " + name + "`"}
	}

	for _, w := range a.workspace().ModelWarnings(ctx, bundle) {
		printf(a.io.Err, "! %s\n", w)
	}
	if m := os.Getenv("ANTHROPIC_MODEL"); m != "" && resolved["model"] != "" {
		printf(a.io.Err, "! %s\n", tui.ModelOverride(m))
	}

	if dryRun {
		a.printRunPlan(spec, extra)
		return nil
	}

	bin, err := claude.Find()
	if err != nil {
		return &ExitError{Code: 1, Err: errors.New("claude is not installed or not on PATH"), Fix: "install Claude Code (https://code.claude.com/docs) and make sure `claude` runs in this shell"}
	}
	printf(a.io.Err, "Running claude with %q (+ your global agents). Nothing is written to your Claude config.\n", name)
	started := a.now()
	code, err := runner.Run(bin, spec, extra, runner.Streams{In: a.io.In, Out: a.io.Out, Err: a.io.Err})
	a.printRunUsage(started)
	if err != nil {
		return &ExitError{Code: 1, Err: err, Fix: "run `squad doctor` to check claude"}
	}
	if code != 0 {
		return &ExitError{Code: code}
	}
	return nil
}

func (a *app) printRunPlan(spec runner.Spec, extra []string) {
	parts := []string{"claude"}
	if spec.AgentsJSON != "" {
		parts = append(parts, "--agents", "'<agents below>'")
	}
	parts = append(parts, "--settings", "<temporary file with the settings below>")
	if spec.Main != "" {
		parts = append(parts, "--agent", spec.Main)
	}
	parts = append(parts, extra...)
	printf(a.io.Out, "Would run:\n  %s\n", strings.Join(parts, " "))

	if spec.AgentsJSON != "" {
		var pretty bytes.Buffer
		if json.Indent(&pretty, []byte(spec.AgentsJSON), "", "  ") != nil {
			pretty.WriteString(spec.AgentsJSON)
		}
		printf(a.io.Out, "\nAgents (--agents):\n%s\n", pretty.String())
	}
	printf(a.io.Out, "\nSettings (--settings):\n%s", spec.SettingsJSON)
}

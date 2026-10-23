package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/EdrenaVosh/squadup/internal/apply"
	"github.com/EdrenaVosh/squadup/internal/fsx"
	"github.com/EdrenaVosh/squadup/internal/preset"
	"github.com/EdrenaVosh/squadup/internal/tui"
)

// opFlags are the flags shared by use, restore and undo.
type opFlags struct {
	dryRun  bool
	force   bool
	save    bool
	discard bool
}

func (f *opFlags) register(cmd *cobra.Command) {
	flags := cmd.Flags()
	flags.BoolVar(&f.dryRun, "dry-run", false, "show what would change and stop")
	flags.BoolVar(&f.force, "force", false, "overwrite files squad did not write (a backup is taken first)")
	flags.BoolVar(&f.save, "save-edits", false, "keep your manual edits of the equipped preset by saving them into it")
	flags.BoolVar(&f.discard, "discard-edits", false, "overwrite your manual edits of the equipped preset")
}

func (f opFlags) options() (apply.Options, error) {
	if f.save && f.discard {
		return apply.Options{}, &ExitError{Code: 1, Err: errors.New("--save-edits and --discard-edits exclude each other"), Fix: "pass only one of them"}
	}
	opts := apply.Options{Force: f.force}
	switch {
	case f.save:
		opts.Edits = apply.EditsSave
	case f.discard:
		opts.Edits = apply.EditsDiscard
	}
	return opts, nil
}

func (a *app) engine() apply.Engine {
	return apply.Engine{Paths: a.paths, Now: a.now}
}

func (a *app) newUse() *cobra.Command {
	var f opFlags
	cmd := &cobra.Command{
		Use:   "use <preset>",
		Short: "Equip a preset: write its agents and settings into ~/.claude",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			opts, err := f.options()
			if err != nil {
				return err
			}
			plan, err := a.engine().PlanUse(args[0], opts)
			if err != nil {
				return planError(err)
			}
			return a.operate(plan, opts, f)
		},
	}
	f.register(cmd)
	return cmd
}

func (a *app) newRestore() *cobra.Command {
	var f opFlags
	cmd := &cobra.Command{
		Use:   "restore",
		Short: "Return Claude Code to your original setup (Initial)",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			opts, err := f.options()
			if err != nil {
				return err
			}
			plan, err := a.engine().PlanUse("initial", opts)
			if err != nil {
				return planError(err)
			}
			return a.operate(plan, opts, f)
		},
	}
	f.register(cmd)
	return cmd
}

func (a *app) newUndo() *cobra.Command {
	var f opFlags
	var to string
	cmd := &cobra.Command{
		Use:   "undo",
		Short: "Undo the last change, or go back to a point from `squad log`",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			opts, err := f.options()
			if err != nil {
				return err
			}
			plan, err := a.engine().PlanUndo(to, opts)
			if err != nil {
				return planError(err)
			}
			return a.operate(plan, opts, f)
		},
	}
	f.register(cmd)
	cmd.Flags().StringVar(&to, "to", "", "id from `squad log`: return to the state before that operation")
	return cmd
}

// planError turns planning failures into user errors with a fix.
func planError(err error) error {
	switch {
	case errors.Is(err, preset.ErrNotFound):
		return &ExitError{Code: 1, Err: err, Fix: "run `squad list` to see the available presets"}
	case errors.Is(err, apply.ErrNothingToUndo):
		return &ExitError{Code: 1, Err: err, Fix: "change something first with `squad use <preset>`"}
	case errors.Is(err, apply.ErrNoBackup):
		return &ExitError{Code: 1, Err: err, Fix: "run `squad log` to see the points you can return to"}
	}
	return &ExitError{Code: 1, Err: err, Fix: "fix the problem above and run the command again"}
}

// operate shows the plan, gets the user's decisions and executes it.
func (a *app) operate(plan *apply.Plan, opts apply.Options, f opFlags) error {
	if plan.NoOp {
		printf(a.io.Out, "%s\n", tui.AlreadyEquipped(plan.Target))
		return nil
	}
	renderPlan(a.io.Out, plan)

	if len(plan.Conflicts) > 0 {
		return &ExitError{
			Code: 1,
			Err:  errors.New("some files are in the way"),
			Fix:  "move them away, or re-run with --force (a backup is taken first)",
		}
	}
	if f.dryRun {
		printf(a.io.Out, "\n%s\n", tui.DryRun)
		return nil
	}

	if len(plan.Drift) > 0 && opts.Edits == apply.EditsAsk {
		policy, err := a.decideEdits()
		if err != nil {
			return err
		}
		opts.Edits = policy
	}
	if !a.yes {
		if !a.io.Interactive {
			return &ExitError{Code: 1, Err: errors.New("changing your config needs confirmation"), Fix: "re-run with --yes to confirm"}
		}
		ok, err := a.confirm(tui.ApplyPrompt)
		if err != nil {
			return &ExitError{Code: 1, Err: fmt.Errorf("read the answer: %w", err)}
		}
		if !ok {
			return a.cancelled()
		}
	}

	res, err := a.engine().Execute(plan, opts)
	if err != nil {
		return executeError(err)
	}

	switch plan.Command {
	case "use":
		printf(a.io.Out, "\n%s\n", tui.Equipped(plan.Target, res.BackupID))
	case "restore":
		printf(a.io.Out, "\n%s\n", tui.Restored(res.BackupID))
	case "undo":
		printf(a.io.Out, "\n%s\n", tui.Undone(plan.Target, res.BackupID))
	}
	for _, note := range plan.Notes {
		printf(a.io.Out, "Note: %s\n", note)
	}
	return nil
}

// decideEdits settles what happens to manual edits: ask on a terminal,
// otherwise require an explicit flag.
func (a *app) decideEdits() (apply.EditsPolicy, error) {
	if !a.io.Interactive {
		return apply.EditsAsk, &ExitError{
			Code: 1,
			Err:  errors.New("manual edits detected in the equipped preset"),
			Fix:  "re-run with --save-edits to keep them in the preset, or --discard-edits to overwrite them",
		}
	}
	for {
		answer, err := a.ask(tui.DriftPrompt)
		if err != nil {
			return apply.EditsAsk, &ExitError{Code: 1, Err: fmt.Errorf("read the answer: %w", err)}
		}
		switch strings.ToLower(strings.TrimSpace(answer)) {
		case "s", "save":
			return apply.EditsSave, nil
		case "d", "discard":
			return apply.EditsDiscard, nil
		case "c", "cancel", "":
			return apply.EditsAsk, a.cancelled()
		}
	}
}

func (a *app) cancelled() error {
	printf(a.io.Err, "%s\n", tui.Cancelled)
	return &ExitError{Code: 3}
}

// executeError turns an Execute failure into a user error with a fix.
func executeError(err error) error {
	switch {
	case errors.Is(err, fsx.ErrLocked):
		return &ExitError{Code: 1, Err: err, Fix: "wait for the other squad to finish"}
	case errors.Is(err, apply.ErrStale):
		return &ExitError{Code: 1, Err: err, Fix: "run the command again to see the new plan"}
	case errors.Is(err, apply.ErrConflicts):
		return &ExitError{Code: 1, Err: err, Fix: "move the files away, or re-run with --force (a backup is taken first)"}
	case errors.Is(err, apply.ErrDrift):
		return &ExitError{Code: 1, Err: err, Fix: "re-run with --save-edits or --discard-edits"}
	}
	return &ExitError{Code: 1, Err: err, Fix: "check that the Claude config and ~/.squadup are writable, then try again"}
}

func renderPlan(w io.Writer, p *apply.Plan) {
	switch p.Command {
	case "use":
		printf(w, "Equip %q (mode: %s)\n", p.Target, p.Mode)
	case "restore":
		printf(w, "Restore your original setup (Initial)\n")
	case "undo":
		printf(w, "Undo: return to the state before backup %s (equipped afterwards: %s)\n", p.BackupID, p.Target)
	}

	if len(p.Files) == 0 && len(p.Settings) == 0 {
		printf(w, "\n  %s\n", tui.NoFileChanges)
	}
	if len(p.Files) > 0 {
		printf(w, "\n  agents\n")
		for _, op := range p.Files {
			printf(w, "    %s %s\n", opMark(op.Kind), op.Path)
		}
	}
	if len(p.Settings) > 0 {
		printf(w, "\n  settings.json\n")
		for _, op := range p.Settings {
			switch op.Kind {
			case apply.OpAdd:
				printf(w, "    + %s: %s\n", op.Key, op.New)
			case apply.OpRemove:
				printf(w, "    - %s: %s\n", op.Key, op.Old)
			default:
				printf(w, "    ~ %s: %s → %s\n", op.Key, op.Old, op.New)
			}
		}
	}

	if len(p.Drift) > 0 {
		printf(w, "\n%s\n", tui.ManualEditsHeader)
		for _, d := range p.Drift {
			printf(w, "    ~ %s (%s)\n", d.Path, d.Kind)
		}
	}
	if len(p.Conflicts) > 0 {
		printf(w, "\n%s\n", tui.ConflictsHeader)
		for _, c := range p.Conflicts {
			printf(w, "    ! %s\n", c)
		}
	}
	warnings := p.Warnings
	if model := os.Getenv("ANTHROPIC_MODEL"); model != "" && changesKey(p, "model") {
		warnings = append(warnings, tui.ModelOverride(model))
	}
	for _, warning := range warnings {
		printf(w, "\n! %s\n", warning)
	}
}

func changesKey(p *apply.Plan, key string) bool {
	for _, op := range p.Settings {
		if op.Key == key {
			return true
		}
	}
	return false
}

func opMark(kind apply.OpKind) string {
	switch kind {
	case apply.OpAdd:
		return "+"
	case apply.OpRemove:
		return "-"
	}
	return "~"
}

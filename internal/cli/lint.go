package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/EdrenaVosh/squadup/internal/lint"
	"github.com/EdrenaVosh/squadup/internal/preset"
)

type lintSummary struct {
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
	Infos    int `json:"infos"`
}

type lintOutput struct {
	Targets []lint.Report `json:"targets"`
	Summary lintSummary   `json:"summary"`
}

func (a *app) newLint() *cobra.Command {
	var asJSON, strict bool
	cmd := &cobra.Command{
		Use:   "lint [preset]",
		Short: "Check agents and presets for mistakes, and estimate what they cost in context",
		Long: "Checks one preset, or `initial` for your original agents, or everything when no name is given.\n" +
			"Exit code 2 means errors were found (with --strict, warnings too).",
		Args: cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			targets, err := a.lintTargets(args)
			if err != nil {
				return err
			}
			reports := make([]lint.Report, 0, len(targets))
			for _, name := range targets {
				report, err := a.workspace().Lint(name)
				if err != nil {
					return &ExitError{Code: 1, Err: err, Fix: "check the permissions of " + a.paths.Home}
				}
				reports = append(reports, report)
			}

			out := lintOutput{Targets: reports}
			for _, r := range reports {
				out.Summary.Errors += r.Count(lint.LevelError)
				out.Summary.Warnings += r.Count(lint.LevelWarn)
				out.Summary.Infos += r.Count(lint.LevelInfo)
			}
			if asJSON {
				if err := writeJSON(a.io.Out, out); err != nil {
					return err
				}
			} else {
				renderLint(a, out)
			}
			if out.Summary.Errors > 0 || (strict && out.Summary.Warnings > 0) {
				return &ExitError{Code: 2}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print machine-readable output")
	cmd.Flags().BoolVar(&strict, "strict", false, "fail on warnings too")
	return cmd
}

// lintTargets resolves the argument to preset names; Initial comes last.
func (a *app) lintTargets(args []string) ([]string, error) {
	store := preset.Store{Dir: a.paths.PresetsDir()}
	if len(args) == 1 {
		name := args[0]
		if name != "initial" && !store.Exists(name) {
			return nil, &ExitError{Code: 1, Err: fmt.Errorf("preset %q not found", name), Fix: "run `squad list` to see the available presets"}
		}
		return []string{name}, nil
	}
	names, err := store.Names()
	if err != nil {
		return nil, &ExitError{Code: 1, Err: err, Fix: "check the permissions of " + a.paths.PresetsDir()}
	}
	var out []string
	for _, n := range names {
		if n != "initial" {
			out = append(out, n)
		}
	}
	return append(out, "initial"), nil
}

func renderLint(a *app, out lintOutput) {
	for i, r := range out.Targets {
		if i > 0 {
			printf(a.io.Out, "\n")
		}
		printf(a.io.Out, "%s\n", r.Name)
		if len(r.Findings) == 0 {
			printf(a.io.Out, "  no problems\n")
		}
		for _, f := range r.Findings {
			printf(a.io.Out, "  %-5s  %-20s  %-20s  %s\n", f.Level, f.File, f.Rule, f.Message)
			if f.Fix != "" {
				printf(a.io.Out, "  %-5s  %-20s  %-20s  → %s\n", "", "", "", f.Fix)
			}
		}
		printf(a.io.Out, "  %s\n", costLine(r.Cost))
	}
	s := out.Summary
	printf(a.io.Out, "\n%s, %s, %s.\n", plural(s.Errors, "error"), plural(s.Warnings, "warning"), plural(s.Infos, "note"))
}

func costLine(c lint.Cost) string {
	if len(c.Agents) == 0 {
		return "cost: ~0 tokens per session (no agents)"
	}
	parts := make([]string, len(c.Agents))
	for i, ag := range c.Agents {
		parts[i] = fmt.Sprintf("%s ~%d", ag.Name, ag.PromptTokens)
	}
	return fmt.Sprintf("cost: ~%d tokens per session for names and descriptions; prompts load when an agent is called (%s)",
		c.SessionTokens, strings.Join(parts, ", "))
}

func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}

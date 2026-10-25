package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/EdrenaVosh/squadup/internal/apply"
	"github.com/EdrenaVosh/squadup/internal/editor"
	"github.com/EdrenaVosh/squadup/internal/fsx"
	"github.com/EdrenaVosh/squadup/internal/lint"
	"github.com/EdrenaVosh/squadup/internal/preset"
	"github.com/EdrenaVosh/squadup/internal/tui"
	"github.com/EdrenaVosh/squadup/internal/workspace"
	"github.com/EdrenaVosh/squadup/templates"
)

// libraryError turns a failure of a preset operation into a user error.
func libraryError(err error, protectedFix string) error {
	switch {
	case errors.Is(err, workspace.ErrInvalidName), errors.Is(err, workspace.ErrReservedName):
		return nameError(err)
	case errors.Is(err, preset.ErrNotFound):
		return &ExitError{Code: 1, Err: err, Fix: "run `squad list` to see the presets"}
	case errors.Is(err, preset.ErrExists):
		return &ExitError{Code: 1, Err: err, Fix: "choose another name"}
	case errors.Is(err, workspace.ErrProtected):
		return &ExitError{Code: 1, Err: err, Fix: protectedFix}
	case errors.Is(err, workspace.ErrEquipped):
		return &ExitError{Code: 1, Err: err, Fix: "re-run with --force; its files stay in your Claude config until you run `squad restore`"}
	case errors.Is(err, fsx.ErrLocked):
		return &ExitError{Code: 1, Err: err, Fix: "wait for the other squad to finish"}
	}
	return &ExitError{Code: 1, Err: err, Fix: "check the permissions of ~/.squadup"}
}

const cloneInitialFix = "make an editable copy with `squad clone initial <name>`"

// ---- new ----

func (a *app) newNew() *cobra.Command {
	var from, title, description, mode, template string
	cmd := &cobra.Command{
		Use:   "new [name]",
		Short: "Create a preset: blank, or a copy of another preset, Initial or the current setup",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 && template != "" {
				args = []string{template}
			}
			if len(args) == 0 {
				if a.canOpenInterface() {
					return a.openInterface(tui.Start{Kind: tui.StartNew})
				}
				return &ExitError{Code: 1, Err: errors.New("no preset name given"), Fix: "use `squad new <name>`"}
			}
			name := args[0]
			if template != "" && from != "" {
				return &ExitError{Code: 2, Err: errors.New("--template and --from cannot be combined"), Fix: "use one of them"}
			}
			if mode != "" && mode != preset.ModeReplace && mode != preset.ModeExtend {
				return &ExitError{Code: 2, Err: fmt.Errorf("mode %q is not valid", mode), Fix: "use replace or extend"}
			}
			if err := workspace.CheckName(name); err != nil {
				return nameError(err)
			}

			w := a.workspace()
			switch from {
			case "":
				if template != "" {
					if err := w.NewFromTemplate(template, name); err != nil {
						if errors.Is(err, templates.ErrUnknown) {
							return &ExitError{Code: 1, Err: err, Fix: "run `squad templates` to see the starters"}
						}
						return libraryError(err, cloneInitialFix)
					}
					count := ""
					if b, err := w.Store().LoadPreset(name); err == nil {
						count = " (" + tui.AgentCount(len(b.Agents)) + ")"
					}
					printf(a.io.Out, "Created preset %q from the template %q%s.\nLook at it with `squad lint %s`, equip it with `squad use %s`.\n", name, template, count, name, name)
					break
				}
				if _, err := w.New(name); err != nil {
					return libraryError(err, cloneInitialFix)
				}
				printf(a.io.Out, "Created preset %q with no agents.\nAdd some with `squad edit %s`, or start from your setup with `squad new <name> --from current`.\n", name, name)
			case "current":
				if err := a.capturePreset(name); err != nil {
					return err
				}
			default:
				skipped, err := w.Clone(from, name)
				if err != nil {
					return libraryError(err, cloneInitialFix)
				}
				a.reportSkipped(skipped)
				printf(a.io.Out, "Created preset %q as a copy of %q.\n", name, from)
			}

			return a.tweakPreset(name, title, description, mode)
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "copy a preset, `initial`, or the `current` Claude Code setup")
	cmd.Flags().StringVar(&template, "template", "", "start from a template (see `squad templates`)")
	cmd.Flags().StringVar(&title, "title", "", "title of the preset")
	cmd.Flags().StringVar(&description, "description", "", "description of the preset")
	cmd.Flags().StringVar(&mode, "mode", "", "replace (default): the preset replaces your agents; extend: it keeps them")
	return cmd
}

// tweakPreset applies the title, description and mode flags to a stored preset.
func (a *app) tweakPreset(name, title, description, mode string) error {
	if title == "" && description == "" && mode == "" {
		return nil
	}
	b, err := a.workspace().Store().Load(name)
	if err != nil {
		return libraryError(err, "")
	}
	if title != "" {
		b.Preset.Title = title
	}
	if description != "" {
		b.Preset.Description = description
	}
	if mode != "" {
		b.Preset.Mode = mode
	}
	if err := a.workspace().Save(*b); err != nil {
		return libraryError(err, "")
	}
	return nil
}

func (a *app) reportSkipped(skipped []preset.Skipped) {
	for _, s := range skipped {
		printf(a.io.Err, "Skipped %s: %s\n", s.Path, s.Reason)
	}
}

// ---- clone ----

func (a *app) newClone() *cobra.Command {
	return &cobra.Command{
		Use:   "clone <preset> <new-name>",
		Short: "Copy a preset (or Initial) under a new name",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			skipped, err := a.workspace().Clone(args[0], args[1])
			if err != nil {
				return libraryError(err, cloneInitialFix)
			}
			a.reportSkipped(skipped)
			count := ""
			if b, err := a.workspace().Store().LoadPreset(args[1]); err == nil {
				count = " (" + tui.AgentCount(len(b.Agents)) + ")"
			}
			printf(a.io.Out, "Copied %q to %q%s.\n", args[0], args[1], count)
			return nil
		},
	}
}

// ---- rename ----

func (a *app) newRename() *cobra.Command {
	return &cobra.Command{
		Use:   "rename <preset> <new-name>",
		Short: "Rename a preset",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			old, name := args[0], args[1]
			state, _ := apply.LoadState(a.paths)
			if err := a.workspace().Rename(old, name); err != nil {
				return libraryError(err, cloneInitialFix)
			}
			printf(a.io.Out, "Renamed %q to %q.\n", old, name)
			if state.Active == old {
				printf(a.io.Out, "It stays equipped.\n")
			}
			return nil
		},
	}
}

// ---- rm ----

func (a *app) newRm() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "rm <preset>",
		Short: "Delete a preset (not Initial; the equipped one only with --force)",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			name := args[0]
			if name == "initial" {
				return libraryError(workspace.ErrProtected, "use `squad restore` to go back to your original setup")
			}
			if !a.yes {
				if !a.io.Interactive {
					return &ExitError{Code: 1, Err: errors.New("deleting a preset needs confirmation"), Fix: "re-run with --yes to confirm"}
				}
				ok, err := a.confirm(fmt.Sprintf("Delete preset %q? [y/N] ", name))
				if err != nil {
					return &ExitError{Code: 1, Err: fmt.Errorf("read the answer: %w", err)}
				}
				if !ok {
					return a.cancelled()
				}
			}
			wasActive, err := a.workspace().Remove(name, force)
			if err != nil {
				return libraryError(err, "")
			}
			printf(a.io.Out, "Removed preset %q.\n", name)
			if wasActive {
				printf(a.io.Out, "Its files are still in your Claude config; `squad restore` takes them away.\n")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "also delete the equipped preset")
	return cmd
}

// ---- edit ----

func (a *app) newEdit() *cobra.Command {
	var raw bool
	var promptFile string
	var independent bool
	cmd := &cobra.Command{
		Use:   "edit <preset> [agent]",
		Short: "Edit a preset or one of its agents",
		Long:  "Opens the editor of the terminal interface. With --raw the file (preset.yaml, or the agent's .md) opens in $VISUAL or $EDITOR instead, and the result is checked afterwards.",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if name == "initial" {
				return libraryError(workspace.ErrProtected, cloneInitialFix)
			}
			w := a.workspace()
			if !w.Store().Exists(name) {
				return libraryError(fmt.Errorf("%w: %s", preset.ErrNotFound, name), "")
			}
			target := filepath.Join(a.paths.PresetDir(name), "preset.yaml")
			if len(args) == 2 {
				target = filepath.Join(a.paths.PresetDir(name), "agents", args[1]+".md")
				if !preset.ValidName(args[1]) || !fileExists(target) {
					return &ExitError{Code: 1, Err: fmt.Errorf("preset %q has no agent %q", name, args[1]), Fix: "run `squad lint " + name + "` to see its agents"}
				}
			}

			if promptFile != "" {
				return a.setPromptFromFile(name, args[1:], promptFile)
			}
			if cmd.Flags().Changed("independent") {
				return a.setIndependent(name, args[1:], independent)
			}
			if !raw {
				return a.editInterface(name, args[1:])
			}
			if err := a.runEditor(target); err != nil {
				return err
			}
			report, err := w.Lint(name)
			if err != nil {
				return &ExitError{Code: 1, Err: err, Fix: "run `squad lint " + name + "`"}
			}
			renderLint(a, lintOutput{Targets: []lint.Report{report}, Summary: summarize(report)})
			if report.Count(lint.LevelError) > 0 {
				return &ExitError{Code: 2}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&raw, "raw", false, "open the file in $EDITOR instead of the terminal interface")
	cmd.Flags().BoolVar(&independent, "independent", false, "make the agent independent (--independent=false undoes it): it judges without CLAUDE.md or knowing how the work was made, e.g. a code reviewer")
	cmd.Flags().StringVar(&promptFile, "prompt-file", "", "replace the agent's prompt with the text of this Markdown file")
	return cmd
}

func summarize(r lint.Report) lintSummary {
	return lintSummary{Errors: r.Count(lint.LevelError), Warnings: r.Count(lint.LevelWarn), Infos: r.Count(lint.LevelInfo)}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// editInterface opens the editor of the terminal interface, or explains how
// to edit without one.
func (a *app) editInterface(preset string, agent []string) error {
	if !a.canOpenInterface() {
		return &ExitError{
			Code: 1,
			Err:  errors.New("editing needs a terminal"),
			Fix:  "run it in a terminal, or use --raw to open the file in $EDITOR",
		}
	}
	start := tui.Start{Kind: tui.StartEdit, Preset: preset}
	if len(agent) > 0 {
		start.Agent = agent[0]
	}
	return a.openInterface(start)
}

// runEditor opens a file in the user's editor and waits.
func (a *app) runEditor(path string) error {
	cmd := editor.Command(path)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = a.io.In, a.io.Out, a.io.Err
	if err := cmd.Run(); err != nil {
		return &ExitError{Code: 1, Err: fmt.Errorf("the editor %q failed: %w", cmd.Args[0], err), Fix: "set $EDITOR to an editor that works in this terminal"}
	}
	return nil
}

// setPromptFromFile replaces the prompt of an agent with the text of a file.
func (a *app) setPromptFromFile(preset string, agent []string, path string) error {
	if len(agent) != 1 {
		return &ExitError{Code: 2, Err: errors.New("--prompt-file needs the agent whose prompt it replaces"), Fix: "use `squad edit <preset> <agent> --prompt-file <file>`"}
	}
	text, err := workspace.ReadPromptFile(path)
	if err != nil {
		return &ExitError{Code: 1, Err: err, Fix: "give the path of a Markdown file with the prompt in it"}
	}
	if err := a.workspace().SetPrompt(preset, agent[0], text); err != nil {
		if errors.Is(err, workspace.ErrNoAgent) {
			return &ExitError{Code: 1, Err: err, Fix: "`squad lint " + preset + "` shows the agents of the preset"}
		}
		return libraryError(err, cloneInitialFix)
	}
	printf(a.io.Out, "The prompt of %s is now the text of %s (%s).\n", agent[0], filepath.Base(path), plural(strings.Count(text, "\n"), "line"))
	return nil
}

func (a *app) setIndependent(preset string, agent []string, independent bool) error {
	if len(agent) != 1 {
		return &ExitError{Code: 2, Err: errors.New("--independent needs the agent it applies to"), Fix: "use `squad edit <preset> <agent> --independent`"}
	}
	if err := a.workspace().SetIndependent(preset, agent[0], independent); err != nil {
		if errors.Is(err, workspace.ErrNoAgent) {
			return &ExitError{Code: 1, Err: err, Fix: "`squad lint " + preset + "` shows the agents of the preset"}
		}
		return libraryError(err, cloneInitialFix)
	}
	if independent {
		printf(a.io.Out, "%s is independent: it does not load CLAUDE.md, it is told to judge by the task and the code alone, and the main agent is told to give it nothing else.\n", agent[0])
	} else {
		printf(a.io.Out, "%s is an ordinary agent again.\n", agent[0])
	}
	return nil
}

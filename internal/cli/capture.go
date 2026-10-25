package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/EdrenaVosh/squad-up/internal/tui"
	"github.com/EdrenaVosh/squad-up/internal/workspace"
)

func (a *app) newCapture() *cobra.Command {
	return &cobra.Command{
		Use:   "capture <name>",
		Short: "Save the current Claude Code setup as a new preset",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return a.capturePreset(args[0])
		},
	}
}

// capturePreset stores the live Claude Code setup as a preset.
func (a *app) capturePreset(name string) error {
	if err := workspace.CheckName(name); err != nil {
		return nameError(err)
	}
	store := a.workspace().Store()
	if store.Exists(name) {
		return &ExitError{Code: 1, Err: fmt.Errorf("preset %q already exists", name), Fix: "choose another name"}
	}
	skipped, err := a.workspace().Capture(name)
	if err != nil {
		return &ExitError{Code: 1, Err: err, Fix: "repair the file named above and run the command again"}
	}
	bundle, err := store.Load(name)
	if err != nil {
		return &ExitError{Code: 1, Err: err, Fix: "check the permissions of " + a.paths.PresetsDir()}
	}

	a.reportSkipped(skipped)
	summary := tui.AgentCount(len(bundle.Agents))
	if len(bundle.Agents) > 0 {
		summary += " (" + strings.Join(bundle.Preset.Agents, " · ") + ")"
	}
	printf(a.io.Out, "Captured preset %q: %s\nNext: squad use %s\n", name, summary, name)
	return nil
}

package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/EdrenaVosh/squadup/internal/preset"
	"github.com/EdrenaVosh/squadup/internal/tui"
)

func (a *app) newCapture() *cobra.Command {
	return &cobra.Command{
		Use:   "capture <name>",
		Short: "Save the current Claude Code setup as a new preset",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			name := args[0]
			if name == "initial" {
				return &ExitError{Code: 2, Err: errors.New(`"initial" is reserved for your original setup`), Fix: "pick another preset name"}
			}
			if !preset.ValidName(name) {
				return &ExitError{
					Code: 2,
					Err:  fmt.Errorf("%q is not a valid preset name", name),
					Fix:  "use lowercase letters, digits and dashes, starting with a letter or digit (up to 64 characters)",
				}
			}

			store := preset.Store{Dir: a.paths.PresetsDir()}
			if store.Exists(name) {
				return &ExitError{Code: 1, Err: fmt.Errorf("preset %q already exists", name), Fix: "choose another name"}
			}
			bundle, skipped, err := preset.FromConfig(a.paths, name)
			if err != nil {
				return &ExitError{Code: 1, Err: err, Fix: "repair the file named above and run the command again"}
			}
			if err := store.Save(bundle); err != nil {
				return &ExitError{Code: 1, Err: err, Fix: "check the permissions of " + a.paths.PresetsDir()}
			}

			for _, s := range skipped {
				printf(a.io.Err, "Skipped %s: %s\n", s.Path, s.Reason)
			}
			summary := tui.AgentCount(len(bundle.Agents))
			if len(bundle.Agents) > 0 {
				summary += " (" + strings.Join(bundle.Preset.Agents, " · ") + ")"
			}
			printf(a.io.Out, "Captured preset %q: %s\nNext: squad use %s\n", name, summary, name)
			return nil
		},
	}
}

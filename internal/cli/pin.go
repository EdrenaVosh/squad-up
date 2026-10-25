package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/EdrenaVosh/squad-up/internal/preset"
	"github.com/EdrenaVosh/squad-up/internal/project"
)

// projectPin returns the preset pinned by the nearest .squad-up file above the
// current folder, or nil.
func (a *app) projectPin() (*project.Pin, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, nil //nolint:nilerr // without a working directory there is no project
	}
	pin, err := project.Find(dir)
	if err != nil {
		return nil, &ExitError{Code: 1, Err: err, Fix: "put one preset name in the file, or delete it"}
	}
	return pin, nil
}

func (a *app) newPin() *cobra.Command {
	var clear bool
	cmd := &cobra.Command{
		Use:   "pin <preset>",
		Short: "Pin a preset to this project (writes a .squad-up file); `squad run` then needs no name",
		Long: "Writes the preset's name into .squad-up in the current folder. Commit it to share the choice with your team:\n" +
			"the file holds only a name, and each person's own library decides what it means.\n" +
			"`squad run` with no name uses the nearest .squad-up above the current folder, and `squad status` says when it differs from what is equipped.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			dir, err := os.Getwd()
			if err != nil {
				return &ExitError{Code: 1, Err: err, Fix: "run it from an existing folder"}
			}
			if clear {
				removed, err := project.Remove(dir)
				if err != nil {
					return &ExitError{Code: 1, Err: err, Fix: "check the permissions of " + dir}
				}
				if removed {
					printf(a.io.Out, "Removed %s.\n", project.FileName)
				} else {
					printf(a.io.Out, "This folder has no %s: nothing to remove.\n", project.FileName)
				}
				return nil
			}
			if len(args) != 1 {
				return &ExitError{Code: 2, Err: errors.New("which preset should be pinned?"), Fix: "use `squad pin <preset>`; `squad list` shows the presets"}
			}
			name := args[0]
			if name == "initial" {
				return &ExitError{Code: 1, Err: errors.New(`"initial" is your own setup and cannot be pinned`), Fix: "pin one of your presets"}
			}
			if !a.workspace().Store().Exists(name) {
				return libraryError(fmt.Errorf("%w: %s", preset.ErrNotFound, name), "")
			}
			if err := project.Write(dir, name); err != nil {
				return &ExitError{Code: 1, Err: err, Fix: "check the permissions of " + dir}
			}
			printf(a.io.Out, "Pinned %q to this folder (%s).\nRun it with `squad run`, or equip it with `squad use %s`.\n", name, project.FileName, name)
			return nil
		},
	}
	cmd.Flags().BoolVar(&clear, "clear", false, "remove the .squad-up file of this folder")
	return cmd
}

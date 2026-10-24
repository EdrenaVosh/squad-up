package cli

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/EdrenaVosh/squadup/internal/team"
)

func (a *app) newNotes() *cobra.Command {
	var clear bool
	cmd := &cobra.Command{
		Use:   "notes",
		Short: "Show or clear the squad's shared notes of this project",
		Long: "The agents of a squad share a tiny notes file, " + team.NotesFile + ", in the project they work in:\n" +
			"decisions and why, what was tried and ruled out, and the defects still open (lines that start with !).\n" +
			"Without flags the notes are printed; --clear deletes them, so the next task starts fresh.",
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			dir, err := os.Getwd()
			if err != nil {
				return &ExitError{Code: 1, Err: err, Fix: "run it from an existing folder"}
			}
			path := filepath.Join(dir, filepath.FromSlash(team.NotesFile))
			data, err := os.ReadFile(path)
			missing := errors.Is(err, fs.ErrNotExist)
			if err != nil && !missing {
				return &ExitError{Code: 1, Err: err, Fix: "check the permissions of " + path}
			}
			if clear {
				if missing {
					printf(a.io.Out, "No notes here: nothing to clear.\n")
					return nil
				}
				if err := os.Remove(path); err != nil {
					return &ExitError{Code: 1, Err: err, Fix: "check the permissions of " + path}
				}
				_ = os.Remove(filepath.Dir(path)) // only when it is empty
				printf(a.io.Out, "Cleared %s.\n", team.NotesFile)
				return nil
			}
			if missing {
				printf(a.io.Out, "No notes here yet. The agents of a squad create %s when they work in a project.\n", team.NotesFile)
				return nil
			}
			text := strings.TrimRight(string(data), "\n")
			printf(a.io.Out, "%s\n", text)
			open := 0
			for _, l := range strings.Split(text, "\n") {
				if strings.HasPrefix(l, "!") {
					open++
				}
			}
			printf(a.io.Err, "%s · about %d tokens · %d open\n", plural(strings.Count(text, "\n")+1, "line"), (len(text)+3)/4, open)
			return nil
		},
	}
	cmd.Flags().BoolVar(&clear, "clear", false, "delete the notes so the next task starts fresh")
	return cmd
}

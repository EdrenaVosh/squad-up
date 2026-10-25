package cli

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/EdrenaVosh/squad-up/internal/tui"
)

type logEntry struct {
	ID        string    `json:"id"`
	Time      time.Time `json:"time"`
	Command   string    `json:"command"`
	Preset    string    `json:"preset"`
	Available bool      `json:"backupAvailable"`
}

func (a *app) newLog() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "log",
		Short: "List the operations that changed your config, newest first",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			engine := a.engine()
			journal, err := engine.Journal()
			if err != nil {
				return &ExitError{Code: 1, Err: err, Fix: "fix or delete " + a.paths.LogFile() + " (backups are not affected)"}
			}
			backups, err := engine.Backups()
			if err != nil {
				return &ExitError{Code: 1, Err: err, Fix: "check the permissions of " + a.paths.BackupsDir()}
			}
			available := map[string]bool{}
			for _, b := range backups {
				available[b.ID] = true
			}

			entries := make([]logEntry, 0, len(journal))
			for i := len(journal) - 1; i >= 0; i-- {
				j := journal[i]
				entries = append(entries, logEntry{ID: j.Backup, Time: j.Time, Command: j.Command, Preset: j.Preset, Available: available[j.Backup]})
			}

			if asJSON {
				return writeJSON(a.io.Out, entries)
			}
			if len(entries) == 0 {
				printf(a.io.Out, "%s\n", tui.NoLogYet)
				return nil
			}
			width := len("ID")
			for _, e := range entries {
				width = max(width, len(e.ID))
			}
			printf(a.io.Out, "%-*s  %-19s  %-8s  %s\n", width, "ID", "WHEN", "COMMAND", "PRESET")
			for _, e := range entries {
				note := ""
				if !e.Available {
					note = "  (backup removed)"
				}
				printf(a.io.Out, "%-*s  %-19s  %-8s  %s%s\n", width, e.ID, e.Time.UTC().Format("2006-01-02 15:04:05"), e.Command, e.Preset, note)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print machine-readable output")
	return cmd
}

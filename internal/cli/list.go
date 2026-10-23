package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/EdrenaVosh/squadup/internal/tui"
	"github.com/EdrenaVosh/squadup/internal/workspace"
)

func (a *app) newList() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List presets; the active one is marked",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			entries, err := a.workspace().Entries()
			if err != nil {
				return &ExitError{Code: 1, Err: err, Fix: "fix or delete the file named above; files squad wrote stay in place"}
			}
			if asJSON {
				return writeJSON(a.io.Out, entries)
			}
			renderList(a, entries)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print machine-readable output")
	return cmd
}

func renderList(a *app, entries []workspace.Entry) {
	width := 0
	for _, e := range entries {
		width = max(width, len(e.Name))
	}
	line := func(e workspace.Entry) {
		marker := "○"
		switch {
		case e.Active:
			marker = "●"
		case e.Initial:
			marker = "◇"
		}
		var detail string
		switch {
		case e.Error != "":
			detail = "unreadable — " + e.Error
		case e.Initial:
			detail = tui.InitialLabel(e.CapturedAt.UTC().Format("2006-01-02"))
		default:
			detail = strings.Join(nonEmpty(e.Title, tui.AgentCount(e.Agents)), " · ")
		}
		printf(a.io.Out, "  %s %-*s  %s\n", marker, width, e.Name, detail)
	}

	for _, e := range entries {
		if !e.Initial {
			line(e)
		}
	}
	if len(entries) == 1 {
		printf(a.io.Out, "  %s\n", tui.NoPresetsYet)
	}
	printf(a.io.Out, "  %s\n", tui.ListDivider)
	line(entries[len(entries)-1])
}

func writeJSON(w io.Writer, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encode JSON: %w", err)
	}
	_, err = w.Write(append(data, '\n'))
	return err
}

func nonEmpty(items ...string) []string {
	return slices.DeleteFunc(items, func(s string) bool { return s == "" })
}

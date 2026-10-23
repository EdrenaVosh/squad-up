package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/EdrenaVosh/squadup/internal/apply"
	"github.com/EdrenaVosh/squadup/internal/initial"
	"github.com/EdrenaVosh/squadup/internal/preset"
	"github.com/EdrenaVosh/squadup/internal/tui"
)

type listEntry struct {
	Name        string     `json:"name"`
	Title       string     `json:"title,omitempty"`
	Description string     `json:"description,omitempty"`
	Agents      int        `json:"agents"`
	Active      bool       `json:"active"`
	Initial     bool       `json:"initial"`
	CapturedAt  *time.Time `json:"capturedAt,omitempty"`
	Error       string     `json:"error,omitempty"`
}

func (a *app) newList() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List presets; the active one is marked",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			entries, err := a.listEntries()
			if err != nil {
				return err
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

// listEntries returns the stored presets alphabetically, then Initial.
func (a *app) listEntries() ([]listEntry, error) {
	state, err := apply.LoadState(a.paths)
	if err != nil {
		return nil, &ExitError{Code: 1, Err: err, Fix: "fix or delete the state file; files squad wrote stay in place"}
	}
	meta, err := initial.Load(a.paths)
	if err != nil {
		return nil, &ExitError{Code: 1, Err: err, Fix: "run `squad initial --recapture` once your config is in its original state"}
	}
	active := state.ActivePreset()

	store := preset.Store{Dir: a.paths.PresetsDir()}
	names, err := store.Names()
	if err != nil {
		return nil, &ExitError{Code: 1, Err: err, Fix: "check the permissions of " + a.paths.PresetsDir()}
	}

	var entries []listEntry
	for _, name := range names {
		if name == "initial" {
			continue
		}
		e := listEntry{Name: name, Active: name == active}
		if p, err := store.LoadPreset(name); err != nil {
			e.Error = firstLine(err.Error())
		} else {
			e.Title, e.Description, e.Agents = p.Title, p.Description, len(p.Agents)
		}
		entries = append(entries, e)
	}
	captured := meta.CapturedAt
	entries = append(entries, listEntry{
		Name:        "initial",
		Title:       "Initial",
		Description: "Your original Claude Code setup",
		Agents:      len(meta.Summary().AgentNames),
		Active:      active == "initial",
		Initial:     true,
		CapturedAt:  &captured,
	})
	return entries, nil
}

func renderList(a *app, entries []listEntry) {
	width := 0
	for _, e := range entries {
		width = max(width, len(e.Name))
	}
	line := func(e listEntry) {
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

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

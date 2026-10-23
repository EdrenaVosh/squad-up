package cli

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/EdrenaVosh/squadup/internal/apply"
	"github.com/EdrenaVosh/squadup/internal/claude"
	"github.com/EdrenaVosh/squadup/internal/tui"
)

const statusTimeout = 15 * time.Second

type driftJSON struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
}

type claudeJSON struct {
	Found    bool   `json:"found"`
	Version  string `json:"version,omitempty"`
	Provider string `json:"provider,omitempty"`
	LoggedIn bool   `json:"loggedIn"`
	Error    string `json:"error,omitempty"`
}

type statusJSON struct {
	Active    string      `json:"active"`
	Drift     []driftJSON `json:"drift"`
	Claude    claudeJSON  `json:"claude"`
	ClaudeDir string      `json:"claudeDir"`
	Home      string      `json:"home"`
	Warnings  []string    `json:"warnings"`
}

func (a *app) newStatus() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show the active preset, manual edits and the Claude Code in use",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), statusTimeout)
			defer cancel()

			state, err := apply.LoadState(a.paths)
			if err != nil {
				return &ExitError{Code: 1, Err: err, Fix: "fix or delete the state file; files squad wrote stay in place"}
			}
			drift, err := apply.Drift(a.paths, state)
			if err != nil {
				return &ExitError{Code: 1, Err: err, Fix: "check the permissions of " + a.paths.ClaudeDir}
			}

			st := statusJSON{
				Active:    state.ActivePreset(),
				Drift:     []driftJSON{},
				Claude:    gatherClaude(ctx),
				ClaudeDir: a.paths.ClaudeDir,
				Home:      a.paths.Home,
				Warnings:  []string{},
			}
			for _, d := range drift {
				st.Drift = append(st.Drift, driftJSON{Path: d.Path, Kind: string(d.Kind)})
			}
			if model := os.Getenv("ANTHROPIC_MODEL"); model != "" {
				st.Warnings = append(st.Warnings, tui.ModelOverride(model))
			}

			if asJSON {
				return writeJSON(a.io.Out, st)
			}
			renderStatus(a, st)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print machine-readable output")
	return cmd
}

// gatherClaude asks the claude binary about itself. Failures are part of the
// report, not errors: status must work when claude is missing.
func gatherClaude(ctx context.Context) claudeJSON {
	bin, err := claude.Find()
	if err != nil {
		return claudeJSON{}
	}
	c := claude.Client{Bin: bin}
	info := claudeJSON{Found: true}
	if info.Version, err = c.Version(ctx); err != nil {
		info.Error = err.Error()
		return info
	}
	if auth, err := c.Auth(ctx); err != nil {
		info.Error = err.Error()
	} else {
		info.Provider, info.LoggedIn = auth.APIProvider, auth.LoggedIn
	}
	return info
}

func renderStatus(a *app, st statusJSON) {
	row := func(label, value string) { printf(a.io.Out, "%-14s %s\n", label, value) }

	active := st.Active
	if active == "initial" {
		active += " · your original setup"
	}
	row("Active preset", active)

	if len(st.Drift) == 0 {
		row("Drift", tui.NoDrift)
	} else {
		row("Drift", tui.ManualEdits)
		for _, d := range st.Drift {
			printf(a.io.Out, "%-14s   %s (%s)\n", "", d.Path, d.Kind)
		}
	}

	switch {
	case !st.Claude.Found:
		row("Claude", tui.ClaudeNotFound)
	case st.Claude.Error != "":
		row("Claude", tui.ClaudeUnavailable)
	default:
		parts := []string{st.Claude.Version, st.Claude.Provider}
		if !st.Claude.LoggedIn {
			parts = append(parts, "not logged in")
		}
		row("Claude", strings.Join(nonEmpty(parts...), " · "))
	}
	row("Config dir", st.ClaudeDir)
	row("Squadup home", st.Home)

	for _, w := range st.Warnings {
		printf(a.io.Out, "\n! %s\n", w)
	}
}

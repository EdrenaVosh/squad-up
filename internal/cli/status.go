package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/EdrenaVosh/squadup/internal/apply"
	"github.com/EdrenaVosh/squadup/internal/claude"
	"github.com/EdrenaVosh/squadup/internal/models"
	"github.com/EdrenaVosh/squadup/internal/tui"
	"github.com/EdrenaVosh/squadup/internal/workspace"
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

	authMethod string
}

type modelsJSON struct {
	Probed      bool       `json:"probed"`
	ProbedAt    *time.Time `json:"probedAt,omitempty"`
	Verified    []string   `json:"verified"`
	Unavailable []string   `json:"unavailable"`
}

type statusJSON struct {
	Active    string      `json:"active"`
	Drift     []driftJSON `json:"drift"`
	Claude    claudeJSON  `json:"claude"`
	Models    modelsJSON  `json:"models"`
	ClaudeDir string      `json:"claudeDir"`
	Home      string      `json:"home"`
	Warnings  []string    `json:"warnings"`
	Pinned    *pinnedJSON `json:"pinned,omitempty"`
}

// pinnedJSON is the preset the project pins with a .squadup file.
type pinnedJSON struct {
	Preset  string `json:"preset"`
	File    string `json:"file"`
	Matches bool   `json:"matches"` // whether it is the equipped preset
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
			st.Models = a.modelsSummary(st.Claude)
			if model := os.Getenv("ANTHROPIC_MODEL"); model != "" {
				st.Warnings = append(st.Warnings, tui.ModelOverride(model))
			}

			pin, err := a.projectPin()
			if err != nil {
				st.Warnings = append(st.Warnings, err.Error())
			}
			if pin != nil {
				st.Pinned = &pinnedJSON{Preset: pin.Preset, File: pin.File, Matches: pin.Preset == st.Active}
				if !st.Pinned.Matches {
					equipped := "nothing is equipped"
					if st.Active != "" {
						equipped = st.Active + " is equipped"
					}
					st.Warnings = append(st.Warnings, fmt.Sprintf("This project pins %s (%s), but %s. Run `squad use %s`.", pin.Preset, pin.File, equipped, pin.Preset))
				}
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
		info.Provider, info.LoggedIn, info.authMethod = auth.APIProvider, auth.LoggedIn, auth.AuthMethod
	}
	return info
}

func renderStatus(a *app, st statusJSON) {
	row := func(label, value string) { printf(a.io.Out, "%-14s %s\n", label, value) }

	active := st.Active
	switch {
	case active == "initial":
		active += " · your original setup"
	case !a.workspace().Store().Exists(active):
		active += " (preset removed)"
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
	if st.Pinned != nil && st.Pinned.Matches {
		row("Pinned", st.Pinned.Preset+" · "+st.Pinned.File)
	}
	row("Models", modelsRow(st.Models))
	row("Config dir", st.ClaudeDir)
	row("Squadup home", st.Home)

	for _, w := range st.Warnings {
		printf(a.io.Out, "\n! %s\n", w)
	}
}

// modelsSummary reports the models that a saved probe and the policy settle.
func (a *app) modelsSummary(c claudeJSON) modelsJSON {
	summary := modelsJSON{Verified: []string{}, Unavailable: []string{}}
	if !c.Found || c.Error != "" {
		return summary
	}
	cache, err := models.LoadCache(a.paths)
	if err != nil || !cache.Valid(c.Version, c.authMethod, a.now()) {
		return summary
	}
	policy, _ := a.workspace().Policy()
	summary.Probed = true
	at := cache.ProbedAt
	summary.ProbedAt = &at
	summary.Verified, summary.Unavailable = workspace.VerifiedAndUnavailable(models.Evaluate(models.Catalog(), policy, cache))
	return summary
}

func modelsRow(m modelsJSON) string {
	if !m.Probed {
		return tui.NotProbed
	}
	var parts []string
	if len(m.Verified) > 0 {
		parts = append(parts, "verified: "+strings.Join(m.Verified, ", "))
	}
	if len(m.Unavailable) > 0 {
		parts = append(parts, "unavailable: "+strings.Join(m.Unavailable, ", "))
	}
	if len(parts) == 0 {
		return "probed, nothing verified"
	}
	return strings.Join(parts, " · ")
}

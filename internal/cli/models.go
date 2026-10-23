package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/EdrenaVosh/squadup/internal/claude"
	"github.com/EdrenaVosh/squadup/internal/models"
	"github.com/EdrenaVosh/squadup/internal/tui"
	"github.com/EdrenaVosh/squadup/internal/workspace"
)

const (
	probeTimeout   = 30 * time.Second
	modelsDeadline = 2 * time.Minute
)

type modelRow struct {
	Name       string   `json:"name"`
	UsedBy     []string `json:"usedBy"`
	Status     string   `json:"status"`
	ResolvedTo string   `json:"resolvedTo,omitempty"`
	Reason     string   `json:"reason,omitempty"`
}

func (a *app) newModels() *cobra.Command {
	var probe, asJSON bool
	cmd := &cobra.Command{
		Use:   "models",
		Short: "Show which models you can use, and optionally test them",
		Long: "Claude Code has no command that lists the models your account can use, so squad combines a built-in catalogue,\n" +
			"the availableModels policy from your settings and, with --probe, one tiny request per model alias.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), modelsDeadline)
			defer cancel()
			return a.runModels(ctx, probe, asJSON)
		},
	}
	cmd.Flags().BoolVar(&probe, "probe", false, "send one tiny request per model alias to find out what works (uses a few tokens)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "print machine-readable output")
	return cmd
}

func (a *app) runModels(ctx context.Context, probe, asJSON bool) error {
	policy, notes := a.workspace().Policy()

	var (
		client   claude.Client
		found    bool
		version  string
		auth     claude.AuthStatus
		haveAuth bool
	)
	if bin, err := claude.Find(); err == nil {
		client, found = claude.Client{Bin: bin}, true
		if v, err := client.Version(ctx); err == nil {
			version = v
			if s, err := client.Auth(ctx); err == nil {
				auth, haveAuth = s, true
			}
		}
	}

	cache, err := models.LoadCache(a.paths)
	if err != nil {
		return &ExitError{Code: 1, Err: err, Fix: "check the permissions of " + a.paths.CacheDir()}
	}
	if cache != nil && haveAuth && !cache.Valid(version, auth.AuthMethod, a.now()) {
		notes = append(notes, "The saved probe is out of date (claude was updated, the account changed, or it is older than a day); run `squad models --probe` to refresh it.")
		cache = nil
	}
	if cache != nil && !haveAuth {
		cache = nil
	}

	if probe {
		printf(a.io.Err, "Probing %s with one tiny request each…\n", strings.Join(models.ProbeAliases, ", "))
		probed, err := a.workspace().ProbeModels(ctx)
		switch {
		case errors.Is(err, workspace.ErrNoClaude):
			return &ExitError{Code: 1, Err: errors.New("cannot probe: claude was not found"), Fix: "install Claude Code (https://code.claude.com/docs) and check it with `squad doctor`"}
		case errors.Is(err, workspace.ErrNotLoggedIn):
			return &ExitError{
				Code: 1,
				Err:  errors.New("cannot probe: claude is not logged in"),
				Fix:  "run `claude auth login` (login is stored per config directory, so a custom CLAUDE_CONFIG_DIR needs its own login)",
			}
		case err != nil:
			return &ExitError{Code: 1, Err: err, Fix: "run `squad doctor` to check claude"}
		}
		cache = probed
	}

	results := models.Evaluate(models.Catalog(), policy, cache)
	rows := make([]modelRow, len(results))
	for i, r := range results {
		var usedBy []string
		if r.Agents {
			usedBy = append(usedBy, "agents")
		}
		if r.Settings {
			usedBy = append(usedBy, "settings")
		}
		rows[i] = modelRow{Name: r.Name, UsedBy: usedBy, Status: string(r.Status), ResolvedTo: r.ResolvedTo, Reason: r.Reason}
	}

	switch {
	case !found:
		notes = append(notes, "claude was not found, so nothing could be checked against your account; run `squad doctor`.")
	case haveAuth && !auth.LoggedIn:
		notes = append(notes, "claude is not logged in, so nothing could be checked against your account; run `claude auth login`.")
	case haveAuth && auth.APIProvider != "" && auth.APIProvider != "firstParty":
		notes = append(notes, fmt.Sprintf("claude uses %s, where aliases resolve to other model versions than on the Anthropic API.", auth.APIProvider))
	}
	if m := os.Getenv("ANTHROPIC_MODEL"); m != "" {
		notes = append(notes, tui.ModelOverride(m))
	}
	if cache == nil && !probe && found {
		notes = append(notes, "Nothing has been probed yet. `squad models --probe` finds out which models your account can use (four tiny requests).")
	}
	for _, n := range notes {
		printf(a.io.Err, "%s\n", n)
	}

	if asJSON {
		return writeJSON(a.io.Out, rows)
	}
	renderModels(a, rows)
	if cache != nil {
		printf(a.io.Out, "\nProbed %s with %s.\n", age(a.now(), cache.ProbedAt), cache.ClaudeVersion)
	}
	return nil
}

func renderModels(a *app, rows []modelRow) {
	nameW, forW, statusW, idW := len("MODEL"), len("USED BY"), len("STATUS"), len("RESOLVES TO")
	for _, r := range rows {
		nameW = max(nameW, len(r.Name))
		forW = max(forW, len(strings.Join(r.UsedBy, ", ")))
		statusW = max(statusW, len(r.Status))
		idW = max(idW, len(r.ResolvedTo))
	}
	line := func(name, usedBy, status, id, note string) {
		text := fmt.Sprintf("%-*s  %-*s  %-*s  %-*s  %s", nameW, name, forW, usedBy, statusW, status, idW, id, note)
		printf(a.io.Out, "%s\n", strings.TrimRight(text, " "))
	}
	line("MODEL", "USED BY", "STATUS", "RESOLVES TO", "NOTE")
	for _, r := range rows {
		line(r.Name, strings.Join(r.UsedBy, ", "), r.Status, r.ResolvedTo, r.Reason)
	}
}

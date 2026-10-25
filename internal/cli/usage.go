package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/EdrenaVosh/squad-up/internal/tui"
	"github.com/EdrenaVosh/squad-up/internal/usage"
)

type tokensJSON struct {
	Input      int `json:"input"`
	CacheWrite int `json:"cacheWrite"`
	CacheRead  int `json:"cacheRead"`
	Output     int `json:"output"`
	Messages   int `json:"messages"`
	Total      int `json:"total"` // input + cacheWrite + output, as Claude Code counts a run
}

type agentUsageJSON struct {
	Name    string `json:"name"`
	Runs    int    `json:"runs"`
	Running int    `json:"running"`
	Model   string `json:"model,omitempty"`
	tokensJSON
}

type usageJSON struct {
	Found   bool             `json:"found"`
	Project string           `json:"project"`
	Session string           `json:"session,omitempty"`
	Agents  []agentUsageJSON `json:"agents"`
	Total   tokensJSON       `json:"total"`
}

func toTokensJSON(t usage.Tokens) tokensJSON {
	return tokensJSON{Input: t.Input, CacheWrite: t.CacheWrite, CacheRead: t.CacheRead, Output: t.Output, Messages: t.Calls, Total: t.Total()}
}

func toUsageJSON(r usage.Report) usageJSON {
	out := usageJSON{Found: r.Found, Project: r.Project, Session: r.Session, Agents: []agentUsageJSON{}, Total: toTokensJSON(r.Total)}
	for _, a := range r.Agents {
		out.Agents = append(out.Agents, agentUsageJSON{Name: a.Name, Runs: a.Runs, Running: a.Running, Model: a.Model, tokensJSON: toTokensJSON(a.Tokens)})
	}
	return out
}

func (a *app) newUsage() *cobra.Command {
	var watch, asJSON, list bool
	var session, dir string
	cmd := &cobra.Command{
		Use:   "usage",
		Short: "Show the tokens each agent of a Claude Code session has spent",
		Long: "Reads the session's own transcripts, so the numbers are what was really sent, not an estimate:\n" +
			"the main session and every subagent run, per agent. By default it reports the newest session of this folder.\n" +
			"With --watch the live report opens in the terminal interface and follows the session as it works.\n\n" +
			"SENT is fresh input plus what was written to the prompt cache, CACHED what was read back from it,\n" +
			"TOTAL is SENT plus OUT, the way Claude Code counts a run (cache reads are left out).",
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if dir == "" {
				wd, err := os.Getwd()
				if err != nil {
					return &ExitError{Code: 1, Err: err, Fix: "run it from an existing folder"}
				}
				dir = wd
			}
			if list {
				return a.listSessions(dir, asJSON)
			}
			if watch {
				if !a.canOpenInterface() {
					return &ExitError{Code: 1, Err: errors.New("--watch needs a terminal"), Fix: "run it in a terminal, or leave out --watch for a one-time report"}
				}
				return a.openInterface(tui.Start{Kind: tui.StartUsage, Cwd: dir})
			}
			if session == "" {
				latest, ok := usage.LatestSession(a.paths.ClaudeDir, dir)
				if !ok {
					if asJSON {
						return writeJSON(a.io.Out, usageJSON{Project: usage.ProjectDir(a.paths.ClaudeDir, dir), Agents: []agentUsageJSON{}})
					}
					printf(a.io.Out, "No Claude Code session in %s yet. Start `squad run <preset>` (or claude) here.\n", dir)
					return nil
				}
				session = latest
			}
			report, err := usage.NewWatcher(usage.ProjectDir(a.paths.ClaudeDir, dir), session).Refresh()
			if err != nil {
				return &ExitError{Code: 1, Err: err, Fix: "check the permissions of " + a.paths.ClaudeDir}
			}
			if asJSON {
				return writeJSON(a.io.Out, toUsageJSON(report))
			}
			if !report.Found {
				printf(a.io.Out, "The session %s has no transcript in %s.\n", session, report.Project)
				return nil
			}
			printf(a.io.Out, "Session %s in %s\n\n", session, dir)
			printUsageTable(a, a.io.Out, report)
			return nil
		},
	}
	cmd.Flags().BoolVar(&watch, "watch", false, "open the live report in the terminal interface")
	cmd.Flags().BoolVar(&list, "list", false, "list the sessions of the folder with their ids, to pick one for --session")
	cmd.Flags().BoolVar(&asJSON, "json", false, "print machine-readable output")
	cmd.Flags().StringVar(&session, "session", "", "the session to report, instead of the newest")
	cmd.Flags().StringVar(&dir, "dir", "", "the folder whose sessions to read, instead of this one")
	return cmd
}

// printUsageTable writes the report as a table of plain text.
func printUsageTable(a *app, w interface{ Write([]byte) (int, error) }, r usage.Report) {
	row := func(name string, runs, running string, t usage.Tokens, share string) {
		printf(w, "%-16s %5s %-9s %8s %8s %8s %9s  %s\n", name, runs, running,
			usage.Short(t.Input+t.CacheWrite), usage.Short(t.CacheRead), usage.Short(t.Output), usage.Short(t.Total()), share)
	}
	printf(w, "%-16s %5s %-9s %8s %8s %8s %9s  %s\n", "AGENT", "RUNS", "STATE", "SENT", "CACHED", "OUT", "TOTAL", "SHARE")
	total := max(r.Total.Total(), 1)
	for _, ag := range r.Agents {
		runs, state := "—", "idle"
		if ag.Name != usage.MainName {
			runs, state = fmt.Sprintf("%d", ag.Runs), "done"
		}
		if ag.Running > 0 {
			state = "working"
		}
		pct := ag.Tokens.Total() * 100 / total
		row(ag.Name, runs, state, ag.Tokens, fmt.Sprintf("%s %d%%", strings.Repeat("█", pct/10)+strings.Repeat("░", 10-pct/10), pct))
	}
	row("SQUAD", "", "", r.Total, "")
	printf(w, "\nTOTAL is SENT + OUT; CACHED (cache reads) is left out, as Claude Code does.\n")
}

// printRunUsage reports what a `squad run` session spent, once claude has
// exited. It prints nothing when no transcript of the run can be found.
func (a *app) printRunUsage(since time.Time) {
	dir, err := os.Getwd()
	if err != nil {
		return
	}
	session, ok := usage.SessionSince(a.paths.ClaudeDir, dir, since)
	if !ok {
		return
	}
	report, err := usage.NewWatcher(usage.ProjectDir(a.paths.ClaudeDir, dir), session).Refresh()
	if err != nil || !report.Found || report.Total.Total() == 0 {
		return
	}
	printf(a.io.Err, "\nTokens this session (squad usage --session %s):\n", session)
	printUsageTable(a, a.io.Err, report)
}

type sessionJSON struct {
	ID      string    `json:"id"`
	Started time.Time `json:"started,omitzero"`
	Last    time.Time `json:"last"`
	Total   int       `json:"total"`
	Prompt  string    `json:"prompt,omitempty"`
}

// listSessions prints the newest sessions of the folder, with the id that
// --session takes. The interface does not show ids; this is where they live.
func (a *app) listSessions(dir string, asJSON bool) error {
	const limit = 30
	project := usage.ProjectDir(a.paths.ClaudeDir, dir)
	out := []sessionJSON{}
	for _, s := range usage.ListSessions(a.paths.ClaudeDir, dir, limit) {
		head := usage.ReadHead(filepath.Join(project, s.ID+".jsonl"))
		row := sessionJSON{ID: s.ID, Started: head.Started, Last: s.Last, Prompt: head.Prompt}
		if report, err := usage.NewWatcher(project, s.ID).Refresh(); err == nil {
			row.Total = report.Total.Total()
		}
		out = append(out, row)
	}
	if asJSON {
		return writeJSON(a.io.Out, out)
	}
	if len(out) == 0 {
		printf(a.io.Out, "No Claude Code session in %s yet.\n", dir)
		return nil
	}
	printf(a.io.Out, "%-36s  %-16s  %-16s  %8s  %s\n", "SESSION", "STARTED", "LAST ACTIVITY", "TOKENS", "FIRST PROMPT")
	for _, r := range out {
		started := "—"
		if !r.Started.IsZero() {
			started = r.Started.Local().Format("Jan 2 15:04")
		}
		printf(a.io.Out, "%-36s  %-16s  %-16s  %8s  %s\n", r.ID, started, r.Last.Local().Format("Jan 2 15:04"), usage.Short(r.Total), r.Prompt)
	}
	printf(a.io.Out, "\nSee one with: squad usage --session <id>\n")
	return nil
}

package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/EdrenaVosh/squadup/internal/initial"
	"github.com/EdrenaVosh/squadup/internal/tui"
)

// renderWelcome prints the first-run text: what was found and saved as Initial.
func renderWelcome(w io.Writer, meta *initial.Meta) {
	s := meta.Summary()
	printf(w, "%s\n", tui.WelcomeTitle)
	if s.Clean {
		printf(w, "%s\n\n", tui.CleanSetup)
	} else {
		printf(w, "%s\n%s\n\n", tui.FoundExisting, tui.SavedAsInitial)
		renderSetup(w, s)
		printf(w, "\n")
	}
	printf(w, "%s\n\n", tui.NothingChanged)
}

// renderInitial prints `squad initial`.
func renderInitial(w io.Writer, meta *initial.Meta) {
	s := meta.Summary()
	printf(w, "%s\n", tui.InitialHeader(meta.CapturedAt.UTC().Format("2006-01-02 15:04 MST")))
	if s.Clean {
		printf(w, "%s\n", tui.CleanSetup)
		return
	}
	renderSetup(w, s)
}

func renderSetup(w io.Writer, s initial.Summary) {
	agents := tui.Unset
	if n := len(s.AgentNames); n > 0 {
		agents = fmt.Sprintf("%d   %s", n, strings.Join(s.AgentNames, " · "))
	}
	row := func(label, value string) {
		if value == "" {
			value = tui.Unset
		}
		printf(w, "  %-13s%s\n", label, value)
	}
	row(tui.LabelAgents, agents)
	row(tui.LabelModel, s.Model)
	row(tui.LabelEffort, s.Effort)
	row(tui.LabelPermissions, s.Permissions)
	row(tui.LabelMainAgent, s.Main)

	agentIssues := s.Issues
	if s.SettingsInvalid {
		agentIssues--
	}
	if agentIssues > 0 {
		printf(w, "\n  %s\n", tui.AgentsHaveIssues(agentIssues))
	}
	if s.SettingsInvalid {
		printf(w, "\n  %s\n", tui.SettingsBroken)
	}
}

package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/EdrenaVosh/squadup/internal/initial"
	"github.com/EdrenaVosh/squadup/internal/workspace"
)

// probeDoneMsg carries the outcome of the model check started from the welcome
// screen.
type probeDoneMsg struct{ err error }

// welcomeScreen is the first thing a new user sees: what was found and saved as
// Initial, and what to do next.
type welcomeScreen struct {
	ws      workspace.Workspace
	meta    *initial.Meta
	cursor  int
	probing bool
}

func newWelcome(ws workspace.Workspace, meta *initial.Meta) *welcomeScreen {
	return &welcomeScreen{ws: ws, meta: meta}
}

func (w *welcomeScreen) Init() tea.Cmd { return nil }

// finished is true once there is a preset: the welcome has done its job.
func (w *welcomeScreen) finished() bool {
	entries, err := w.ws.Entries()
	return err == nil && len(entries) > 1
}

var welcomeChoices = []struct{ label, note string }{
	{"Start from my current setup", "an editable copy of what you have now"},
	{"Create a first preset", "blank, or from your current setup"},
	{"Check which models I can use", "four tiny requests to Claude"},
	{"Go to the main screen", "look around first"},
}

func (w *welcomeScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch m := msg.(type) {
	case probeDoneMsg:
		w.probing = false
		if m.err != nil {
			return w, flashErr(probeError(m.err))
		}
		return w, flash("Models checked: `squad models` shows the details")
	case tea.KeyPressMsg:
		if w.probing {
			return w, nil
		}
		switch m.String() {
		case "up", "k":
			w.cursor = max(w.cursor-1, 0)
		case "down", "j":
			w.cursor = min(w.cursor+1, len(welcomeChoices)-1)
		case "esc", "q":
			return w, pop()
		case "enter":
			return w, w.choose()
		}
	}
	return w, nil
}

func (w *welcomeScreen) choose() tea.Cmd {
	switch w.cursor {
	case 0:
		return push(w.cloneInitial())
	case 1:
		return push(newCreateFlow(w.ws, nil))
	case 2:
		body := []string{"This sends one tiny request for each of fable, opus,", "sonnet and haiku, and uses a few tokens of your plan."}
		return push(newConfirm("Check your models", body, "Check", false, func() tea.Cmd {
			w.probing = true
			return w.probe
		}))
	}
	return pop()
}

func (w *welcomeScreen) cloneInitial() screen {
	return newPrompt("Start from my current setup", "An editable copy; your original setup is never changed.", "my-setup", presetNameCheck(w.ws), func(name string) tea.Cmd {
		skipped, err := w.ws.Clone("initial", name)
		if err != nil {
			return flashErr(err.Error())
		}
		text := fmt.Sprintf("Created %q from your current setup", name)
		if len(skipped) > 0 {
			text += fmt.Sprintf(" (%d agent files could not be copied)", len(skipped))
		}
		return tea.Batch(push(newPresetScreen(w.ws, name)), flash(text))
	})
}

func (w *welcomeScreen) probe() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	_, err := w.ws.ProbeModels(ctx)
	return probeDoneMsg{err: err}
}

func probeError(err error) string {
	switch {
	case errors.Is(err, workspace.ErrNoClaude):
		return "claude was not found — install Claude Code, then run squad models --probe"
	case errors.Is(err, workspace.ErrNotLoggedIn):
		return "claude is not logged in — run `claude auth login`, then squad models --probe"
	}
	return err.Error()
}

func (w *welcomeScreen) Hints() []hint {
	return []hint{{"↑↓", "choose"}, {"enter", "go"}, {"esc", "skip"}}
}

func (w *welcomeScreen) View(c viewCtx) string {
	inner := c.w - 2
	s := w.meta.Summary()
	var lines []string
	add := func(l ...string) { lines = append(lines, l...) }

	add("")
	if s.Clean {
		add(" "+sBold.Render(CleanSetup), "")
	} else {
		add(" "+sBold.Render(FoundExisting), " "+SavedAsInitial, "")
		agents := Unset
		if n := len(s.AgentNames); n > 0 {
			agents = fmt.Sprintf("%d   %s", n, strings.Join(s.AgentNames, " · "))
		}
		for _, r := range [][2]string{
			{"Agents", agents}, {"Model", s.Model}, {"Effort", s.Effort}, {"Permissions", s.Permissions}, {"Main agent", s.Main},
		} {
			add(truncate("   "+sMuted.Render(pad(r[0], 13))+orDash(r[1], Unset), inner))
		}
		if s.Issues > 0 {
			add("", " "+sWarn.Render(truncate(issuesText(s), inner-2)))
		}
		add("")
	}
	add(" " + sMuted.Render(NothingChanged))
	add("")
	for i, ch := range welcomeChoices {
		mark := "  "
		label := ch.label
		if i == w.cursor {
			mark, label = sAccent.Render("▸ "), sBold.Render(ch.label)
		}
		add(truncate(" "+mark+pad(label, 32)+sMuted.Render(ch.note), inner))
	}
	if w.probing {
		add("", " "+sAccent.Render("Checking models…"))
	}
	return box("Welcome to squadup", "", lines, c.w, c.h, 0)
}

func issuesText(s initial.Summary) string {
	n := s.Issues
	if s.SettingsInvalid {
		n--
	}
	if n > 0 {
		return AgentsHaveIssues(n)
	}
	return SettingsBroken
}

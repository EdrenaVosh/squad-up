package tui

import (
	"errors"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/EdrenaVosh/squad-up/internal/initial"
	"github.com/EdrenaVosh/squad-up/internal/models"
	"github.com/EdrenaVosh/squad-up/internal/workspace"
)

// welcomeScreen is the first thing a new user sees: what was found and saved as
// Initial, and what to do next.
type welcomeScreen struct {
	ws     workspace.Workspace
	meta   *initial.Meta
	cursor int
	appear reveal // the logo drawing itself
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
	{"Browse starter templates", "Bug Hunter and Feature Creator"},
	{"Check which models I can use", "four tiny requests to Claude"},
	{"Go to the main screen", "look around first"},
}

func (w *welcomeScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch m := msg.(type) {
	case tea.KeyPressMsg:
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
		return push(browseTemplates(w.ws))
	case 3:
		return push(checkModelsConfirm(w.ws))
	}
	return pop()
}

func (w *welcomeScreen) cloneInitial() screen {
	return newNamePrompt(w.ws, "Start from my current setup", "An editable copy; your original setup is never changed. "+nameHelp, "my-setup", func(id, name string) tea.Cmd {
		skipped, err := w.ws.Clone("initial", id)
		if err == nil {
			err = withTitle(w.ws, id, name)
		}
		if err != nil {
			return flashErr(err.Error())
		}
		text := fmt.Sprintf("Created %q from your current setup", name)
		if len(skipped) > 0 {
			text += fmt.Sprintf(" (%d agent files could not be copied)", len(skipped))
		}
		return tea.Batch(push(newPresetScreen(w.ws, id)), flash(text))
	})
}

// checkModelsConfirm asks what to check before it spends anything: the four
// aliases, or every release as well. Each request carries Claude Code's normal
// start-up prompt, so it is not as small as it sounds.
func checkModelsConfirm(ws workspace.Workspace) screen {
	cost := "each request carries Claude Code's own start-up prompt, roughly 20k tokens, mostly cached"
	options := []option{
		{label: "The four latest", value: "aliases", note: fmt.Sprintf("fable, opus, sonnet and haiku: 4 requests; %s", cost)},
		{label: "Every release", value: "releases", note: fmt.Sprintf("also the older ones, such as sonnet 5 or opus 4.6, by their full ID: about %d requests; %s", len(models.Versions())-2, cost)},
	}
	return newChoice("Check your models", options, 0, func(o option) tea.Cmd {
		return push(newModelCheck(ws, o.value == "releases"))
	})
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

	if c.h >= 30 {
		tall := c.h >= 34 && c.w >= 100
		add(bigLogoBlock(inner, w.appear.frames(c), c.shift(), 0, tall, c.anim)...)
	} else {
		add(logoLines(inner, w.appear.frames(c))...)
	}
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
			mark, label = selBar()+" ", sBold.Render(ch.label)
		}
		add(truncate(" "+mark+pad(label, 32)+sMuted.Render(ch.note), inner))
	}
	return c.box("Welcome to squad-up", "", lines, 0)
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

package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/EdrenaVosh/squadup/internal/models"
	"github.com/EdrenaVosh/squadup/internal/workspace"
)

// The messages of a running check.
type (
	probeResultMsg struct {
		alias string
		res   models.ProbeResult
	}
	probeFinishedMsg struct{ err error }
)

// modelCheck asks Claude about each model, one tiny request each, and shows how
// every one of them is getting on. The requests run in parallel and take a few
// seconds, so the screen must say what is happening.
type modelCheck struct {
	ws      workspace.Workspace
	results map[string]models.ProbeResult
	done    bool
	err     error
	ch      chan tea.Msg
	cancel  context.CancelFunc
	ctx     context.Context
}

func newModelCheck(ws workspace.Workspace) *modelCheck {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	return &modelCheck{ws: ws, results: map[string]models.ProbeResult{}, ch: make(chan tea.Msg, len(models.ProbeAliases)+2), ctx: ctx, cancel: cancel}
}

// Init starts the requests and waits for the first answer.
func (m *modelCheck) Init() tea.Cmd {
	return func() tea.Msg {
		go func() {
			_, err := m.ws.ProbeModelsEach(m.ctx, func(alias string, res models.ProbeResult) {
				m.ch <- probeResultMsg{alias: alias, res: res}
			})
			m.ch <- probeFinishedMsg{err: err}
		}()
		return <-m.ch
	}
}

func (m *modelCheck) next() tea.Cmd { return func() tea.Msg { return <-m.ch } }

func (m *modelCheck) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch v := msg.(type) {
	case probeResultMsg:
		m.results[v.alias] = v.res
		return m, m.next()
	case probeFinishedMsg:
		m.done, m.err = true, v.err
		m.cancel()
		return m, reload() // the model lists read the new answers
	case tea.KeyPressMsg:
		if !m.done {
			if v.String() == "esc" || v.String() == "q" {
				m.cancel()
				return m, pop()
			}
			return m, nil
		}
		return m, pop()
	}
	return m, nil
}

func (m *modelCheck) Hints() []hint {
	if m.done {
		return []hint{{"any key", "close"}}
	}
	return []hint{{"esc", "stop"}}
}

func (m *modelCheck) View(c viewCtx) string {
	w := choiceWidth(c) - 4
	var lines []string
	add := func(l string) { lines = append(lines, l) }

	if m.err != nil && len(m.results) == 0 {
		for _, l := range wrapText(probeError(m.err), w) {
			add(sErr.Render(l))
		}
		add("")
		add(sMuted.Render("Nothing was sent."))
		return modalSized(c, "Check your models", lines, choiceWidth(c))
	}

	head := spinner(animFrame(c)) + " " + sBold.Render("Asking Claude about each model…") + sMuted.Render("  one tiny request each, a few tokens")
	if m.done {
		head = sOK.Render("✓") + " " + sBold.Render("Done")
	}
	add(head)
	add("")
	ok := 0
	for _, alias := range models.ProbeAliases {
		res, answered := m.results[alias]
		label := pad(sBold.Render(alias), 9)
		switch {
		case !answered && m.done:
			add(label + sMuted.Render("no answer"))
		case !answered:
			add(label + sMuted.Render(spinnerWord(c)))
		case res.OK:
			ok++
			add(label + sOK.Render("✓ available") + sMuted.Render("  "+res.ID))
		default:
			wrapped := wrapText(res.Error, w-9-2)
			add(label + sErr.Render("✗ unavailable"))
			for _, l := range wrapped {
				add(pad("", 9) + sMuted.Render(l))
			}
		}
	}
	if m.done {
		add("")
		add(fmt.Sprintf("%s of %d available. %s", sBold.Render(fmt.Sprintf("%d", ok)), len(models.ProbeAliases),
			sMuted.Render("Models you cannot use are greyed out when you pick one, and the answer is kept for this version of Claude Code.")))
	}
	return modalSized(c, "Check your models", wrapLines(lines, w), choiceWidth(c))
}

// spinnerWord is what a model that has not answered yet shows.
func spinnerWord(c viewCtx) string {
	dots := strings.Repeat("·", 1+animFrame(c)/4%3)
	return "asking" + dots
}

// wrapLines wraps lines that are longer than w.
func wrapLines(lines []string, w int) []string {
	var out []string
	for _, l := range lines {
		if lipglossWidth(l) <= w {
			out = append(out, l)
			continue
		}
		out = append(out, wrapText(l, w)...)
	}
	return out
}

package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/EdrenaVosh/squadup/internal/models"
	"github.com/EdrenaVosh/squadup/internal/workspace"
)

func TestARunningCheckShowsWhoHasAnsweredAndWhoIsStillBeingAsked(t *testing.T) {
	m := newModelCheck(workspace.Workspace{})
	defer m.cancel()
	m.Update(probeResultMsg{alias: "opus", res: models.ProbeResult{OK: true, ID: "claude-opus-5-5"}})
	m.Update(probeResultMsg{alias: "fable", res: models.ProbeResult{Error: "claude -p failed: this model is not on your plan"}})

	view := ansi.Strip(m.View(viewCtx{w: 80, h: 24}))

	for _, want := range []string{"Asking Claude about each model", "opus", "✓ available", "claude-opus-5-5", "fable", "✗ unavailable", "not on your plan", "sonnet", "asking", "haiku"} {
		if !strings.Contains(view, want) {
			t.Errorf("the screen lacks %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "Done") {
		t.Errorf("the check is not done:\n%s", view)
	}
	if hints := m.Hints(); len(hints) != 1 || hints[0].key != "esc" {
		t.Errorf("hints = %v: while it runs only esc, to stop it", hints)
	}
}

func TestStoppingARunningCheckClosesItAndCancelsTheRequests(t *testing.T) {
	m := newModelCheck(workspace.Workspace{})

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

	if cmd == nil || m.ctx.Err() == nil {
		t.Errorf("cmd %v, context error %v", cmd, m.ctx.Err())
	}
}

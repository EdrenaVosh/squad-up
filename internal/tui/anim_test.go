package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/EdrenaVosh/squadup/internal/testutil"
	"github.com/EdrenaVosh/squadup/internal/tui"
)

// animated is a harness whose interface runs its animations. The clock does
// not run by itself: the test moves it with frame.
func animated(t *testing.T) *harness {
	t.Helper()
	ws, _ := newWorkspace(t)
	testutil.HideClaude(t)
	h := &harness{t: t, ws: ws, paths: ws.Paths,
		m: tui.New(tui.Env{Paths: ws.Paths, Now: ws.Now, ClaudeVersion: "2.1.309", Animate: true})}
	m, _ := h.m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	h.m = m
	h.view() // the first draw starts the animations
	return h
}

// frame advances the animation clock by n frames.
func (h *harness) frame(n int) {
	h.t.Helper()
	for i := 0; i < n; i++ {
		m, _ := h.m.Update(tui.TickMsg())
		h.m = m
	}
}

func TestTheRosterDealsInAndRestartsOnANewSelection(t *testing.T) {
	h := animated(t)

	h.contains("· · ·") // the second tile has not arrived yet
	h.lacks("▂▃▅")
	h.frame(14)
	h.lacks("· · ·")
	h.contains("2 tools")

	h.press("down", "down") // Solo Indie has three agents
	h.view()
	h.contains("· · ·")
	h.frame(14)
	h.lacks("· · ·")
}

func TestWithoutAnimationEverythingIsThereAtOnce(t *testing.T) {
	h := newHarness(t)
	h.lacks("· · ·")
	h.contains("2 tools")
}

func TestTheFrameClockChangesTheColorsButNotTheText(t *testing.T) {
	h := animated(t)
	h.frame(14)
	first := h.m.View().Content
	h.frame(3)
	second := h.m.View().Content

	if first == second {
		t.Error("the shine and the selection pulse should move the colors")
	}
	if before, after := h.view(), func() string { h.frame(5); return h.view() }(); before != after {
		t.Errorf("the text must not change between frames once the roster is in:\n%s\n%s", before, after)
	}
}

func TestMouseSelectsAndOpensPresets(t *testing.T) {
	h := newHarness(t)
	h.view()

	h.send(tea.MouseClickMsg{X: 3, Y: 4, Button: tea.MouseLeft}) // the second preset
	h.contains("Refactor Day")
	h.send(tea.MouseClickMsg{X: 3, Y: 4, Button: tea.MouseLeft}) // again: open it
	h.contains("AGENTS")
	h.press("esc")

	h.view()
	h.send(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	h.contains("Solo Indie")
	h.send(tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	h.contains("Refactor Day")

	h.send(tea.MouseClickMsg{X: 60, Y: 3, Button: tea.MouseLeft}) // the card side does nothing
	h.contains("Refactor Day")
}

func TestTheEquipCelebrationPlaysOutAndSettles(t *testing.T) {
	h := animated(t)
	h.press("down", "down", "e") // Solo Indie
	h.view()
	h.press("y")
	h.view()

	h.contains("Squad ready", "  0%")
	spinning := false
	for _, s := range []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"} {
		spinning = spinning || strings.Contains(h.view(), s)
	}
	if !spinning {
		t.Errorf("the agents should be checking in:\n%s", h.view())
	}
	h.frame(6)
	top := strings.Join(strings.Split(h.view(), "\n")[1:5], "")
	if !strings.ContainsAny(top, "✦✧·*•") {
		t.Errorf("confetti should be falling:\n%s", h.view())
	}

	h.frame(8)
	h.contains("✓ ■ planner")
	h.frame(60)
	h.contains("100%", "✓ ■ tester", "Done.")
	h.lacks("⠋")
	top = strings.Join(strings.Split(h.view(), "\n")[1:5], "")
	if strings.ContainsAny(top, "✦✧·*•") {
		t.Errorf("the confetti should be over:\n%s", h.view())
	}
}

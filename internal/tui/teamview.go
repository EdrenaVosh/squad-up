package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/EdrenaVosh/squadup/internal/workspace"
)

// sectionBand is the rule that opens a group on the main screen. The agents
// get a bold, colored band; the squad's own settings a quiet dotted one, so
// the two kinds of card never read as one.
func sectionBand(label, note string, w, shift int, agents bool) string {
	right := " " + sBold.Render(note) + " "
	if agents {
		chip := lipgloss.NewStyle().Foreground(lipgloss.Color("#1A1200")).Background(brandStops[0]).Bold(true).Render(" ◆ " + label + " ")
		left := " " + chip + " "
		fill := max(w-lipglossWidth(left)-lipglossWidth(right), 0)
		rule := make([]string, 0, fill)
		for i := 0; i < fill; i++ {
			rule = append(rule, fg(flowColor(i, max(fill, 1), shift)).Bold(true).Render("━"))
		}
		return left + strings.Join(rule, "") + right
	}
	chip := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Background(brandStops[3]).Bold(true).Render(" ▪ " + label + " ")
	left := " " + chip + " "
	fill := max(w-lipglossWidth(left)-lipglossWidth(right), 0)
	return left + fg(brandStops[3]).Render(strings.Repeat("┄", fill)) + right
}

// flowStrip is one line that shows how the agents are organised: who leads,
// the order the work goes down the slots, and the review loop that sends
// defects back.
func flowStrip(c workspace.Card, w int) string {
	switch {
	case c.Initial:
		return " " + sMuted.Render("your original agents, exactly as they were: no flow, no shared notes")
	case len(c.Agents) < 2:
		return " " + sMuted.Render("one agent: nothing to hand over")
	case c.Flow == nil:
		return " " + sMuted.Render("separate helpers · set a Team flow to link them")
	}

	var steps []string
	for _, a := range c.Agents {
		if a.Name == c.Main {
			continue
		}
		step := swatch(a.Color) + " " + a.Name
		if c.Sharing && !a.Shared {
			step += fg(cIsolated).Render("⊘")
		}
		steps = append(steps, step)
	}
	lead := sMuted.Render("you")
	if c.Main != "" {
		lead = fg(brandStops[0]).Render("◆ ") + sBold.Render(c.Main)
	}
	line := " " + lead + sMuted.Render(" ▸ ") + strings.Join(steps, sMuted.Render(" → "))
	if c.Flow.Loops() {
		line += "   " + fg(brandStops[1]).Render("↺ ") + sMuted.Render(fmt.Sprintf("%s finds defects → %s fixes · up to %d", c.Flow.Gate, c.Flow.Fix, c.Flow.MaxRounds()))
	}
	return truncate(line, w)
}

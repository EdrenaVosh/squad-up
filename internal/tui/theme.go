package tui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// One amber accent, shades of grey for the rest. The colors are mid-tones so
// that they read on both light and dark terminals; ordinary text keeps the
// terminal's own foreground.
var (
	cAccent = lipgloss.Color("#E5A50A")
	cMuted  = lipgloss.Color("#8A8F98")
	cOK     = lipgloss.Color("#30A46C")
	cErr    = lipgloss.Color("#E5484D")
	cWarn   = lipgloss.Color("#C99A0A")
)

// agentColors are the eight colors Claude Code gives agents; a swatch uses the
// same one the agent has inside Claude Code.
var agentColors = map[string]color.Color{
	"red":    lipgloss.Color("#E5484D"),
	"blue":   lipgloss.Color("#3E63DD"),
	"green":  lipgloss.Color("#30A46C"),
	"yellow": lipgloss.Color("#F5D90A"),
	"purple": lipgloss.Color("#8E4EC6"),
	"orange": lipgloss.Color("#F76B15"),
	"pink":   lipgloss.Color("#D6409F"),
	"cyan":   lipgloss.Color("#05A2C2"),
}

func fg(c color.Color) lipgloss.Style { return lipgloss.NewStyle().Foreground(c) }

var (
	sAccent = fg(cAccent).Bold(true)
	sMuted  = fg(cMuted)
	sOK     = fg(cOK)
	sErr    = fg(cErr)
	sWarn   = fg(cWarn)
	sBold   = lipgloss.NewStyle().Bold(true)
)

// swatch is the colored square in front of an agent.
func swatch(name string) string {
	if c, ok := agentColors[name]; ok {
		return fg(c).Render("■")
	}
	return sMuted.Render("■")
}

// truncate cuts s to w cells, marking the cut with an ellipsis.
func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	return ansi.Truncate(s, w, "…")
}

// pad cuts or pads s to exactly w cells.
func pad(s string, w int) string {
	if w <= 0 {
		return ""
	}
	n := ansi.StringWidth(s)
	if n > w {
		return ansi.Truncate(s, w, "…")
	}
	return s + strings.Repeat(" ", w-n)
}

// center puts s in the middle of w cells.
func center(s string, w int) string {
	n := ansi.StringWidth(s)
	if n >= w {
		return truncate(s, w)
	}
	left := (w - n) / 2
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", w-n-left)
}

// fit returns exactly h lines of exactly w cells: extra lines are cut, missing
// ones are blank.
func fit(lines []string, w, h int) []string {
	out := make([]string, h)
	for i := range out {
		if i < len(lines) {
			out[i] = pad(lines[i], w)
		} else {
			out[i] = strings.Repeat(" ", w)
		}
	}
	return out
}

// box draws a rounded frame of exactly w×h cells around the lines, which are
// fitted to w-2 cells. dividerAt is the inner column of a vertical divider the
// caller draws inside the lines (0 for none); the bottom edge joins it with ┴.
func box(title, right string, lines []string, w, h, dividerAt int) string {
	inner := w - 2
	edge := sMuted
	bar := func(n int) string { return strings.Repeat("─", max(n, 0)) }

	left := " " + sAccent.Render(title) + " "
	label := ""
	if right != "" {
		label = " " + sMuted.Render(right) + " "
	}
	// "╭─" and "─╮" take 4 cells of the w.
	fill := w - 4 - ansi.StringWidth(left) - ansi.StringWidth(label)
	if fill < 1 {
		label = ""
		fill = w - 4 - ansi.StringWidth(left)
	}
	if fill < 0 {
		left = " " + sAccent.Render(truncate(title, max(w-6, 1))) + " "
		fill = max(w-4-ansi.StringWidth(left), 0)
	}
	rows := []string{edge.Render("╭─") + left + edge.Render(bar(fill)) + label + edge.Render("─╮")}

	for _, l := range fit(lines, inner, h-2) {
		rows = append(rows, edge.Render("│")+l+edge.Render("│"))
	}

	if dividerAt > 0 && dividerAt < inner {
		rows = append(rows, edge.Render("╰"+bar(dividerAt)+"┴"+bar(inner-dividerAt-1)+"╯"))
	} else {
		rows = append(rows, edge.Render("╰"+bar(inner)+"╯"))
	}
	return strings.Join(rows, "\n")
}

// lipglossWidth is the width of a string in terminal cells, ignoring styling.
func lipglossWidth(s string) int { return ansi.StringWidth(s) }

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
// The top and bottom edges run along the brand gradient.
func box(title, right string, lines []string, w, h, dividerAt int) string {
	return frame(title, 0, right, lines, w, h, dividerAt)
}

// frame is box with a shining title: shine is the position of the highlight
// in the title (see gradientText), 0 for none.
func frame(title string, shine float64, right string, lines []string, w, h, dividerAt int) string {
	inner := w - 2
	colors := gradient(w)
	edge := func(from int, text string) string {
		var b strings.Builder
		for i, r := range []rune(text) {
			b.WriteString(fg(colors[min(from+i, w-1)]).Render(string(r)))
		}
		return b.String()
	}
	side := fg(cDim).Render("│")

	label := ""
	if right != "" {
		label = " " + sMuted.Render(right) + " "
	}
	// "╭─" and "─╮" take 4 cells of the w.
	room := w - 4
	titleText := " " + title + " "
	fill := room - ansi.StringWidth(titleText) - ansi.StringWidth(label)
	if fill < 1 {
		label = ""
		fill = room - ansi.StringWidth(titleText)
	}
	if fill < 0 {
		titleText = " " + truncate(title, max(w-6, 1)) + " "
		fill = max(room-ansi.StringWidth(titleText), 0)
	}
	top := edge(0, "╭─") + gradientText(titleText, shine) + edge(2+ansi.StringWidth(titleText), strings.Repeat("─", fill)) + label +
		edge(w-2, "─╮")
	rows := []string{top}

	for i, l := range fit(lines, inner, h-2) {
		// The divider runs the full height, also below where the panes end.
		if dividerAt > 0 && dividerAt < inner && i >= len(lines) {
			l = strings.Repeat(" ", dividerAt) + fg(cDim).Render("│") + strings.Repeat(" ", inner-dividerAt-1)
		}
		rows = append(rows, side+l+side)
	}

	bottom := "╰" + strings.Repeat("─", inner) + "╯"
	if dividerAt > 0 && dividerAt < inner {
		bottom = "╰" + strings.Repeat("─", dividerAt) + "┴" + strings.Repeat("─", inner-dividerAt-1) + "╯"
	}
	return strings.Join(append(rows, edge(0, bottom)), "\n")
}

// lipglossWidth is the width of a string in terminal cells, ignoring styling.
func lipglossWidth(s string) int { return ansi.StringWidth(s) }

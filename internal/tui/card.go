package tui

import (
	"fmt"
	"strings"

	"github.com/EdrenaVosh/squadup/internal/workspace"
)

// cardLines draws the card of a preset in w cells and h lines: the title, and
// the agents as a roster of tiles that deal in one after another. frames is
// how long ago the card started to appear. When the tiles do not fit, the
// agents become a compact list.
func cardLines(c workspace.Card, w, h, frames int) []string {
	badge := ""
	switch {
	case c.Active && c.Initial:
		badge = sAccent.Render("● ACTIVE")
	case c.Active:
		badge = sAccent.Render("● EQUIPPED")
	case c.Initial:
		badge = sMuted.Render("◇ ORIGINAL")
	}
	title := c.Title
	if title == "" {
		title = c.Name
	}
	header := []string{spread(" "+accentDot(c.Accent)+" "+sBold.Render(title), badge+" ", w)}

	switch {
	case c.Error != "":
		header = append(header,
			" "+sErr.Render(truncate("✗ "+c.Error, w-2)),
			"",
			" "+sMuted.Render(truncate("Fix preset.yaml, then come back. `squad lint "+c.Name+"` has the details.", w-2)))
		return fit(header, w, h)
	case c.Initial && c.CapturedAt != nil:
		header = append(header, " "+sMuted.Render(truncate(captured(c), w-2)))
	default:
		header = append(header, " "+sMuted.Render(typewriter(truncate(c.Description, w-2), frames)))
	}
	header = append(header, "")

	leftW := min((w-2)*55/100, 30)
	pair := func(l1, v1, l2, v2 string) string {
		return " " + pad(sMuted.Render(pad(l1, 15))+v1, leftW) + pad(sMuted.Render(pad(l2, 12))+v2, w-2-leftW)
	}
	footer := []string{
		"",
		pair("Default model", modelOrDash(c.Model), "Main agent", orDash(c.Main, "—")),
		pair("Permissions", permissionWord(c), "Mode", orDash(c.Mode, "—")),
		" " + sMuted.Render(pad("Lint", 15)) + lintWord(c) + sMuted.Render(fmt.Sprintf(" · ~%d tokens/session", c.SessionTokens)),
	}

	room := max(h-len(header)-len(footer), 1)
	roster := rosterLines(c.Agents, w-2, room, frames)
	for i := range roster {
		roster[i] = " " + roster[i]
	}
	return fit(append(append(header, roster...), footer...), w, h)
}

// accentDot is the colored dot of a preset's accent.
func accentDot(accent string) string {
	if c, ok := agentColors[accent]; ok {
		return fg(c).Render("●")
	}
	return sMuted.Render("●")
}

// modelOrDash is a model pill, or a dash when the preset sets none.
func modelOrDash(model string) string {
	if model == "" {
		return sMuted.Render("—")
	}
	return modelPill(model)
}

// typewriter shows the first characters of s: six more with every frame.
func typewriter(s string, frames int) string {
	runes := []rune(s)
	if n := frames * 6; n < len(runes) {
		return string(runes[:n]) + "▍"
	}
	return s
}

func captured(c workspace.Card) string {
	return "your original setup · captured " + c.CapturedAt.UTC().Format("2006-01-02")
}

func orDash(s, dash string) string {
	if s == "" {
		return dash
	}
	return s
}

// permissionWord is the permission mode of a card, in one word.
func permissionWord(c workspace.Card) string {
	if c.Permissions == "" {
		return "—"
	}
	word, _, _ := strings.Cut(c.Permissions, " ")
	return word
}

// lintWord summarises the findings of a card.
func lintWord(c workspace.Card) string {
	switch {
	case c.Errors > 0:
		return sErr.Render(plural(c.Errors, "error"))
	case c.Warnings > 0:
		return sWarn.Render(plural(c.Warnings, "warning"))
	case c.Infos > 0:
		return sOK.Render("✓ clean") + sMuted.Render(" · "+plural(c.Infos, "note"))
	}
	return sOK.Render("✓ clean")
}

func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// spread puts left and right at the two ends of a line of w cells.
func spread(left, right string, w int) string {
	gap := w - lipglossWidth(left) - lipglossWidth(right)
	if gap < 1 {
		return truncate(left, w-lipglossWidth(right)-1) + " " + right
	}
	return left + strings.Repeat(" ", gap) + right
}

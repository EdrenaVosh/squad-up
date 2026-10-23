package tui

import (
	"fmt"
	"strings"

	"github.com/EdrenaVosh/squadup/internal/workspace"
)

// cardLines draws the card of a preset in w cells and h lines. When the agents
// do not fit, the table loses its tail first.
func cardLines(c workspace.Card, w, h int) []string {
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
	header := []string{spread(" "+sBold.Render(title), badge+" ", w)}

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
		header = append(header, " "+sMuted.Render(truncate(c.Description, w-2)))
	}
	header = append(header, "")

	var slots []string
	for i, a := range c.Agents {
		slots = append(slots, fmt.Sprintf(" %s  %s %s %s %s %s",
			sMuted.Render(fmt.Sprintf("SLOT %d", i+1)),
			swatch(a.Color), pad(a.Name, 12), pad(orDash(a.Model, "inherit"), 8), pad(orDash(a.Effort, "—"), 7), orDash(a.PermissionMode, "default")))
	}
	if len(slots) == 0 {
		slots = []string{" " + sMuted.Render("no agents yet")}
	}

	leftW := min((w-2)*55/100, 30)
	pair := func(l1, v1, l2, v2 string) string {
		return " " + pad(sMuted.Render(pad(l1, 15))+v1, leftW) + pad(sMuted.Render(pad(l2, 12))+v2, w-2-leftW)
	}
	footer := []string{
		"",
		pair("Default model", orDash(c.Model, "—"), "Main agent", orDash(c.Main, "—")),
		pair("Permissions", permissionWord(c), "Mode", orDash(c.Mode, "—")),
		" " + sMuted.Render(pad("Lint", 15)) + lintWord(c) + sMuted.Render(fmt.Sprintf(" · ~%d tokens/session", c.SessionTokens)),
	}

	room := max(h-len(header)-len(footer), 1)
	if len(slots) > room {
		slots = append(slots[:room-1], " "+sMuted.Render(fmt.Sprintf("… +%d more", len(slots)-room+1)))
	}
	return fit(append(append(header, slots...), footer...), w, h)
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

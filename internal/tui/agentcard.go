package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// cardWidth is the outer width of an agent card.
const cardWidth = 44

// AgentCardInfo is what an agent card shows.
type AgentCardInfo struct {
	Name           string
	Preset         string // where the agent was found
	Color          string // the agent's own color
	Role           string // the first sentence of its description
	Model          string
	Effort         string
	Tools          string
	PermissionMode string
}

// permissionLevels orders the permission modes by how much the agent may do
// without asking, from 1 to 5.
var permissionLevels = map[string]int{
	"plan": 1, "default": 2, "manual": 2, "dontAsk": 2, "acceptEdits": 3, "auto": 4, "bypassPermissions": 5,
}

// permissionBar draws the strength of a permission mode: "▰▰▰▱▱ acceptEdits".
func permissionBar(mode string) string {
	if mode == "" {
		mode = "default"
	}
	level, ok := permissionLevels[mode]
	if !ok {
		level = 2
	}
	return strings.Repeat("▰", level) + strings.Repeat("▱", 5-level) + " " + mode
}

// RenderAgentCard draws the card of an agent as a framed block of text. With
// color false it is plain text that survives a pipe.
func RenderAgentCard(c AgentCardInfo, color bool) string {
	style := func(s lipgloss.Style, text string) string {
		if !color {
			return text
		}
		return s.Render(text)
	}
	frame := sMuted
	if col, ok := agentColors[c.Color]; ok {
		frame = fg(col)
	}

	inner := cardWidth - 4 // one space of padding on each side of the frame
	row := func(content string) string {
		return style(frame, "│") + " " + pad(content, inner) + " " + style(frame, "│")
	}
	field := func(label, value string) string {
		return style(sMuted, pad(label, 13)) + truncate(value, inner-13)
	}

	name := style(frame, "■") + " " + style(sBold, c.Name)
	header := name
	if c.Preset != "" {
		header = spread(name, style(sMuted, c.Preset), inner)
	}

	lines := []string{row(header), row("")}
	role := c.Role
	if role == "" {
		role = "No description."
	}
	for _, l := range wrapText(role, inner) {
		lines = append(lines, row(l))
	}
	lines = append(lines, row(""),
		row(field("Model", orDash(c.Model, "inherit"))),
		row(field("Effort", orDash(c.Effort, "—"))),
		row(field("Tools", orDash(strings.TrimSpace(c.Tools), "all tools"))),
		row(field("Permissions", permissionBar(c.PermissionMode))),
	)

	bar := strings.Repeat("─", cardWidth-2)
	top := style(frame, "╭"+bar+"╮")
	bottom := style(frame, "╰"+bar+"╯")
	return top + "\n" + strings.Join(lines, "\n") + "\n" + bottom + "\n"
}

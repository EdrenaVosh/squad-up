package tui

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/EdrenaVosh/squadup/internal/workspace"
)

const (
	tileW = 17 // outer width of an agent tile
	tileH = 7  // and its height
)

// dealDelay is how many frames pass between one tile and the next appearing.
const dealDelay = 2

// tileColor is the border color of an agent: its own color in Claude Code.
func tileColor(a workspace.AgentInfo) color.Color {
	if c, ok := agentColors[a.Color]; ok {
		return c
	}
	return cMuted
}

// agentTile draws one agent as a framed tile of tileW×tileH cells. stage
// says how far it has appeared: 0 an empty outline, 1 the name, 2 everything.
func agentTile(a workspace.AgentInfo, slot, stage int) []string {
	inner := tileW - 2
	border := fg(tileColor(a))
	if stage == 0 {
		border = fg(cDim)
	}
	row := func(content string) string {
		return border.Render("│") + " " + pad(content, inner-1) + border.Render("│")
	}
	head := fmt.Sprintf("─ %d ", slot)
	top := border.Render("╭" + head + strings.Repeat("─", inner-len([]rune(head))) + "╮")
	bottom := border.Render("╰" + strings.Repeat("─", inner) + "╯")

	if stage == 0 {
		blank := row("")
		return []string{top, blank, blank, row(sMuted.Render("· · ·")), blank, blank, bottom}
	}
	lines := []string{top, row(swatch(a.Color) + " " + sBold.Render(truncate(a.Name, inner-4))), row(""), row(""), row(""), row(""), bottom}
	if stage >= 2 {
		lines[2] = row(modelOrDashW(a.Model, inner-2))
		lines[3] = row(effortBars(a.Effort))
		lines[4] = row(permissionStrength(a.PermissionMode))
		lines[5] = row(sMuted.Render(toolsText(a.Tools)))
	}
	return lines
}

func modelOrDashW(model string, w int) string {
	if model == "" {
		return modelPill("inherit")
	}
	return truncate(modelPill(model), w)
}

func toolsText(n int) string {
	switch n {
	case 0:
		return "all tools"
	case 1:
		return "1 tool"
	}
	return fmt.Sprintf("%d tools", n)
}

// rosterLines lays the agents out as rows of tiles that fit w cells. Tiles
// deal in with the frames; when even one row of them does not fit in h lines
// the agents are listed compactly instead.
func rosterLines(agents []workspace.AgentInfo, w, h, frames int) []string {
	if len(agents) == 0 {
		return []string{sMuted.Render("no agents yet — enter opens the preset, then add one")}
	}
	perRow := max(min((w+1)/(tileW+1), 6), 1)
	tileRows := (len(agents) + perRow - 1) / perRow
	if tileRows*tileH+(tileRows-1) > h {
		return compactRoster(agents, w, h)
	}

	var out []string
	for r := 0; r < tileRows; r++ {
		if r > 0 {
			out = append(out, "")
		}
		var tiles [][]string
		for i := r * perRow; i < min((r+1)*perRow, len(agents)); i++ {
			stage := min(max((frames-i*dealDelay+2)/2, 0), 2)
			tiles = append(tiles, agentTile(agents[i], i+1, stage))
		}
		for line := 0; line < tileH; line++ {
			parts := make([]string, len(tiles))
			for i, t := range tiles {
				parts[i] = t[line]
			}
			out = append(out, strings.Join(parts, " "))
		}
	}
	return out
}

// compactRoster is the agents as one line each, for cards too small for tiles.
func compactRoster(agents []workspace.AgentInfo, w, h int) []string {
	var out []string
	for i, a := range agents {
		if i == h-1 && len(agents) > h {
			out = append(out, sMuted.Render(fmt.Sprintf("… +%d more", len(agents)-i)))
			break
		}
		out = append(out, truncate(fmt.Sprintf("%s %s %s %s",
			sMuted.Render(fmt.Sprintf("%d", i+1)), swatch(a.Color), pad(a.Name, 14), permissionStrength(a.PermissionMode)), w))
	}
	return out
}

// shortMode shortens the long permission modes to fit a tile.
func shortMode(mode string) string {
	switch mode {
	case "acceptEdits":
		return "edits"
	case "bypassPermissions":
		return "bypass"
	}
	return mode
}

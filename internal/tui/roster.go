package tui

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	"github.com/EdrenaVosh/squadup/internal/workspace"
)

// Agent tiles come in two sizes, and every tile of a screen has the same one,
// whatever the number of agents.
const (
	smallTileW, smallTileH = 17, 7  // for the compact layout
	bigTileW, bigTileH     = 30, 13 // for the spacious one, which widens with the room
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

// tileOpts say how a tile is drawn.
type tileOpts struct {
	big      bool // the large tile, with the role and labels
	w        int  // outer width; 0 is the size of the kind of tile
	stage    int  // how far it has appeared: 0 an outline, 1 the name, 2 everything
	selected bool // the agent the cursor is on
	live     bool // the preset is equipped: the tile shows a heartbeat
	frame    int  // the animation frame
	team     bool // the agents work as a squad, so the tile says if it shares the notes
}

// equalizer is four bars that move: the agents of an equipped preset are alive.
func equalizer(frame, slot int) string {
	const levels = "▁▂▃▄▅▆▇█"
	runes := []rune(levels)
	var b strings.Builder
	for k := 0; k < 4; k++ {
		phase := float64(frame)/3 + float64(slot)*1.7 + float64(k)*1.1
		b.WriteRune(runes[int((math.Sin(phase)+1)/2*7.99)])
	}
	return b.String()
}

// tileSize is the outer size of a tile.
func tileSize(big bool) (w, h int) {
	if big {
		return bigTileW, bigTileH
	}
	return smallTileW, smallTileH
}

// agentTile draws one agent as a framed tile.
func agentTile(a workspace.AgentInfo, slot int, o tileOpts) []string {
	w, height := tileSize(o.big)
	if o.w > 0 {
		w = o.w
	}
	inner := w - 2
	col := tileColor(a)
	if o.stage == 0 {
		col = cDim
	}
	border := fg(col)
	corners := [6]string{"╭", "╮", "╰", "╯", "─", "│"}
	if o.selected {
		corners = [6]string{"┏", "┓", "┗", "┛", "━", "┃"}
		border = border.Bold(true)
	}
	row := func(content string) string {
		return border.Render(corners[5]) + " " + pad(content, inner-1) + border.Render(corners[5])
	}

	head := fmt.Sprintf("%s %d ", corners[4], slot)
	tail := ""
	if o.live && o.stage == 2 {
		if o.frame > 0 {
			tail = " " + fg(brandStops[0]).Render(equalizer(o.frame, slot)) + " "
		} else {
			tail = " " + fg(brandStops[0]).Render("●") + " "
		}
	}
	fill := inner - len([]rune(head)) - lipglossWidth(tail)
	top := border.Render(corners[0]+head) + border.Render(strings.Repeat(corners[4], max(fill, 0))) + tail + border.Render(corners[1])
	bottom := border.Render(corners[2] + strings.Repeat(corners[4], inner) + corners[3])

	blank := row("")
	lines := []string{top}
	if o.stage == 0 {
		for i := 1; i < height-1; i++ {
			if i == height/2 {
				lines = append(lines, row(sMuted.Render("· · ·")))
			} else {
				lines = append(lines, blank)
			}
		}
		return append(lines, bottom)
	}

	name := sBold.Render(truncate(a.Name, inner-4))
	if o.selected {
		name = sAccent.Render(truncate(a.Name, inner-4))
	}
	nameRow := swatch(a.Color) + " " + name
	if o.team && !o.big {
		nameRow = swatch(a.Color) + " " + sBold.Render(truncate(a.Name, inner-6))
		if o.selected {
			nameRow = swatch(a.Color) + " " + sAccent.Render(truncate(a.Name, inner-6))
		}
		mark := fg(cShared).Render("⇄")
		if !a.Shared {
			mark = fg(cIsolated).Render("⊘")
		}
		nameRow = pad(nameRow, inner-3) + mark
	}
	lines = append(lines, row(nameRow))
	if o.stage >= 2 {
		if o.big {
			lines = append(lines, bigTileBody(a, inner, row, o.team)...)
		} else {
			lines = append(lines,
				row(modelOrDashW(a.Model, inner-2)),
				row(effortBars(a.Effort)),
				row(permissionStrength(a.PermissionMode)),
				row(sMuted.Render(toolsText(a.Tools))))
		}
	}
	for len(lines) < height-1 {
		lines = append(lines, blank)
	}
	return append(lines, bottom)
}

// bigTileBody is what the large tile shows under the name: what the agent is
// for, then its model, effort, access and tools, each with a label.
func bigTileBody(a workspace.AgentInfo, inner int, row func(string) string, team bool) []string {
	out := []string{row(""), row(fg(cDim).Render(strings.Repeat("─", inner-2)))}
	role := wrapText(orDash(a.Role, "No description yet."), inner-2)
	for i := 0; i < 2; i++ {
		text := ""
		if i < len(role) {
			text = role[i]
			if i == 1 && len(role) > 2 {
				text = truncate(text+"…", inner-2)
			}
		}
		out = append(out, row(sMuted.Render(text)))
	}
	labeled := func(label, value string) string { return row(sMuted.Render(pad(label, 8)) + value) }
	out = append(out, row(""),
		labeled("model", modelOrDashW(a.Model, inner-10)),
		labeled("effort", effortBars(a.Effort)),
		labeled("access", permissionStrength(a.PermissionMode)),
		labeled("tools", toolsLine(a, inner-10)))
	if team {
		out = append(out, labeled("context", shareChip(a.Shared)))
	}
	return out
}

// toolsLine fits as many tool chips as there are room for, then a count.
func toolsLine(a workspace.AgentInfo, w int) string {
	if len(a.ToolNames) == 0 {
		return sMuted.Render("all tools")
	}
	var parts []string
	used := 0
	for i, t := range a.ToolNames {
		rest := len(a.ToolNames) - i
		chip := fg(cAccent).Render("‹") + t + fg(cAccent).Render("›")
		reserve := 0
		if rest > 1 {
			reserve = 4 // " +n"
		}
		if used+len(t)+2+reserve > w && i > 0 {
			parts = append(parts, sMuted.Render(fmt.Sprintf("+%d", rest)))
			break
		}
		parts = append(parts, chip)
		used += len(t) + 3
	}
	return strings.Join(parts, " ")
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

// tileRect is where a tile was drawn, relative to the roster, for the mouse.
type tileRect struct{ x, y, w, h, index int }

// rosterOpts say how the roster is drawn.
type rosterOpts struct {
	frames   int  // since the roster began to appear
	frame    int  // the animation frame
	selected int  // the selected agent, or -1
	live     bool // the preset is equipped
	big      bool // large tiles
	team     bool // the agents share notes, so the tiles say who does
}

// rosterLines lays the agents out as rows of equal tiles that fit w cells.
// Tiles deal in with the frames. When the rows of them do not fit in h lines
// the agents are listed compactly instead.
func rosterLines(agents []workspace.AgentInfo, w, h int, o rosterOpts) ([]string, []tileRect, int) {
	if len(agents) == 0 {
		return []string{sMuted.Render("no agents yet — enter opens the preset, then add one")}, nil, w
	}
	tw, th := tileSize(o.big)
	if o.big {
		// Wider terminals get wider tiles, but every tile of a screen is the same.
		tw = min(max((w+1)/4-1, bigTileW), 42)
	}
	tw = min(tw, w)
	perRow := max(min((w+1)/(tw+1), 8), 1)
	tileRows := (len(agents) + perRow - 1) / perRow
	if tileRows*th+(tileRows-1) > h {
		return compactRoster(agents, w, h, o.selected), nil, w
	}

	var out []string
	var rects []tileRect
	for r := 0; r < tileRows; r++ {
		if r > 0 {
			out = append(out, "")
		}
		var tiles [][]string
		for i := r * perRow; i < min((r+1)*perRow, len(agents)); i++ {
			stage := min(max((o.frames-i*dealDelay+2)/2, 0), 2)
			tiles = append(tiles, agentTile(agents[i], i+1, tileOpts{
				big: o.big, w: tw, stage: stage, selected: i == o.selected, live: o.live, frame: o.frame, team: o.team,
			}))
			rects = append(rects, tileRect{x: (i - r*perRow) * (tw + 1), y: len(out), w: tw, h: th, index: i})
		}
		for line := 0; line < th; line++ {
			parts := make([]string, len(tiles))
			for i, t := range tiles {
				parts[i] = t[line]
			}
			out = append(out, strings.Join(parts, " "))
		}
	}
	return out, rects, min(perRow, len(agents))*(tw+1) - 1
}

// compactRoster is the agents as one line each, for cards too small for tiles.
func compactRoster(agents []workspace.AgentInfo, w, h, selected int) []string {
	var out []string
	for i, a := range agents {
		if i == h-1 && len(agents) > h {
			out = append(out, sMuted.Render(fmt.Sprintf("… +%d more", len(agents)-i)))
			break
		}
		mark := " "
		if i == selected {
			mark = selBar()
		}
		out = append(out, truncate(fmt.Sprintf("%s%s %s %s %s",
			mark, sMuted.Render(fmt.Sprintf("%d", i+1)), swatch(a.Color), pad(a.Name, 14), permissionStrength(a.PermissionMode)), w))
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

package tui

import (
	"fmt"
	"strings"

	"github.com/EdrenaVosh/squadup/internal/workspace"
)

// The main screen switches to its spacious layout from this size.
const (
	spaciousW = 100
	spaciousH = 30
)

// airyH is the terminal height from which the main screen adds padding.
const airyH = 38

// headerLines are the keys of the info cluster in the header.
func (h *home) headerInfo(c viewCtx) []string {
	equipped := "◇ " + sBold.Render("initial") + sMuted.Render(" is active")
	for _, e := range h.entries {
		if e.Active && !e.Initial {
			equipped = fg(brandStops[0]).Render("◉") + " " + sBold.Render(e.Name) + sMuted.Render(" is equipped")
		}
	}
	sync := sOK.Render("✓") + sMuted.Render(" in sync with your config")
	if len(h.drift) > 0 {
		sync = sWarn.Render("⚠") + " " + plural(len(h.drift), "manual edit")
	}
	version := sMuted.Render("claude not found")
	if c.claude != "" {
		version = sMuted.Render("claude ") + c.claude
		if h.model != "" {
			version += sMuted.Render(" · ") + modelPill(h.model)
		}
	}
	presets, agents := 0, 0
	for _, e := range h.entries {
		if !e.Initial {
			presets++
			agents += e.Agents
		}
	}
	stats := sMuted.Render(fmt.Sprintf("%s · %s", plural(presets, "preset"), plural(agents, "agent")))
	return []string{version, equipped, sync, stats}
}

// scatter hashes a cell to a number that looks random but is always the same.
func scatter(x, y int) uint32 {
	h := uint32(x+1)*2654435761 ^ uint32(y+1)*2246822519
	h ^= h >> 15
	h *= 2246822519
	h ^= h >> 13
	return h
}

// twinkle is one row of a faint star field: stars blink in and out, each on
// its own beat. With animation off the stars stand still.
func twinkle(w, row, frame int, anim bool) string {
	glyphs := []string{" ", " ", "·", "·", "✧", "✦", "✧", "·", " ", " ", " ", " ", " ", " ", " ", " "}
	var b strings.Builder
	for x := 0; x < w; x++ {
		key := scatter(x, row)
		if key%19 != 0 {
			b.WriteByte(' ')
			continue
		}
		phase := 5 // a still star
		if anim {
			phase = (frame/2 + int(key>>8)) % len(glyphs)
		}
		g := glyphs[phase]
		if g == " " {
			b.WriteByte(' ')
			continue
		}
		b.WriteString(fg(flowColor(x, w, frame)).Render(g))
	}
	return b.String()
}

// headerBand is the big logo on the left, stars in the middle and the status
// of the config on the right.
func (h *home) headerBand(c viewCtx, inner int, tall bool) []string {
	logo := bigLogo(tall, c.shift())
	logoW := bigLogoWidth(tall)
	info := h.headerInfo(c)
	out := make([]string, len(logo))
	for i, l := range logo {
		text := ""
		if j := i - (len(logo)-len(info))/2; j >= 0 && j < len(info) {
			text = info[j]
		}
		textW := lipglossWidth(text)
		sky := inner - 10 - logoW - textW
		out[i] = "   " + l + "  " + twinkle(max(sky, 0), i, c.frame, c.anim) + "   " + text
		if text == "" {
			out[i] = "   " + l + "  " + twinkle(max(inner-6-logoW, 0), i, c.frame, c.anim)
		}
		out[i] = pad(out[i], inner)
	}
	return out
}

// viewSpacious is the main screen on a big terminal: the logo, the presets as
// cards, and for the selected one its roster, panels and the selected agent.
func (h *home) viewSpacious(c viewCtx, rows int) string {
	inner := c.w - 2
	tall := c.h >= 44 && c.w >= 120
	h.air = c.h >= airyH
	header := h.headerBand(c, inner, tall)
	if h.air {
		header = append([]string{pad("", inner)}, header...) // space between the frame's title and the logo
	}
	header = append(header, fg(cDim).Render(strings.Repeat("─", inner)))
	bodyH := rows - 2 - len(header)

	listW := min(max(inner/4, 30), 46)
	h.listW, h.ox, h.oy = listW, c.ox, c.oy
	left := h.cardList(listW, bodyH, len(header), c)
	right := h.detail(inner-listW-1, bodyH, len(header), listW, c)

	lines := append(header, panes(left, right, listW, inner)...)
	return frame("◆ squadup", c.shift(), "", lines, c.w, rows, listW)
}

// cardList is the presets as cards of three lines each, Initial pinned at the
// bottom. headerRows is where the list starts, for the mouse.
func (h *home) cardList(w, rows, headerRows int, c viewCtx) []string {
	h.hits = h.hits[:0]
	vis := h.visible()
	var presets []workspace.Entry
	var initialEntry *workspace.Entry
	for i := range vis {
		if vis[i].Initial {
			initialEntry = &vis[i]
		} else {
			presets = append(presets, vis[i])
		}
	}

	title := " " + sMuted.Render("PRESETS") + " " + fg(cDim).Render(fmt.Sprintf("%d", len(presets)))
	lines := []string{title, ""}
	if h.air {
		lines = append([]string{""}, lines...)
	}
	footer := []string{" " + fg(cDim).Render(strings.Repeat("─", max(w-2, 0)))}
	if initialEntry != nil {
		footer = append(footer, h.card3(*initialEntry, h.cursor == len(presets), w, c)...)
	}

	const cardH = 4 // three lines and a gap
	room := max((rows-len(lines)-len(footer)-2)/cardH, 1)
	start := 0
	if h.cursor >= room && h.cursor < len(presets) {
		start = h.cursor - room + 1
	}
	for i := start; i < len(presets) && i < start+room; i++ {
		h.hits = append(h.hits, hitRow{line: headerRows + len(lines), index: i})
		lines = append(lines, h.card3(presets[i], i == h.cursor, w, c)...)
		lines = append(lines, "")
	}
	if len(presets) == 0 {
		msg := "no presets yet — n creates one"
		if h.query != "" {
			msg = "nothing matches"
		}
		lines = append(lines, " "+sMuted.Render(truncate(msg, w-2)))
	}
	if start+room < len(presets) {
		lines = append(lines, " "+sMuted.Render(fmt.Sprintf("↓ %d more", len(presets)-start-room)))
	}
	for len(lines) < rows-len(footer) {
		lines = append(lines, "")
	}
	lines = lines[:max(rows-len(footer), 0)]
	if initialEntry != nil {
		h.hits = append(h.hits, hitRow{line: headerRows + len(lines) + 1, index: len(presets)})
	}
	if h.filtering {
		lines[len(lines)-1] = " " + h.filter.View()
	} else if h.query != "" {
		lines[len(lines)-1] = " " + sMuted.Render("/ "+h.query)
	}
	return append(lines, footer...)
}

// card3 draws one preset as a card of three lines: the name, its agents as
// colored squares, and what it is for.
func (h *home) card3(e workspace.Entry, selected bool, w int, c viewCtx) []string {
	bar := "  "
	nameStyle := sMuted
	if selected {
		col := brandStops[0]
		if c.anim {
			col = pulse(c.frame, brandStops[0], brandStops[1])
		}
		bar = fg(col).Render("▌") + " "
		nameStyle = sBold
	}
	marker := sMuted.Render("○")
	badge := ""
	switch {
	case e.Active && !e.Initial:
		marker = fg(brandStops[0]).Bold(true).Render("◉")
		badge = fg(brandStops[0]).Render("● LIVE")
	case e.Active:
		marker = sAccent.Render("●")
		badge = sAccent.Render("● ACTIVE")
	case e.Initial:
		marker = sMuted.Render("◇")
	case agentColors[e.Accent] != nil:
		marker = accentDot(e.Accent)
	}
	title := e.Title
	if title == "" || e.Initial {
		title = e.Name
	}
	if !selected {
		nameStyle = sPlain
	}
	first := " " + bar + marker + " " + nameStyle.Render(truncate(title, w-12))
	first = spread(first, badge+" ", w)

	var squares []string
	for i := 0; i < e.Agents && i < 8; i++ {
		col := ""
		if i < len(e.Colors) {
			col = e.Colors[i]
		}
		squares = append(squares, swatch(col))
	}
	if e.Agents > 8 {
		squares = append(squares, sMuted.Render(fmt.Sprintf("+%d", e.Agents-8)))
	}
	second := " " + bar + "  " + strings.Join(squares, " ") + sMuted.Render(" "+plural(e.Agents, "agent"))
	desc := e.Description
	if e.Initial {
		desc = "your original setup"
	}
	if e.Error != "" {
		desc = "✗ " + e.Error
	}
	third := " " + bar + "  " + sMuted.Render(truncate(desc, w-6))
	return []string{pad(first, w), pad(second, w), pad(third, w)}
}

// detail is the right side: the title of the selected preset, its roster with
// the selected agent beside it, a row of panels, and what equipping would do.
func (h *home) detail(w, rows, headerRows, listW int, c viewCtx) []string {
	card := h.card
	frames := h.appear.frames(c)
	live := card.Active && !card.Initial

	badge := ""
	switch {
	case card.Active && card.Initial:
		badge = sAccent.Render("● ACTIVE")
	case card.Active:
		badge = fg(brandStops[0]).Bold(true).Render("● EQUIPPED")
	case card.Initial:
		badge = sMuted.Render("◇ ORIGINAL")
	}
	title := card.Title
	if title == "" {
		title = card.Name
	}
	titleW := w - 2 - lipglossWidth(badge) - 1
	head := []string{}
	if h.air {
		head = append(head, "")
	}
	switch {
	case card.Error != "":
		head = append(head, spread(" "+accentDot(card.Accent)+" "+sBold.Render(title), badge+" ", w))
		return fit(append(head, " "+sErr.Render(truncate("✗ "+card.Error, w-2)), "",
			" "+sMuted.Render(truncate("Fix preset.yaml, then come back. `squad lint "+card.Name+"` has the details.", w-2))), w, rows)
	case card.Initial && card.CapturedAt != nil:
		head = append(head, spread(" "+accentDot(card.Accent)+" "+sBold.Render(title)+sMuted.Render("  "+truncate(captured(card), max(titleW-lipglossWidth(title)-6, 0))), badge+" ", w))
	default:
		desc := typewriter(truncate(card.Description, max(titleW-lipglossWidth(title)-6, 0)), frames)
		head = append(head, spread(" "+accentDot(card.Accent)+" "+sBold.Render(title)+sMuted.Render("  "+desc), badge+" ", w))
	}
	if h.air {
		head = append(head, "")
	}
	head = append(head,
		sectionBand("AGENTS", plural(len(card.Agents), "slot")+" · in order", w-2, c.shift(), true),
		flowStrip(card, w-2))
	if h.air {
		head = append(head, "")
	}

	// The layout under the title does not depend on how many agents there are:
	// the roster has room for one or two rows of equal tiles, and the panels
	// sit at a fixed height under it.
	rowsOfTiles := min(max((rows-len(head)-panelH-2+1)/(bigTileH+1), 1), 2)
	region := rowsOfTiles*bigTileH + (rowsOfTiles - 1)
	ph := min(max(rows-len(head)-region-2, panelH), 14)
	showPanels := rows-len(head)-region-1 >= panelH

	opts := rosterOpts{frames: frames, frame: c.frame, selected: h.agent, live: live, big: true, team: card.Sharing}
	roster, rects, _ := rosterLines(card.Agents, w-2, region, opts)
	for _, r := range rects {
		h.tiles = append(h.tiles, tileHit{
			x: c.ox + 1 + listW + 1 + 1 + r.x, y: c.oy + 1 + h.topRows + headerRows + len(head) + r.y, w: r.w, h: r.h, index: r.index,
		})
	}
	for len(roster) < region {
		roster = append(roster, "")
	}
	out := append([]string{}, head...)
	for _, l := range roster[:region] {
		out = append(out, " "+l)
	}

	if showPanels {
		if ph > panelH { // spare room: a gap between the agents and the squad's own cards
			out = append(out, "")
			ph--
		}
		out = append(out, sectionBand("SQUAD", "settings, cost and health of the whole preset", w-2, c.shift(), false))
		if h.air && ph > panelH {
			out = append(out, "")
			ph--
		}
		for _, l := range panelsRow(card, h.activity, h.plan, h.planErr, w-2, ph) {
			out = append(out, " "+l)
		}
	} else {
		leftW := min((w-2)*55/100, 30)
		pair := func(l1, v1, l2, v2 string) string {
			return " " + pad(sMuted.Render(pad(l1, 15))+v1, leftW) + pad(sMuted.Render(pad(l2, 12))+v2, w-2-leftW)
		}
		out = append(out, "",
			pair("Default model", modelOrDash(card.Model), "Main agent", orDash(card.Main, "—")),
			pair("Permissions", permissionWord(card), "Mode", orDash(card.Mode, "—")),
			" "+sMuted.Render(pad("Lint", 15))+lintWord(card)+sMuted.Render(fmt.Sprintf(" · ~%d tokens/session", card.SessionTokens)))
	}
	return fit(out, w, rows)
}

// firstLine is the first line of an error message.
func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

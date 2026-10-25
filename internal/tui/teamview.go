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
		return " " + sMuted.Render("your original agents, exactly as they were: no flow, nothing added")
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
		if a.Independent {
			step += fg(cIndependent).Bold(true).Render("◈")
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

// flowNode is one box of the flow diagram.
type flowNode struct {
	text  string // styled
	width int
}

// flowDiagram draws how the agents depend on each other: the lead on the left,
// the agents in slot order along arrows that light up one after another, and,
// for a review loop, an arc under them from the gate back to the agent that
// fixes. It is empty when the card has no flow or the diagram does not fit in
// w cells.
func flowDiagram(c workspace.Card, w, frame int, anim bool) []string {
	if c.Initial || c.Flow == nil || len(c.Agents) < 2 {
		return nil
	}
	lead := flowNode{text: sMuted.Render("you"), width: 3}
	if c.Main != "" {
		lead = flowNode{text: fg(brandStops[0]).Bold(true).Render("◆ ") + sBold.Render(c.Main), width: 2 + lipglossWidth(c.Main)}
	}
	nodes := []flowNode{lead}
	index := map[string]int{} // agent name → its node
	if c.Main != "" {
		index[c.Main] = 0
	}
	for _, a := range c.Agents {
		if a.Name == c.Main {
			continue
		}
		text, width := swatch(a.Color)+" "+a.Name, 2+lipglossWidth(a.Name)
		if a.Independent {
			text += fg(cIndependent).Bold(true).Render("◈")
			width++
		}
		index[a.Name] = len(nodes)
		nodes = append(nodes, flowNode{text: text, width: width})
	}
	const arrowW = 5 // " ──▶ "
	total := 1
	for _, n := range nodes {
		total += n.width
	}
	total += arrowW * (len(nodes) - 1)
	if len(nodes) < 2 || total > w {
		return nil
	}

	// The arrow that is lit moves along the flow, then along the loop.
	segments := len(nodes) - 1
	lit := -1
	if anim && frame > 0 {
		lit = (frame / 5) % (segments + 2)
	}
	arrow := func(k int) string {
		if k == lit {
			return fg(brandStops[0]).Bold(true).Render(" ━━▶ ")
		}
		return fg(cDim).Render(" ──▶ ")
	}
	row := " "
	starts := make([]int, len(nodes))
	x := 1
	for i, n := range nodes {
		if i > 0 {
			row += arrow(i - 1)
			x += arrowW
		}
		starts[i] = x
		row += n.text
		x += n.width
	}
	out := []string{row}
	if !c.Flow.Loops() {
		return out
	}

	gate, okG := index[c.Flow.Gate]
	fix, okF := index[c.Flow.Fix]
	if !okG || !okF {
		return out
	}
	gx, fx := starts[gate]+nodes[gate].width/2, starts[fix]+nodes[fix].width/2
	left, right := min(gx, fx), max(gx, fx)
	arc := fg(brandStops[1])
	if lit == segments {
		arc = fg(brandStops[0]).Bold(true)
	}

	cells := func() []string {
		r := make([]string, max(total, w)+1)
		for i := range r {
			r[i] = " "
		}
		return r
	}
	up, base := cells(), cells()
	up[fx], up[gx] = arc.Render("▲"), arc.Render("│")
	for i := left; i <= right; i++ {
		base[i] = arc.Render("─")
	}
	base[left], base[right] = arc.Render("╰"), arc.Render("╯")
	full := fmt.Sprintf(" ↺ %s finds defects · up to %d ", c.Flow.Gate, c.Flow.MaxRounds())
	short := fmt.Sprintf(" ↺ ×%d ", c.Flow.MaxRounds())
	put := func(at int, label string) {
		for i, r := range []rune(label) {
			base[at+i] = sMuted.Render(string(r))
		}
	}
	room := right - left - 1
	switch {
	case lipglossWidth(full) <= room:
		put(left+1+(room-lipglossWidth(full))/2, full)
	case right+1+lipglossWidth(full) <= w:
		put(right+1, full)
	case lipglossWidth(short) <= room:
		put(left+1+(room-lipglossWidth(short))/2, short)
	}
	for len(up) > 0 && up[len(up)-1] == " " {
		up = up[:len(up)-1]
	}
	for len(base) > 0 && base[len(base)-1] == " " {
		base = base[:len(base)-1]
	}
	return append(out, strings.Join(up, ""), strings.Join(base, ""))
}

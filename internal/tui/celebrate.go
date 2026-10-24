package tui

import (
	"fmt"
	"image/color"
	"strings"
)

// confettiFrames is how long the confetti falls.
const confettiFrames = 36

var confettiGlyphs = []string{"✦", "✧", "·", "*", "•"}

// confetti draws h lines of w cells of falling sparkles in the agent colors.
// Each piece starts above the top at its own time and sits at a fixed column,
// so the same frame always draws the same picture. After confettiFrames the
// rain is over and the lines are blank.
func confetti(w, h, frames int) []string {
	cells := make([][]string, h)
	for y := range cells {
		cells[y] = make([]string, w)
	}
	palette := agentColorList()
	if frames < confettiFrames && w > 0 {
		for i := 0; i < w/2; i++ {
			x := (i*37 + 11) % w
			start := (i * 7) % 18
			y := (frames-start)/2 - 1
			if frames < start || y < 0 || y >= h {
				continue
			}
			cells[y][x] = fg(palette[i%len(palette)]).Render(confettiGlyphs[i%len(confettiGlyphs)])
		}
	}
	lines := make([]string, h)
	for y, row := range cells {
		var b strings.Builder
		for _, c := range row {
			if c == "" {
				b.WriteByte(' ')
			} else {
				b.WriteString(c)
			}
		}
		lines[y] = b.String()
	}
	return lines
}

// agentColorList is the agent colors in a fixed order.
func agentColorList() []color.Color {
	order := []string{"red", "orange", "yellow", "green", "cyan", "blue", "purple", "pink"}
	out := make([]color.Color, len(order))
	for i, name := range order {
		out[i] = agentColors[name]
	}
	return out
}

// progressBar fills w cells along the brand gradient up to fraction.
func progressBar(w int, fraction float64) string {
	filled := int(float64(w) * min(max(fraction, 0), 1))
	colors := gradient(w)
	var b strings.Builder
	for i := 0; i < w; i++ {
		if i < filled {
			b.WriteString(fg(colors[i]).Render("█"))
		} else {
			b.WriteString(fg(cDim).Render("░"))
		}
	}
	return b.String()
}

// doneLines is the screen after an equip: confetti, the agents checking in one
// by one, a progress bar, and the facts that matter afterwards.
func (e *equipScreen) doneLines(c viewCtx) []string {
	inner := c.w - 2
	f := e.appear.frames(c)

	lines := confetti(inner, 4, f)
	headline := "✦ Squad ready ✦"
	if e.name == "initial" {
		headline = "✦ Back to your original setup ✦"
	}
	lines = append(lines, center(gradientText(headline, 0), inner), "")

	var names []string
	var colors []string
	if card, err := e.ws.Card(e.name); err == nil {
		for _, a := range card.Agents {
			names = append(names, a.Name)
			colors = append(colors, a.Color)
		}
	}
	for i, name := range names {
		arrived := f >= 4+3*i
		mark := spinner(f)
		if arrived {
			mark = sOK.Bold(true).Render("✓")
		}
		lines = append(lines, center(fmt.Sprintf("%s %s %s", mark, swatch(colors[i]), name), inner))
	}
	if len(names) > 0 {
		lines = append(lines, "")
	}

	fraction := min(float64(f)/15, 1)
	barW := min(40, inner-8)
	lines = append(lines, center(progressBar(barW, fraction)+fmt.Sprintf(" %3d%%", int(fraction*100)), inner), "")

	lines = append(lines, " "+sOK.Render("✓ Done."))
	lines = append(lines, " "+sMuted.Render("A backup of the previous state was taken: "+e.result.BackupID))
	lines = append(lines, " "+sMuted.Render("`squad undo` goes back, `squad restore` returns your original setup."))
	for _, n := range e.plan.Notes {
		lines = append(lines, "", " "+sWarn.Render(truncate("! "+n, inner-2)))
	}
	return lines
}

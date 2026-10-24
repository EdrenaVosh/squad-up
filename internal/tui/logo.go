package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

const (
	wordmark = "S Q U A D U P"
	tagline  = "Switch Claude Code agent teams with one command"
)

// logoLines draws the logo in three lines of w cells: the wordmark appearing
// letter by letter, the eight agent colors popping in and then rippling, and
// the tagline typing itself out. frames is how long ago it started.
func logoLines(w, frames int) []string {
	runes := []rune(wordmark)
	shown := min(frames/2+1, len(runes))
	colors := gradient(len(runes))
	var word strings.Builder
	for i, r := range runes {
		switch {
		case i >= shown:
			word.WriteRune(' ')
		default:
			c := colors[i]
			if i == shown-1 && shown < len(runes) {
				c = lipgloss.Lighten(c, 0.6) // the newest letter flashes
			}
			word.WriteString(fg(c).Bold(true).Render(string(r)))
		}
	}

	palette := agentColorList()
	squares := make([]string, len(palette))
	wave := -1
	if frames >= 24 {
		wave = (frames - 24) / 2 % (len(palette) * 3)
	}
	for i, c := range palette {
		var cell string
		switch {
		case frames < 12+i:
			cell = fg(cDim).Render("·")
		default:
			col := c
			if d := wave - i; wave >= 0 && d >= 0 && d < 3 {
				col = lipgloss.Lighten(c, 0.5-0.15*float64(d))
			}
			cell = fg(col).Render("■")
		}
		squares[i] = cell
	}

	typed := tagline
	if n := (frames - 8) * 3; n < len([]rune(tagline)) {
		typed = string([]rune(tagline)[:max(n, 0)])
	}
	return []string{
		center(word.String(), w),
		center(strings.Join(squares, " "), w),
		center(sMuted.Render(typed), w),
	}
}

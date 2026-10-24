package tui

import (
	"strings"
)

// glyphs is a 5×5 pixel font for the letters of the wordmark.
var glyphs = map[rune][5]string{
	'S': {".####", "#....", ".###.", "....#", "####."},
	'Q': {".###.", "#...#", "#.#.#", "#..#.", ".##.#"},
	'U': {"#...#", "#...#", "#...#", "#...#", ".###."},
	'A': {".###.", "#...#", "#####", "#...#", "#...#"},
	'D': {"####.", "#...#", "#...#", "#...#", "####."},
	'P': {"####.", "#...#", "####.", "#....", "#...."},
}

const bigWord = "SQUADUP"

// bitmap returns the wordmark as rows of pixels: one string per pixel row,
// '#' for a lit pixel, with one empty column between the letters.
func bitmap() [5]string {
	var rows [5]string
	for i, r := range bigWord {
		g := glyphs[r]
		for y := range rows {
			if i > 0 {
				rows[y] += "."
			}
			rows[y] += g[y]
		}
	}
	return rows
}

// bigLogo draws the wordmark in block letters, its colors flowing with shift.
// Tall is five rows of two-cell pixels; otherwise it is three rows made of
// half blocks. A cell that is not part of a letter is a space.
func bigLogo(tall bool, shift int) []string { return bigLogoWipe(tall, shift, 1<<20) }

// bigLogoWipe is bigLogo with only the first cols pixel columns showing, so
// that the logo can be wiped in from the left.
func bigLogoWipe(tall bool, shift, cols int) []string {
	rows := bitmap()
	width := len(rows[0])
	if tall {
		out := make([]string, len(rows))
		for y, row := range rows {
			var b strings.Builder
			for x := 0; x < width; x++ {
				if row[x] == '#' && x < cols {
					b.WriteString(fg(flowColor(x, width, shift)).Render("██"))
				} else {
					b.WriteString("  ")
				}
			}
			out[y] = b.String()
		}
		return out
	}
	out := make([]string, 3)
	for y := range out {
		var b strings.Builder
		for x := 0; x < width; x++ {
			top := rows[2*y][x] == '#' && x < cols
			bottom := 2*y+1 < len(rows) && rows[2*y+1][x] == '#' && x < cols
			cell := " "
			switch {
			case top && bottom:
				cell = "█"
			case top:
				cell = "▀"
			case bottom:
				cell = "▄"
			}
			if cell == " " {
				b.WriteString(cell)
			} else {
				b.WriteString(fg(flowColor(x, width, shift)).Render(cell))
			}
		}
		out[y] = b.String()
	}
	return out
}

// bigLogoWidth is the width of the wordmark in cells.
func bigLogoWidth(tall bool) int {
	w := len(bitmap()[0])
	if tall {
		return 2 * w
	}
	return w
}

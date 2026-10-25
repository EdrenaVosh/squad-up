package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// cClaude is the orange of Claude Code's own mascot.
var cClaude = lipgloss.Color("#D97757")

// mascotW and mascotH are the size of the full mascot in cells.
const (
	mascotW = 9
	mascotH = 3
)

// The mascot is the little orange creature Claude Code greets you with, drawn
// from quarter blocks: a body, two eyes, two arms and four feet.
//
//	 ▐▛███▜▌
//	▝▜█████▛▘
//	  ▘▘ ▝▝
//
// Its arms are the quarter blocks at the ends of the second row. Raised they
// are ▝ and ▘, lowered ▗ and ▖; a typing mascot raises them in turn.
const (
	headOpen   = " ▐▛███▜▌ "
	headBlink  = " ▐█████▌ "
	leftUp     = "▝"
	leftDown   = "▗"
	rightUp    = "▘"
	rightDown  = "▖"
	bodyMiddle = "▜█████▛"
)

// feet are the steps of its shuffle.
var feet = []string{"  ▘▘ ▝▝  ", "  ▝▘ ▘▝  ", "  ▝▝ ▘▘  ", "  ▝▘ ▘▝  "}

// mascot is the working mascot at a frame: it types (the arms go up and down
// in turn), shuffles its feet and blinks now and then. slot offsets the beat
// so that several of them do not move as one. Frame 0 is a still picture, which
// is also what is drawn when animation is off.
func mascot(frame, slot int) []string {
	head, left, right, step := headOpen, leftUp, rightUp, feet[0]
	if frame > 0 {
		t := frame + slot*5
		if t%34 < 2 {
			head = headBlink
		}
		switch (t / 2) % 4 {
		case 0:
			left, right = leftUp, rightDown
		case 1:
			left, right = leftUp, rightUp
		case 2:
			left, right = leftDown, rightUp
		default:
			left, right = leftDown, rightDown
		}
		step = feet[(t/3)%len(feet)]
	}
	rows := []string{head, left + bodyMiddle + right, step}
	for i, r := range rows {
		rows[i] = fg(cClaude).Render(r)
	}
	return rows
}

// miniMascot is a one-line mascot for a tile's edge: it blinks and sways.
func miniMascot(frame, slot int) string {
	body := "▐▛█▜▌"
	if frame > 0 {
		t := frame + slot*5
		switch {
		case t%34 < 2:
			body = "▐███▌"
		case (t/3)%2 == 1:
			body = "▗▛█▜▖"
		}
	}
	return fg(cClaude).Render(body)
}

// mascotRow lays n mascots side by side, each with its own beat.
func mascotRow(n, frame, gap int) []string {
	rows := make([]string, mascotH)
	for i := 0; i < n; i++ {
		m := mascot(frame, i)
		for r := range rows {
			if i > 0 {
				rows[r] += strings.Repeat(" ", gap)
			}
			rows[r] += m[r]
		}
	}
	return rows
}

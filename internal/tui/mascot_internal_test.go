package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func plainMascot(frame, slot int) string {
	rows := mascot(frame, slot)
	for i, r := range rows {
		rows[i] = ansi.Strip(r)
	}
	return strings.Join(rows, "\n")
}

func TestTheMascotIsTheOrangeCreatureOfClaudeCode(t *testing.T) {
	want := " ▐▛███▜▌ \n▝▜█████▛▘\n  ▘▘ ▝▝  "
	if got := plainMascot(0, 0); got != want {
		t.Errorf("still mascot:\n%s\nwant:\n%s", got, want)
	}
	for _, r := range mascot(0, 0) {
		if lipglossWidth(r) != mascotW {
			t.Errorf("row %q is %d cells wide, want %d", ansi.Strip(r), lipglossWidth(r), mascotW)
		}
	}
	if len(mascot(0, 0)) != mascotH {
		t.Errorf("height = %d", len(mascot(0, 0)))
	}
}

func TestTheWorkingMascotTypesShufflesAndBlinks(t *testing.T) {
	seen := map[string]bool{}
	blinked, armsUp, armsDown := false, false, false
	for f := 1; f <= 120; f++ {
		m := plainMascot(f, 0)
		seen[m] = true
		rows := strings.Split(m, "\n")
		blinked = blinked || rows[0] == " ▐█████▌ "
		armsUp = armsUp || strings.HasPrefix(rows[1], "▝") && strings.HasSuffix(rows[1], "▘")
		armsDown = armsDown || strings.HasPrefix(rows[1], "▗") && strings.HasSuffix(rows[1], "▖")
		for _, r := range rows {
			if w := ansi.StringWidth(r); w != mascotW {
				t.Fatalf("frame %d: %q is %d cells wide", f, r, w)
			}
		}
	}
	if len(seen) < 8 {
		t.Errorf("only %d different pictures in 120 frames", len(seen))
	}
	if !blinked || !armsUp || !armsDown {
		t.Errorf("blinked %v, arms up %v, arms down %v", blinked, armsUp, armsDown)
	}
}

func TestMascotsInARowDoNotMoveAsOne(t *testing.T) {
	differ := false
	for f := 1; f <= 40; f++ {
		if plainMascot(f, 0) != plainMascot(f, 1) {
			differ = true
		}
	}
	if !differ {
		t.Error("two slots always draw the same picture")
	}
	rows := mascotRow(3, 0, 3)
	if len(rows) != mascotH || ansi.StringWidth(rows[0]) != 3*mascotW+2*3 {
		t.Errorf("row = %q", rows)
	}
}

func TestTheMiniMascotBlinksAndSways(t *testing.T) {
	seen := map[string]bool{}
	for f := 0; f <= 80; f++ {
		m := ansi.Strip(miniMascot(f, 0))
		seen[m] = true
		if ansi.StringWidth(m) != 5 {
			t.Fatalf("frame %d: %q is not 5 cells wide", f, m)
		}
	}
	if len(seen) != 3 || !seen["▐▛█▜▌"] || !seen["▐███▌"] {
		t.Errorf("pictures = %v", seen)
	}
	if ansi.Strip(miniMascot(0, 0)) != "▐▛█▜▌" {
		t.Error("frame 0 must be the still picture")
	}
}

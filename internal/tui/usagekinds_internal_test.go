package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/EdrenaVosh/squad-up/internal/usage"
)

func TestTheKindBarsShareOneScaleAndSplitIntoThreeKinds(t *testing.T) {
	rep := usage.Report{Agents: []usage.Agent{
		{Name: "main", Tokens: usage.Tokens{Input: 1000, CacheRead: 8000, Output: 1000}},
		{Name: "reviewer", Tokens: usage.Tokens{Input: 500, Output: 500}},
	}}
	f := &usageFeed{}

	lines := kindBars(rep, f, 64)

	if len(lines) != 3 || !strings.Contains(ansi.Strip(lines[0]), "sent") {
		t.Fatalf("lines = %q", lines)
	}
	count := func(s string) int { return strings.Count(ansi.Strip(s), "█") }
	if count(lines[1]) <= count(lines[2]) {
		t.Errorf("main spent ten times more, yet its bar is %d cells and the reviewer's %d", count(lines[1]), count(lines[2]))
	}
	for _, l := range lines {
		if w := ansi.StringWidth(l); w > 64 {
			t.Errorf("%q is %d cells wide", ansi.Strip(l), w)
		}
	}
}

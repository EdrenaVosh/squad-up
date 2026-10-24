package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/EdrenaVosh/squadup/internal/workspace"
)

func TestGradientHasOneColorPerCell(t *testing.T) {
	for _, n := range []int{0, 1, 2, 9, 80} {
		if got := len(gradient(n)); got != n {
			t.Errorf("gradient(%d) has %d colors", n, got)
		}
	}
}

func TestGradientTextKeepsTheTextAndShineOnlyChangesColors(t *testing.T) {
	plain := gradientText("◆ squadup", 0)
	shining := gradientText("◆ squadup", 4)

	if ansi.Strip(plain) != "◆ squadup" || ansi.Strip(shining) != "◆ squadup" {
		t.Errorf("text changed: %q %q", ansi.Strip(plain), ansi.Strip(shining))
	}
	if plain == shining {
		t.Error("a shine at 4 changes nothing")
	}
}

func TestTheShineSweepsThenRests(t *testing.T) {
	prev := -100.0
	for frame := 0; frame <= 10; frame++ {
		pos := shineAt(frame, 10)
		if pos <= prev {
			t.Errorf("the shine stands still or goes back at frame %d: %v after %v", frame, pos, prev)
		}
		prev = pos
	}
	if got := shineAt(40, 10); got != 0 {
		t.Errorf("the shine should rest at frame 40, got %v", got)
	}
	if got := shineAt(50, 10); got == 0 {
		t.Error("the shine should start again every 50 frames")
	}
}

func TestPulseBreathesBetweenTwoColors(t *testing.T) {
	a, b := brandStops[0], brandStops[1]
	r0, _, _, _ := pulse(0, a, b).RGBA()
	ar, _, _, _ := a.RGBA()
	if r0 != ar {
		t.Errorf("frame 0 should be the first color")
	}
	seen := map[uint32]bool{}
	for f := 0; f < 60; f++ {
		_, g, _, _ := pulse(f, a, b).RGBA()
		seen[g] = true
	}
	if len(seen) < 5 {
		t.Errorf("the pulse only visits %d shades", len(seen))
	}
}

func TestEffortBarsRiseWithTheLevel(t *testing.T) {
	want := map[string]string{
		"low": "▂▁▁▁▁ low", "medium": "▂▃▁▁▁ medium", "high": "▂▃▅▁▁ high", "xhigh": "▂▃▅▆▁ xhigh", "max": "▂▃▅▆█ max", "": "▁▁▁▁▁ —", "weird": "▁▁▁▁▁ —",
	}
	for level, text := range want {
		if got := ansi.Strip(effortBars(level)); got != text {
			t.Errorf("effortBars(%q) = %q, want %q", level, got, text)
		}
	}
}

func TestPermissionStrengthGrowsAndShortensLongModes(t *testing.T) {
	for mode, text := range map[string]string{
		"plan": "▰▱▱▱▱ plan", "": "▰▰▱▱▱ default", "acceptEdits": "▰▰▰▱▱ edits", "auto": "▰▰▰▰▱ auto", "bypassPermissions": "▰▰▰▰▰ bypass",
	} {
		if got := ansi.Strip(permissionStrength(mode)); got != text {
			t.Errorf("permissionStrength(%q) = %q, want %q", mode, got, text)
		}
	}
}

func TestSpinnerCyclesThroughTenFrames(t *testing.T) {
	seen := map[string]bool{}
	for f := 0; f < 10; f++ {
		seen[ansi.Strip(spinner(f))] = true
	}
	if len(seen) != 10 || ansi.Strip(spinner(0)) != ansi.Strip(spinner(10)) {
		t.Errorf("frames = %v", seen)
	}
}

func TestRevealStartsAtTheFirstDrawAndEndsWhenAnimationIsOff(t *testing.T) {
	var r reveal
	if got := r.frames(viewCtx{anim: false, frame: 99}); got < 1000 {
		t.Errorf("without animation the reveal is done at once, got %d", got)
	}
	if got := r.frames(viewCtx{anim: true, frame: 10}); got != 0 {
		t.Errorf("first draw = %d, want 0", got)
	}
	if got := r.frames(viewCtx{anim: true, frame: 14}); got != 4 {
		t.Errorf("later draw = %d, want 4", got)
	}
	r.restart()
	if got := r.frames(viewCtx{anim: true, frame: 20}); got != 0 {
		t.Errorf("after restart = %d, want 0", got)
	}
}

func TestTypewriterTypesSixCharactersPerFrame(t *testing.T) {
	text := "A debugger and a tester"
	if got := typewriter(text, 2); got != "A debugger a▍" {
		t.Errorf("frame 2 = %q", got)
	}
	if got := typewriter(text, 99); got != text {
		t.Errorf("finished text = %q", got)
	}
}

func agents(n int) []workspace.AgentInfo {
	out := make([]workspace.AgentInfo, n)
	for i := range out {
		out[i] = workspace.AgentInfo{Name: string(rune('a' + i)), Model: "opus", Effort: "high"}
	}
	return out
}

func TestRosterDealsTilesInOneAfterAnother(t *testing.T) {
	count := func(frames int) (placeholders, full int) {
		text := ansi.Strip(strings.Join(rosterLines(agents(3), 55, 20, frames), "\n"))
		return strings.Count(text, "· · ·"), strings.Count(text, "▂▃▅")
	}
	if p, f := count(0); p != 2 || f != 0 {
		t.Errorf("frame 0: %d placeholders, %d full tiles", p, f)
	}
	if p, f := count(3); p != 1 || f != 1 {
		t.Errorf("frame 3: %d placeholders, %d full tiles", p, f)
	}
	if p, f := count(100); p != 0 || f != 3 {
		t.Errorf("done: %d placeholders, %d full tiles", p, f)
	}
}

func TestRosterFallsBackToAListWhenTilesDoNotFit(t *testing.T) {
	lines := rosterLines(agents(9), 55, 8, 100)
	if len(lines) > 8 {
		t.Fatalf("%d lines in 8", len(lines))
	}
	text := ansi.Strip(strings.Join(lines, "\n"))
	if strings.Contains(text, "╭") || !strings.Contains(text, "more") {
		t.Errorf("expected a compact list with a 'more' line:\n%s", text)
	}
}

func TestRosterWrapsTilesIntoRows(t *testing.T) {
	text := ansi.Strip(strings.Join(rosterLines(agents(4), 55, 20, 100), "\n"))
	if strings.Count(text, "╭") != 4 || strings.Count(text, "\n") != 2*tileH {
		t.Errorf("4 agents at 3 per row should make 2 rows:\n%s", text)
	}
}

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

func TestGradientTextKeepsTheTextAndFlowsWithTheShift(t *testing.T) {
	a, b := gradientText("◆ squadup", 0), gradientText("◆ squadup", 12)

	if ansi.Strip(a) != "◆ squadup" || ansi.Strip(b) != "◆ squadup" {
		t.Errorf("text changed: %q %q", ansi.Strip(a), ansi.Strip(b))
	}
	if a == b {
		t.Error("a shift of 12 changes nothing")
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
		"low": "▂···· low", "medium": "▂▃··· medium", "high": "▂▃▅·· high", "xhigh": "▂▃▅▆· xhigh", "max": "▂▃▅▆█ max", "": "····· —", "weird": "····· —",
	}
	for level, text := range want {
		if got := ansi.Strip(effortBars(level)); got != text {
			t.Errorf("effortBars(%q) = %q, want %q", level, got, text)
		}
	}
}

func TestPermissionStrengthGrowsAndShortensLongModes(t *testing.T) {
	for mode, text := range map[string]string{
		"plan": "■□□□□ plan", "": "■■□□□ default", "acceptEdits": "■■■□□ edits", "auto": "■■■■□ auto", "bypassPermissions": "■■■■■ bypass",
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
		lines, _, _ := rosterLines(agents(3), 55, 20, rosterOpts{frames: frames, selected: -1})
		text := ansi.Strip(strings.Join(lines, "\n"))
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
	lines, _, _ := rosterLines(agents(9), 55, 8, rosterOpts{frames: 100, selected: -1})
	if len(lines) > 8 {
		t.Fatalf("%d lines in 8", len(lines))
	}
	text := ansi.Strip(strings.Join(lines, "\n"))
	if strings.Contains(text, "╭") || !strings.Contains(text, "more") {
		t.Errorf("expected a compact list with a 'more' line:\n%s", text)
	}
}

func TestRosterWrapsTilesIntoRows(t *testing.T) {
	lines, _, _ := rosterLines(agents(4), 55, 20, rosterOpts{frames: 100, selected: -1})
	text := ansi.Strip(strings.Join(lines, "\n"))
	if strings.Count(text, "╭") != 4 || strings.Count(text, "\n") != 2*smallTileH {
		t.Errorf("4 agents at 3 per row should make 2 rows:\n%s", text)
	}
}

func TestTheBigLogoIsMadeOfBlocksAndKeepsItsShape(t *testing.T) {
	for _, tall := range []bool{false, true} {
		lines := bigLogo(tall, 0)
		want := 3
		if tall {
			want = 5
		}
		if len(lines) != want {
			t.Fatalf("tall=%v: %d rows, want %d", tall, len(lines), want)
		}
		for i, l := range lines {
			if w := ansi.StringWidth(l); w != bigLogoWidth(tall) {
				t.Errorf("tall=%v row %d is %d cells wide, want %d", tall, i, w, bigLogoWidth(tall))
			}
		}
		if strings.Join(lines, "") == ansi.Strip(strings.Join(lines, "")) {
			t.Errorf("tall=%v: the logo has no color", tall)
		}
	}
	if bigLogoWidth(false) != 41 || bigLogoWidth(true) != 82 {
		t.Errorf("widths = %d, %d", bigLogoWidth(false), bigLogoWidth(true))
	}
}

func TestTheBigLogoSpellsTheWord(t *testing.T) {
	plain := ansi.Strip(strings.Join(bigLogo(true, 0), "\n"))
	// The first letter, S, starts with a gap in its top-left pixel.
	first := strings.Split(plain, "\n")
	if !strings.HasPrefix(first[0], "  ██████") || !strings.HasPrefix(first[1], "██") {
		t.Errorf("the S is wrong:\n%s", plain)
	}
	if strings.Count(plain, "\n") != 4 {
		t.Errorf("expected five rows:\n%s", plain)
	}
}

func TestTheLogoColorsFlowWithTheShift(t *testing.T) {
	if strings.Join(bigLogo(false, 0), "") == strings.Join(bigLogo(false, 9), "") {
		t.Error("shifting the gradient changes nothing")
	}
	if ansi.Strip(strings.Join(bigLogo(false, 0), "")) != ansi.Strip(strings.Join(bigLogo(false, 9), "")) {
		t.Error("shifting the gradient changed the shape")
	}
}

func TestTheGradientLoopsWithoutASeam(t *testing.T) {
	a, b := flowColor(3, 20, 0), flowColor(3, 20, flowPeriod)
	ar, ag, ab, _ := a.RGBA()
	br, bg, bb, _ := b.RGBA()
	if ar != br || ag != bg || ab != bb {
		t.Error("a full period should come back to the same color")
	}
}

func TestAllTilesOfAScreenHaveTheSameSizeWhateverTheCount(t *testing.T) {
	for _, big := range []bool{false, true} {
		var w, h int
		for n := 1; n <= 7; n++ {
			lines, rects, _ := rosterLines(agents(n), 160, 40, rosterOpts{frames: 100, selected: -1, big: big})
			if len(rects) != n {
				t.Fatalf("big=%v n=%d: %d tiles", big, n, len(rects))
			}
			if n == 1 {
				w, h = rects[0].w, rects[0].h
			}
			for _, r := range rects {
				if r.w != w || r.h != h {
					t.Errorf("big=%v n=%d: tile %d is %dx%d, the first screen's tiles are %dx%d", big, n, r.index, r.w, r.h, w, h)
				}
			}
			for i, l := range lines {
				if ansi.StringWidth(l) > 160 {
					t.Errorf("big=%v n=%d: line %d is too wide", big, n, i)
				}
			}
		}
		if big && (w < bigTileW || h != bigTileH) {
			t.Errorf("a big tile is %dx%d, at least %dx%d expected", w, h, bigTileW, bigTileH)
		}
	}
}

func TestABigTileShowsTheRoleAndLabeledFacts(t *testing.T) {
	a := workspace.AgentInfo{
		Name: "planner", Color: "blue", Model: "opus", Effort: "high", PermissionMode: "plan",
		Role: "Breaks a task into steps before any code is written.", ToolNames: []string{"Read", "Grep", "Glob", "Bash"}, Tools: 4,
	}

	lines := agentTile(a, 2, tileOpts{big: true, stage: 2})
	text := ansi.Strip(strings.Join(lines, "\n"))

	if len(lines) != bigTileH {
		t.Errorf("%d lines, want %d", len(lines), bigTileH)
	}
	for _, l := range lines {
		if ansi.StringWidth(l) != bigTileW {
			t.Errorf("width %d: %q", ansi.StringWidth(l), l)
		}
	}
	for _, want := range []string{"planner", "Breaks a task into steps", "model", "opus", "effort", "access", "tools", "‹Read›", "+"} {
		if !strings.Contains(text, want) {
			t.Errorf("the tile should show %q:\n%s", want, text)
		}
	}
}

func TestToolsThatDoNotFitBecomeACount(t *testing.T) {
	a := workspace.AgentInfo{ToolNames: []string{"Read", "Write", "Edit", "Bash", "Grep", "Glob", "WebFetch"}}
	got := ansi.Strip(toolsLine(a, 20))
	if ansi.StringWidth(got) > 20 || !strings.Contains(got, "+") || !strings.HasPrefix(got, "‹Read›") {
		t.Errorf("toolsLine = %q", got)
	}
	if got := ansi.Strip(toolsLine(workspace.AgentInfo{}, 20)); got != "all tools" {
		t.Errorf("no tools = %q", got)
	}
}

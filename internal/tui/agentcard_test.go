package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

var reviewerCard = AgentCardInfo{
	Name: "reviewer", Preset: "solo", Color: "green",
	Role:  "Reviews the current diff for bugs, missing tests and unclear code.",
	Model: "sonnet", Effort: "high", Tools: "Read, Grep, Glob, Bash", PermissionMode: "acceptEdits",
}

func TestAgentCardPlainTextHasNoEscapeCodes(t *testing.T) {
	got := RenderAgentCard(reviewerCard, false)

	if got != ansi.Strip(got) {
		t.Errorf("plain card contains escape codes:\n%q", got)
	}
	want := `╭──────────────────────────────────────────╮
│ ■ reviewer                          solo │
│                                          │
│ Reviews the current diff for bugs,       │
│ missing tests and unclear code.          │
│                                          │
│ Model        sonnet                      │
│ Effort       high                        │
│ Tools        Read, Grep, Glob, Bash      │
│ Permissions  ▰▰▰▱▱ acceptEdits           │
╰──────────────────────────────────────────╯
`
	if got != want {
		t.Errorf("card:\n%s\nwant:\n%s", got, want)
	}
}

func TestAgentCardEveryLineHasTheSameWidth(t *testing.T) {
	for _, plain := range []bool{false, true} {
		for _, line := range strings.Split(strings.TrimRight(RenderAgentCard(reviewerCard, !plain), "\n"), "\n") {
			if w := ansi.StringWidth(line); w != 44 {
				t.Errorf("width %d: %q", w, line)
			}
		}
	}
}

func TestAgentCardColorsTheFrameWithTheAgentColor(t *testing.T) {
	if got := RenderAgentCard(reviewerCard, true); got == ansi.Strip(got) {
		t.Error("a colored card has no color")
	}
}

func TestAgentCardFillsGapsWithDefaults(t *testing.T) {
	got := RenderAgentCard(AgentCardInfo{Name: "scout"}, false)
	for _, want := range []string{"scout", "inherit", "all tools", "▰▰▱▱▱ default"} {
		if !strings.Contains(got, want) {
			t.Errorf("card should show %q:\n%s", want, got)
		}
	}
}

func TestPermissionStrengthGrowsWithFreedom(t *testing.T) {
	order := []string{"plan", "default", "acceptEdits", "auto", "bypassPermissions"}
	for i, mode := range order {
		bar := strings.Count(permissionBar(mode), "▰")
		if bar != i+1 {
			t.Errorf("%s has %d filled blocks, want %d", mode, bar, i+1)
		}
	}
}

func TestAgentCardTruncatesLongTools(t *testing.T) {
	c := reviewerCard
	c.Tools = strings.Repeat("Read, ", 30)
	for _, line := range strings.Split(strings.TrimRight(RenderAgentCard(c, false), "\n"), "\n") {
		if ansi.StringWidth(line) != 44 {
			t.Errorf("width %d: %q", ansi.StringWidth(line), line)
		}
	}
}

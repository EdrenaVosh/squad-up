package tui

import (
	"image/color"
	"math"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ---- palette ----

// The brand gradient runs amber → pink → violet → blue. It colors the
// wordmark, the frame edges and progress; everything else stays quiet.
var (
	brandStops = []color.Color{
		lipgloss.Color("#FFB000"),
		lipgloss.Color("#FF4D8D"),
		lipgloss.Color("#8E4EC6"),
		lipgloss.Color("#3E9BFF"),
	}
	cDim = lipgloss.Color("#5B6068")
)

// modelColors tint the model pills: one hue per family, so a glance at a
// roster tells which agents are on the big model.
var modelColors = map[string]color.Color{
	"fable":  lipgloss.Color("#D6409F"),
	"opus":   lipgloss.Color("#8E4EC6"),
	"sonnet": lipgloss.Color("#3E63DD"),
	"haiku":  lipgloss.Color("#30A46C"),
}

// gradient returns n colors along the brand gradient.
func gradient(n int) []color.Color {
	if n < 1 {
		return nil
	}
	if n == 1 {
		return brandStops[:1]
	}
	return lipgloss.Blend1D(n, brandStops...)
}

// flowPeriod is how many colors one loop of the flowing gradient has.
const flowPeriod = 72

// flowCycle is the brand gradient there and back, so that it loops without a
// seam: amber → pink → violet → blue → violet → pink → amber.
var flowCycle = lipgloss.Blend1D(flowPeriod,
	brandStops[0], brandStops[1], brandStops[2], brandStops[3], brandStops[2], brandStops[1], brandStops[0])

// flowColor is the color of cell i of a run of n cells, a loop of the gradient
// spread over the run, moved along by shift. Counting shift up makes the
// colors flow.
func flowColor(i, n, shift int) color.Color {
	return flowCycle[((i*flowPeriod/max(n, 1)+shift)%flowPeriod+flowPeriod)%flowPeriod]
}

// gradientText colors each cell of s along the flowing gradient.
func gradientText(s string, shift int) string {
	runes := []rune(s)
	var b strings.Builder
	for i, r := range runes {
		b.WriteString(lipgloss.NewStyle().Foreground(flowColor(i, len(runes), shift)).Bold(true).Render(string(r)))
	}
	return b.String()
}

// blend mixes two colors; t = 0 is a, t = 1 is b.
func blend(a, b color.Color, t float64) color.Color {
	ar, ag, ab, _ := a.RGBA()
	br, bg, bb, _ := b.RGBA()
	mix := func(x, y uint32) uint8 { return uint8((float64(x>>8)*(1-t) + float64(y>>8)*t)) }
	return color.RGBA{mix(ar, br), mix(ag, bg), mix(ab, bb), 0xFF}
}

// pulse moves between a and b with the frame, slowly, like breathing.
func pulse(frame int, a, b color.Color) color.Color {
	t := (1 - math.Cos(float64(frame)/9)) / 2
	return blend(a, b, t)
}

// ---- small widgets ----

// keycap draws a key as a small inverted cap.
func keycap(key string) string {
	return lipgloss.NewStyle().Foreground(cMuted).Reverse(true).Render(" " + key + " ")
}

// selBar is the bar in front of the selected row.
func selBar() string { return fg(brandStops[0]).Render("▌") }

// pill is a short label on a colored ground.
func pill(text string, c color.Color) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Background(c).Bold(true).Render(" " + text + " ")
}

// modelPill is the pill of a model alias, tinted by its family.
func modelPill(model string) string {
	name := orDash(model, "inherit")
	for family, c := range modelColors {
		if strings.Contains(name, family) {
			return pill(name, c)
		}
	}
	return sMuted.Render("[" + name + "]")
}

// Sharing the squad's notes is teal, working in isolation is amber.
var (
	cShared   = lipgloss.Color("#2DD4BF")
	cIsolated = lipgloss.Color("#E5A50A")
)

// sharePill is the state of an agent's context as a colored pill.
func sharePill(shared bool) string {
	if shared {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#06201C")).Background(cShared).Bold(true).Render(" ⇄ shared ")
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#2A1B00")).Background(cIsolated).Bold(true).Render(" ⊘ isolated ")
}

// shareChip is the short mark of an agent's context for a tile.
func shareChip(shared bool) string {
	if shared {
		return fg(cShared).Render("⇄ shared")
	}
	return fg(cIsolated).Render("⊘ isolated")
}

// effortSteps is how many bars an effort level lights up, out of 5.
var effortSteps = map[string]int{"low": 1, "medium": 2, "high": 3, "xhigh": 4, "max": 5}

// effortBars draws the effort as rising bars, with the word after them.
func effortBars(level string) string {
	n, ok := effortSteps[level]
	if !ok {
		return sMuted.Render("····· —")
	}
	heights := []string{"▂", "▃", "▅", "▆", "█"}
	var b strings.Builder
	for i, h := range heights {
		if i < n {
			b.WriteString(fg(brandStops[min(i*len(brandStops)/len(heights), len(brandStops)-1)]).Render(h))
		} else {
			b.WriteString(fg(cDim).Render("·"))
		}
	}
	return b.String() + " " + level
}

// permissionColors goes from calm to loud with the strength of a mode.
var permissionColors = []color.Color{
	lipgloss.Color("#30A46C"), lipgloss.Color("#30A46C"), lipgloss.Color("#E5A50A"), lipgloss.Color("#F76B15"), lipgloss.Color("#E5484D"),
}

// permissionStrength bars a permission mode: green when it asks a lot, red
// when it asks nothing.
func permissionStrength(mode string) string {
	if mode == "" {
		mode = "default"
	}
	level, ok := permissionLevels[mode]
	if !ok {
		level = 2
	}
	c := permissionColors[level-1]
	return fg(c).Render(strings.Repeat("■", level)) + fg(cDim).Render(strings.Repeat("□", 5-level)) + " " + shortMode(mode)
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// spinner is a braille spinner colored along the gradient.
func spinner(frame int) string {
	i := frame % len(spinnerFrames)
	return fg(gradient(len(spinnerFrames))[i]).Render(spinnerFrames[i])
}

// ---- the frame clock ----

// frameInterval is the time between animation frames.
const frameInterval = 80 * time.Millisecond

// tickMsg advances the animation by one frame.
type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(frameInterval, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// reveal times an animation that starts when a screen first draws it. When
// animation is off, it is always finished.
type reveal struct {
	start   int
	started bool
}

func (r *reveal) restart() { r.started = false }

// frames is how many frames have passed since the start, or a large number
// when animation is off.
func (r *reveal) frames(c viewCtx) int {
	if !c.anim {
		return 1 << 20
	}
	if !r.started {
		r.start, r.started = c.frame, true
	}
	return c.frame - r.start
}

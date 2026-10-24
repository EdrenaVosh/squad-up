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

// gradientText colors each cell of s along the gradient. A positive shine
// sweeps a bright band across it: shine is the band's position in cells.
func gradientText(s string, shine float64) string {
	runes := []rune(s)
	colors := gradient(len(runes))
	var b strings.Builder
	for i, r := range runes {
		c := colors[i]
		if shine > 0 {
			if d := math.Abs(float64(i) - shine); d < 3 {
				c = lipgloss.Lighten(c, 0.55*(1-d/3))
			}
		}
		b.WriteString(lipgloss.NewStyle().Foreground(c).Bold(true).Render(string(r)))
	}
	return b.String()
}

// shineAt is where the highlight of a wordmark of n cells is at a frame, or 0
// while it rests. It sweeps once every 4 seconds.
func shineAt(frame, n int) float64 {
	const period = 50 // frames: 4 s at 80 ms
	t := frame % period
	if t > n+8 {
		return 0
	}
	return float64(t) - 4
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

// effortSteps is how many bars an effort level lights up, out of 5.
var effortSteps = map[string]int{"low": 1, "medium": 2, "high": 3, "xhigh": 4, "max": 5}

// effortBars draws the effort as rising bars, with the word after them.
func effortBars(level string) string {
	n, ok := effortSteps[level]
	if !ok {
		return sMuted.Render("▁▁▁▁▁ —")
	}
	heights := []string{"▂", "▃", "▅", "▆", "█"}
	var b strings.Builder
	for i, h := range heights {
		if i < n {
			b.WriteString(fg(brandStops[min(i*len(brandStops)/len(heights), len(brandStops)-1)]).Render(h))
		} else {
			b.WriteString(fg(cDim).Render("▁"))
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
	return fg(c).Render(strings.Repeat("▰", level)) + fg(cDim).Render(strings.Repeat("▱", 5-level)) + " " + shortMode(mode)
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

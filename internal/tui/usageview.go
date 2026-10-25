package tui

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/EdrenaVosh/squadup/internal/usage"
)

// usageScreen is the live report of the tokens each agent spends.
type usageScreen struct{ feed *usageFeed }

func newUsageScreen(feed *usageFeed) *usageScreen { return &usageScreen{feed: feed} }

func (s *usageScreen) Init() tea.Cmd { return nil }

func (s *usageScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return s, nil
	}
	switch key.String() {
	case "esc", "q", "left":
		return s, pop()
	case "?":
		return s, push(newHelp())
	case "z":
		if s.feed.zero() {
			return s, flash("Counting from now: the numbers start at zero. z goes back to the whole session")
		}
		return s, flash("Counting the whole session again")
	}
	return s, nil
}

func (s *usageScreen) Hints() []hint {
	return []hint{{"z", "count from now"}, {"esc", "back"}, {"?", "help"}}
}

// The columns of the table.
const (
	uName, uState, uRuns, uNum = 16, 15, 6, 8
)

func (s *usageScreen) View(c viewCtx) string {
	f := c.usage
	rep := f.shown()
	title := "Usage"
	right := shorten(f.cwd, 40)
	if rep.Found {
		right = fmt.Sprintf("session %s · %s", shortID(rep.Session), rep.Updated.Local().Format("15:04:05"))
	}

	var lines []string
	add := func(l string) { lines = append(lines, l) }
	inner := c.w - 2

	if !rep.Found {
		add("")
		for _, l := range mascot(0, 0) {
			add(center(l, inner))
		}
		add("")
		add(center(sBold.Render("No Claude Code session in this folder yet"), inner))
		add(center(sMuted.Render(shorten(f.cwd, inner-4)), inner))
		add("")
		add(center(sMuted.Render("Start `squad run <preset>` (or claude) here, and the tokens of every agent show up as it works."), inner))
		return c.box(title, right, lines, 0)
	}

	live := usageLive(rep, c)
	add(" " + live + sMuted.Render("  "+tokensShort(rep.Total.Total())+" tokens so far"+baselineNote(f)))
	add("")
	add(" " + sMuted.Render(pad("AGENT", uName+2)+pad("STATE", uState)+pad("RUNS", uRuns)+pad("SENT", uNum)+pad("CACHED", uNum)+pad("OUT", uNum)+pad("TOTAL", uNum+1)+"SHARE"))
	add(" " + fg(cDim).Render(strings.Repeat("─", max(inner-2, 0))))
	barW := max(inner-2-(uName+2+uState+uRuns+uNum*4+1)-6, 8)
	for i, a := range rep.Agents {
		add(" " + agentRow(a, i, rep, f, barW, c))
	}
	add(" " + fg(cDim).Render(strings.Repeat("─", max(inner-2, 0))))
	add(" " + totalRow(rep, f, barW))
	add("")
	add(" " + sMuted.Render(pad("PACE", uName+2)) + fg(brandStops[0]).Render(strings.TrimLeft(sparkline(f.history, min(60, inner-40)), " ")) +
		sMuted.Render(fmt.Sprintf("  %s tokens in the last minute", tokensShort(f.perMinute()))))
	add(" " + sMuted.Render(pad("CACHE HIT", uName+2)) + cacheBar(rep.Total, 20) + sMuted.Render(fmt.Sprintf("  %d%% of the input was read from the cache", cachePercent(rep.Total))))
	add("")
	for _, l := range wrapText(sMuted.Render("SENT is fresh input plus what was written to the cache; CACHED is what was read back from it (cheap, so it is left out of TOTAL). TOTAL is SENT plus OUT, the way Claude Code counts a run. Numbers come from the session's own transcripts, not from an estimate."), inner-2) {
		add(" " + l)
	}
	return c.box(title, right, lines, 0)
}

func baselineNote(f *usageFeed) string {
	if f.base != nil {
		return " · counting from the mark (z)"
	}
	return ""
}

// usageLive is the headline: how many agents are working right now.
func usageLive(rep usage.Report, c viewCtx) string {
	working := 0
	for _, a := range rep.Agents {
		working += a.Running
	}
	mainActive := false
	for _, a := range rep.Agents {
		if a.Name == usage.MainName && rep.Updated.Sub(a.Last) < 20*time.Second {
			mainActive = true
		}
	}
	switch {
	case working > 0:
		return miniMascot(animFrame(c), 0) + " " + fg(cClaude).Bold(true).Render(fmt.Sprintf("%s working", plural(working, "agent")))
	case mainActive:
		return miniMascot(animFrame(c), 0) + " " + fg(cClaude).Bold(true).Render("the session is working")
	}
	return sMuted.Render("◦ idle")
}

// animFrame is the frame of the animation, or 0 when it is off.
func animFrame(c viewCtx) int {
	if !c.anim {
		return 0
	}
	return c.frame
}

func agentRow(a usage.Agent, i int, rep usage.Report, f *usageFeed, barW int, c viewCtx) string {
	tint := f.tint(a.Name)
	name := fg(tint).Render("■") + " " + sBold.Render(truncate(a.Name, uName))
	state := sMuted.Render("idle")
	switch {
	case a.Running > 0:
		state = miniMascot(animFrame(c), i) + fg(cClaude).Render(" working")
	case a.Name == usage.MainName && rep.Updated.Sub(a.Last) < 20*time.Second:
		state = miniMascot(animFrame(c), i) + fg(cClaude).Render(" active")
	case a.Name != usage.MainName && a.Runs > 0:
		state = sMuted.Render("done")
	}
	runs := sMuted.Render("—")
	if a.Name != usage.MainName {
		runs = fmt.Sprintf("%d", a.Runs)
	}
	t := a.Tokens
	share := 0.0
	if rep.Total.Total() > 0 {
		share = float64(t.Total()) / float64(rep.Total.Total())
	}
	return pad(name, uName+2) + pad(state, uState) + pad(runs, uRuns) +
		pad(tokensShort(t.Input+t.CacheWrite), uNum) + pad(sMuted.Render(tokensShort(t.CacheRead)), uNum) + pad(tokensShort(t.Output), uNum) +
		pad(sBold.Render(tokensShort(t.Total())), uNum+1) + shareBar(share, barW, tint) + sMuted.Render(fmt.Sprintf(" %3d%%", int(share*100+0.5)))
}

// totalRow is the whole squad: the sums, and one bar made of every agent's share.
func totalRow(rep usage.Report, f *usageFeed, barW int) string {
	t := rep.Total
	var bar strings.Builder
	used := 0
	for i, a := range rep.Agents {
		n := 0
		if t.Total() > 0 {
			n = a.Tokens.Total() * barW / t.Total()
		}
		if i == len(rep.Agents)-1 {
			n = barW - used
		}
		n = max(min(n, barW-used), 0)
		bar.WriteString(fg(f.tint(a.Name)).Render(strings.Repeat("█", n)))
		used += n
	}
	if used < barW {
		bar.WriteString(fg(cDim).Render(strings.Repeat("░", barW-used)))
	}
	return pad(sBold.Render("SQUAD"), uName+2) + pad("", uState) + pad(sMuted.Render(fmt.Sprintf("%d", subRuns(rep))), uRuns) +
		pad(sBold.Render(tokensShort(t.Input+t.CacheWrite)), uNum) + pad(sMuted.Render(tokensShort(t.CacheRead)), uNum) + pad(sBold.Render(tokensShort(t.Output)), uNum) +
		pad(fg(brandStops[0]).Bold(true).Render(tokensShort(t.Total())), uNum+1) + bar.String() + sMuted.Render(" 100%")
}

func subRuns(rep usage.Report) int {
	n := 0
	for _, a := range rep.Agents {
		n += a.Runs
	}
	return n
}

// shareBar is a bar filled to a fraction, in the agent's color.
func shareBar(fraction float64, w int, c color.Color) string {
	filled := int(fraction*float64(w) + 0.5)
	if fraction > 0 {
		filled = max(filled, 1)
	}
	filled = min(filled, w)
	return fg(c).Render(strings.Repeat("█", filled)) + fg(cDim).Render(strings.Repeat("░", w-filled))
}

func cachePercent(t usage.Tokens) int {
	in := t.Input + t.CacheWrite + t.CacheRead
	if in == 0 {
		return 0
	}
	return t.CacheRead * 100 / in
}

func cacheBar(t usage.Tokens, w int) string {
	return shareBar(float64(cachePercent(t))/100, w, cShared)
}

// shortID is the first part of a session id.
func shortID(id string) string {
	if i := strings.IndexByte(id, '-'); i > 0 {
		return id[:i]
	}
	return truncate(id, 8)
}

// shorten cuts a path from the left, so the end of it stays.
func shorten(s string, w int) string {
	r := []rune(s)
	if len(r) <= w {
		return s
	}
	return "…" + string(r[len(r)-w+1:])
}

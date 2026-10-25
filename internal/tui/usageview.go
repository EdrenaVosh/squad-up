package tui

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/EdrenaVosh/squadup/internal/usage"
)

// usageScreen is the live report of the tokens each agent spends, session by
// session: the sessions of the folder in a list, the selected one in a table.
// The list follows the disk, so a session that starts appears and one that is
// deleted goes away while the screen is open.
type usageScreen struct {
	feed     *usageFeed
	selected string // the id of the selected session; it stays put when the list changes
}

func newUsageScreen(feed *usageFeed) *usageScreen { return &usageScreen{feed: feed} }

func (s *usageScreen) Init() tea.Cmd { return nil }

// current is the selected session, or the newest when that one is gone.
func (s *usageScreen) current() *sessionView {
	if v := s.feed.find(s.selected); v != nil {
		return v
	}
	v := s.feed.latest()
	if v != nil {
		s.selected = v.id
	}
	return v
}

func (s *usageScreen) move(delta int) {
	v := s.current()
	if v == nil {
		return
	}
	for i, w := range s.feed.sessions {
		if w == v {
			s.selected = s.feed.sessions[min(max(i+delta, 0), len(s.feed.sessions)-1)].id
			return
		}
	}
}

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
	case "up", "k":
		s.move(-1)
	case "down", "j":
		s.move(1)
	case "z":
		if v := s.current(); v != nil {
			if v.zero() {
				return s, flash("Counting this session from now: the numbers start at zero. z goes back to all of it")
			}
			return s, flash("Counting the whole session again")
		}
	}
	return s, nil
}

func (s *usageScreen) Hints() []hint {
	return []hint{{"↑↓", "session"}, {"z", "count from now"}, {"esc", "back"}, {"?", "help"}}
}

// The columns of the table.
const (
	uName, uState, uRuns, uNum = 14, 14, 5, 7
	uTable                     = uName + 2 + uState + uRuns + uNum*3 + uNum + 1 // without the share bar
	uListW                     = 38
	uSideBySide                = uListW + 1 + uTable + 10 // the least inner width for the list beside the table
)

func (s *usageScreen) View(c viewCtx) string {
	f := c.usage
	v := s.current()
	inner := c.w - 2

	right := fmt.Sprintf("%s in this folder", plural(len(f.sessions), "session"))
	if v == nil {
		var lines []string
		add := func(l string) { lines = append(lines, l) }
		add("")
		for _, l := range mascot(0, 0) {
			add(center(l, inner))
		}
		add("")
		add(center(sBold.Render("No Claude Code session in this folder yet"), inner))
		add(center(sMuted.Render(shorten(tildePath(f.cwd), inner-4)), inner))
		add("")
		add(center(sMuted.Render("Start `squad run <preset>` (or claude) here, and its sessions show up here as they begin."), inner))
		return c.box("Usage", right, lines, 0)
	}

	if inner >= uSideBySide {
		left := s.sessionList(c, uListW, len(f.sessions)*3+2)
		detail := s.detail(c, v, inner-uListW-1)
		lines := panes(left, detail, uListW, inner)
		h := min(c.h, max(len(lines)+3, 22))
		return frame("Usage", c.shift(), right, lines, c.w, h, uListW)
	}
	// A narrow window: the sessions in a short list above the table.
	list := s.sessionList(c, inner, 3*4)
	lines := append(list, " "+fg(cDim).Render(strings.Repeat("─", max(inner-2, 0))))
	lines = append(lines, s.detail(c, v, inner)...)
	return c.box("Usage", right, lines, 0)
}

// sessionList is the sessions, three lines each: when it began and how much it
// has spent, and what it was first asked. The selected one has a bar.
func (s *usageScreen) sessionList(c viewCtx, w, room int) []string {
	f := c.usage
	lines := []string{" " + sMuted.Render("SESSIONS"), ""}
	selected := s.current()
	rows := max((room-len(lines))/3, 1)
	start := 0
	for i, v := range f.sessions {
		if v == selected && i >= rows {
			start = i - rows + 1
		}
	}
	for i := start; i < len(f.sessions) && i < start+rows; i++ {
		v := f.sessions[i]
		bar, name := "  ", sPlain
		if v == selected {
			bar, name = selBar()+" ", sBold
		}
		mark := sMuted.Render("◦")
		switch {
		case v.working() > 0 || c.usage.now().Sub(v.last) < 20*time.Second:
			mark = miniMascot(animFrame(c), i)
		case c.usage.now().Sub(v.last) < activeWindow:
			mark = fg(cClaude).Render("●")
		}
		total := v.shown().Total.Total()
		first := name.Render(whenText(v, f.now())) + sMuted.Render("  "+tokensShort(total)+" tokens")
		lines = append(lines, truncate(" "+bar+mark+" "+first, w))
		prompt := v.head.Prompt
		if prompt == "" {
			prompt = "…"
		}
		lines = append(lines, truncate("     "+sMuted.Render(truncate(prompt, w-7)), w), "")
	}
	if start+rows < len(f.sessions) {
		lines = append(lines, " "+sMuted.Render(fmt.Sprintf("↓ %d more", len(f.sessions)-start-rows)))
	}
	return lines
}

// whenText says when a session began: the time today, or the date.
func whenText(v *sessionView, now time.Time) string {
	at := v.head.Started
	if at.IsZero() {
		at = v.last
	}
	at, now = at.Local(), now.Local()
	if at.YearDay() == now.YearDay() && at.Year() == now.Year() {
		return at.Format("15:04")
	}
	return at.Format("Jan 2 15:04")
}

// detail is the table of the selected session.
func (s *usageScreen) detail(c viewCtx, v *sessionView, w int) []string {
	f := c.usage
	rep := v.shown()
	var lines []string
	add := func(l string) { lines = append(lines, l) }

	add(" " + usageLive(rep, c) + sMuted.Render("  "+tokensShort(rep.Total.Total())+" tokens"+baselineNote(v)))
	add(" " + sMuted.Render(fmt.Sprintf("began %s · last activity %s", whenText(v, f.now()), ago(f.now().Sub(v.last)))))
	if v.head.Prompt != "" {
		for _, l := range wrapStyled(v.head.Prompt, sMuted, w-3) {
			add("  " + l)
		}
	}
	add("")
	add(" " + sMuted.Render(pad("AGENT", uName+2)+pad("STATE", uState)+pad("RUNS", uRuns)+pad("SENT", uNum)+pad("CACHED", uNum)+pad("OUT", uNum)+pad("TOTAL", uNum+1)+"SHARE"))
	add(" " + fg(cDim).Render(strings.Repeat("─", max(w-2, 0))))
	barW := max(w-2-uTable-6, 6)
	for i, a := range rep.Agents {
		add(" " + agentRow(a, i, rep, f, barW, c))
	}
	add(" " + fg(cDim).Render(strings.Repeat("─", max(w-2, 0))))
	add(" " + totalRow(rep, f, barW))
	add("")
	add(" " + sMuted.Render(pad("CACHE HIT", uName+2)) + cacheBar(rep.Total, 16) + sMuted.Render(fmt.Sprintf("  %d%% of the input came from the cache", cachePercent(rep.Total))))
	add("")
	for _, l := range wrapStyled("SENT is fresh input plus cache writes; CACHED is read back from the cache (cheap, so it is not in TOTAL); TOTAL is SENT plus OUT, as Claude Code counts a run. The numbers come from the session's own transcripts, not from an estimate.", sMuted, w-3) {
		add(" " + l)
	}
	return lines
}

// ago says how long ago in a few words.
func ago(d time.Duration) string {
	switch {
	case d < 5*time.Second:
		return "just now"
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%d h ago", int(d.Hours()))
	}
	return fmt.Sprintf("%d days ago", int(d.Hours()/24))
}

func baselineNote(v *sessionView) string {
	if v.base != nil {
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

// shorten cuts a path from the left, so the end of it stays.
func shorten(s string, w int) string {
	r := []rune(s)
	if len(r) <= w {
		return s
	}
	return "…" + string(r[len(r)-w+1:])
}

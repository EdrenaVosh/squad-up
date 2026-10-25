package tui

import (
	"image/color"
	"time"

	"github.com/EdrenaVosh/squadup/internal/usage"
	"github.com/EdrenaVosh/squadup/internal/workspace"
)

// usageInterval is how often the token report is read again.
const usageInterval = time.Second

// usageFeed is the live report of the latest Claude Code session of the folder
// the interface was started in. The app refreshes it on a one-second clock; the
// screens only read it.
type usageFeed struct {
	claudeDir, cwd string
	now            func() time.Time

	watcher *usage.Watcher
	session string
	report  usage.Report
	base    map[string]usage.Tokens
	colors  map[string]string // agent name → Claude Code color
}

func newUsageFeed(claudeDir, cwd string, now func() time.Time) *usageFeed {
	f := &usageFeed{claudeDir: claudeDir, cwd: cwd, now: now}
	f.refresh()
	return f
}

// refresh reads what the session wrote since the last time. When a newer
// session has started in the folder, the feed follows it.
func (f *usageFeed) refresh() {
	session, ok := usage.LatestSession(f.claudeDir, f.cwd)
	if !ok {
		f.report = usage.Report{}
		return
	}
	if session != f.session || f.watcher == nil {
		f.session = session
		f.watcher = usage.NewWatcher(usage.ProjectDir(f.claudeDir, f.cwd), session)
		f.watcher.Now = f.now
		f.base = nil
	}
	report, err := f.watcher.Refresh()
	if err != nil {
		return
	}
	f.report = report
}

// shown is the report with the baseline taken off, when one is set.
func (f *usageFeed) shown() usage.Report {
	r := f.report
	if f.base == nil {
		return r
	}
	agents := make([]usage.Agent, len(r.Agents))
	copy(agents, r.Agents)
	r.Total = usage.Tokens{}
	for i, a := range agents {
		b := f.base[a.Name]
		a.Tokens = usage.Tokens{
			Input: max(a.Tokens.Input-b.Input, 0), CacheWrite: max(a.Tokens.CacheWrite-b.CacheWrite, 0),
			CacheRead: max(a.Tokens.CacheRead-b.CacheRead, 0), Output: max(a.Tokens.Output-b.Output, 0), Calls: max(a.Tokens.Calls-b.Calls, 0),
		}
		agents[i] = a
		r.Total.Input += a.Tokens.Input
		r.Total.CacheWrite += a.Tokens.CacheWrite
		r.Total.CacheRead += a.Tokens.CacheRead
		r.Total.Output += a.Tokens.Output
		r.Total.Calls += a.Tokens.Calls
	}
	r.Agents = agents
	return r
}

// zero starts counting from now, or goes back to counting the whole session.
func (f *usageFeed) zero() (started bool) {
	if f.base != nil {
		f.base = nil
		return false
	}
	f.base = map[string]usage.Tokens{}
	for _, a := range f.report.Agents {
		f.base[a.Name] = a.Tokens
	}
	return true
}

// setColors takes the colors of the agents of a card.
func (f *usageFeed) setColors(c workspace.Card) {
	f.colors = map[string]string{}
	for _, a := range c.Agents {
		f.colors[a.Name] = a.Color
	}
}

// tint is the color of an agent in the report: the session itself is Claude's
// orange, an agent of the equipped preset has its own color, and any other
// gets one of the eight by its name, always the same.
func (f *usageFeed) tint(name string) color.Color {
	if name == usage.MainName {
		return cClaude
	}
	if c, ok := agentColors[f.colors[name]]; ok {
		return c
	}
	sum := 0
	for _, r := range name {
		sum += int(r)
	}
	list := agentColorList()
	return list[sum%len(list)]
}

func tokensShort(n int) string { return usage.Short(n) }

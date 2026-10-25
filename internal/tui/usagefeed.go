package tui

import (
	"image/color"
	"path/filepath"
	"time"

	"github.com/EdrenaVosh/squadup/internal/usage"
	"github.com/EdrenaVosh/squadup/internal/workspace"
)

// usageInterval is how often the token report is read again.
const usageInterval = time.Second

// maxSessions is how many sessions of a folder the feed follows.
const maxSessions = 15

// activeWindow is how recently a session must have been written to for the
// main screen to call it running.
const activeWindow = 2 * time.Minute

// sessionView is one session of the folder as the feed follows it.
type sessionView struct {
	id      string
	last    time.Time
	head    usage.Head
	watcher *usage.Watcher
	report  usage.Report
	base    map[string]usage.Tokens // the counts when z was pressed
}

// usageFeed follows the newest sessions of the folder the interface was started
// in. Sessions come and go: a new one appears in the list within a second, a
// deleted one is dropped. The app refreshes the feed on a one-second clock; the
// screens only read it.
type usageFeed struct {
	claudeDir, cwd string
	now            func() time.Time

	sessions []*sessionView    // newest first
	colors   map[string]string // agent name → Claude Code color
}

func newUsageFeed(claudeDir, cwd string, now func() time.Time) *usageFeed {
	f := &usageFeed{claudeDir: claudeDir, cwd: cwd, now: now}
	f.refresh()
	return f
}

// refresh lists the sessions again and reads what each wrote since the last
// time. A session that is not on the disk any more is dropped; one that is new
// gets a watcher, and its head (when it started, what it was first asked) is
// read until there is something to show.
func (f *usageFeed) refresh() {
	listed := usage.ListSessions(f.claudeDir, f.cwd, maxSessions)
	known := map[string]*sessionView{}
	for _, v := range f.sessions {
		known[v.id] = v
	}
	dir := usage.ProjectDir(f.claudeDir, f.cwd)
	next := make([]*sessionView, 0, len(listed))
	for _, s := range listed {
		v := known[s.ID]
		if v == nil {
			v = &sessionView{id: s.ID, watcher: usage.NewWatcher(dir, s.ID)}
			v.watcher.Now = f.now
		}
		v.last = s.Last
		if v.head.Prompt == "" || v.head.Started.IsZero() {
			v.head = usage.ReadHead(filepath.Join(dir, s.ID+".jsonl"))
		}
		if report, err := v.watcher.Refresh(); err == nil {
			v.report = report
		}
		next = append(next, v)
	}
	f.sessions = next
}

// find is the session with the id, or nil.
func (f *usageFeed) find(id string) *sessionView {
	for _, v := range f.sessions {
		if v.id == id {
			return v
		}
	}
	return nil
}

// latest is the newest session, or nil when the folder has none.
func (f *usageFeed) latest() *sessionView {
	if len(f.sessions) == 0 {
		return nil
	}
	return f.sessions[0]
}

// running counts the sessions that were written to in the last two minutes.
func (f *usageFeed) running() int {
	n := 0
	for _, v := range f.sessions {
		if f.now().Sub(v.last) < activeWindow {
			n++
		}
	}
	return n
}

// working counts the subagents of the session that are running now.
func (v *sessionView) working() int {
	n := 0
	for _, a := range v.report.Agents {
		n += a.Running
	}
	return n
}

// shown is the report with the baseline taken off, when one is set.
func (v *sessionView) shown() usage.Report {
	r := v.report
	if v.base == nil {
		return r
	}
	agents := make([]usage.Agent, len(r.Agents))
	copy(agents, r.Agents)
	r.Total = usage.Tokens{}
	for i, a := range agents {
		b := v.base[a.Name]
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

// zero starts counting this session from now, or goes back to counting all of it.
func (v *sessionView) zero() (started bool) {
	if v.base != nil {
		v.base = nil
		return false
	}
	v.base = map[string]usage.Tokens{}
	for _, a := range v.report.Agents {
		v.base[a.Name] = a.Tokens
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

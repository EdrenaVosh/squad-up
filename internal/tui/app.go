// Package tui is the terminal interface of squad-up: the main screen with the
// presets, the editors, and the confirmation before a preset is equipped. It
// also holds every user-facing text (copy.go).
package tui

import (
	"context"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/EdrenaVosh/squad-up/internal/claude"
	"github.com/EdrenaVosh/squad-up/internal/initial"
	"github.com/EdrenaVosh/squad-up/internal/paths"
	"github.com/EdrenaVosh/squad-up/internal/workspace"
)

// StartKind says which screen the interface opens on.
type StartKind int

// Screens the interface can start on.
const (
	StartHome  StartKind = iota // the main screen
	StartNew                    // the dialog that creates a preset
	StartEdit                   // the editor of a preset or of one of its agents
	StartUsage                  // the live report of the tokens the agents spend
)

// Start is the screen to open first.
type Start struct {
	Kind   StartKind
	Preset string
	Agent  string // with StartEdit: the agent to edit, or "" for the preset
	Cwd    string // the folder whose Claude Code sessions the usage report follows; the working folder when empty
}

// Env is everything the interface needs from the outside.
type Env struct {
	Paths paths.Paths
	Now   func() time.Time // defaults to time.Now
	// Welcome is set on the very first run: Initial was captured just now, and
	// the interface starts with the welcome screen.
	Welcome *initial.Meta
	// ClaudeVersion is shown in the title bar; when empty it is looked up.
	ClaudeVersion string
	Start         Start
	// Animate turns the motion on: a frame clock runs and screens animate.
	// Tests and terminals that should stay still leave it off.
	Animate bool
	// Cwd is the folder whose Claude Code sessions the usage report follows;
	// the working folder when empty.
	Cwd string
	// Live reads the transcripts of the session again every second. Tests
	// leave it off and send the clock by hand.
	Live bool
}

// minimum size the interface is laid out for.
const (
	minWidth  = 80
	minHeight = 24
)

// viewCtx is what a screen needs to draw itself.
type viewCtx struct {
	w, h   int
	claude string
	frame  int  // the animation frame, advancing while Animate is on
	anim   bool // whether anything should animate
	ox, oy int  // where the top left of the screen sits in the terminal
	usage  *usageFeed
}

type hint struct{ key, label string }

// finisher is implemented by a screen that must not be shown again once
// something else has done its job: the welcome screen, after a first preset.
type finisher interface{ finished() bool }

// fullscreen is implemented by a screen that uses the whole terminal. Every
// other screen opens in a window in the middle, so that a big terminal is not
// filled with empty space.
type fullscreen interface{ fullscreen() bool }

// The window the other screens open in, at most.
const (
	windowW = 120
	windowH = 36
	// The main screen stops growing here; more space is margin.
	homeMaxW = 200
	homeMaxH = 44
)

// screen is one page of the interface; the app keeps a stack of them.
type screen interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (screen, tea.Cmd)
	// View draws exactly c.h lines of c.w cells.
	View(c viewCtx) string
	// Hints are the keys shown in the footer, most important first; the last
	// two are kept when space runs out.
	Hints() []hint
}

// Navigation and status messages.
type (
	pushMsg struct{ s screen }
	// pushAllMsg opens several screens at once, the last one on top.
	pushAllMsg struct{ screens []screen }
	popMsg     struct {
		n    int
		then tea.Cmd
	}
	flashMsg struct {
		text string
		err  bool
	}
	reloadMsg struct{}
	claudeMsg struct{ version string }
	// usageTickMsg is the clock that reads the session's transcripts again.
	usageTickMsg struct{}
	// openUsageMsg opens the usage report.
	openUsageMsg struct{}
)

func usageTick() tea.Cmd {
	return tea.Tick(usageInterval, func(time.Time) tea.Msg { return usageTickMsg{} })
}
func openUsage() tea.Cmd { return func() tea.Msg { return openUsageMsg{} } }

func push(s screen) tea.Cmd { return func() tea.Msg { return pushMsg{s} } }
func pushAll(screens ...screen) tea.Cmd {
	return func() tea.Msg { return pushAllMsg{screens} }
}
func pop() tea.Cmd { return func() tea.Msg { return popMsg{n: 1} } }

// popThen closes the top screen, then runs the command: the order is fixed,
// unlike two commands sent together.
func popThen(then tea.Cmd) tea.Cmd { return func() tea.Msg { return popMsg{n: 1, then: then} } }

func flash(text string) tea.Cmd    { return func() tea.Msg { return flashMsg{text: text} } }
func flashErr(text string) tea.Cmd { return func() tea.Msg { return flashMsg{text: text, err: true} } }
func reload() tea.Cmd              { return func() tea.Msg { return reloadMsg{} } }

type flashState struct {
	text string
	err  bool
}

// App is the root model.
type App struct {
	env    Env
	ws     workspace.Workspace
	w, h   int
	stack  []screen
	flash  flashState
	claude string
	frame  int
	usage  *usageFeed
}

// New builds the interface. It starts at 80×24 until the terminal reports its
// size.
func New(env Env) tea.Model {
	a := &App{
		env:    env,
		ws:     workspace.Workspace{Paths: env.Paths, Now: env.Now},
		w:      minWidth,
		h:      minHeight,
		claude: env.ClaudeVersion,
	}
	cwd := env.Cwd
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	now := env.Now
	if now == nil {
		now = time.Now
	}
	a.usage = newUsageFeed(env.Paths.ClaudeDir, cwd, now)
	a.reloadColors()
	a.stack = []screen{newHome(a.ws)}
	switch {
	case env.Welcome != nil:
		a.stack = append(a.stack, newWelcome(a.ws, env.Welcome))
	case env.Start.Kind == StartNew:
		a.stack = append(a.stack, newCreateFlow(a.ws, nil))
	case env.Start.Kind == StartUsage:
		a.stack = append(a.stack, newUsageScreen(a.usage))
	case env.Start.Kind == StartEdit:
		if s := newEditStart(a.ws, env.Start); s != nil {
			a.stack = append(a.stack, s...)
		}
	}
	return a
}

func (a *App) top() screen { return a.stack[len(a.stack)-1] }

// Init starts the screens and looks up the claude version.
func (a *App) Init() tea.Cmd {
	cmds := []tea.Cmd{}
	for _, s := range a.stack {
		cmds = append(cmds, s.Init())
	}
	if a.claude == "" {
		cmds = append(cmds, lookupClaude)
	}
	if a.env.Animate {
		cmds = append(cmds, tick())
	}
	if a.env.Live {
		cmds = append(cmds, usageTick())
	}
	return tea.Batch(cmds...)
}

func lookupClaude() tea.Msg {
	bin, err := claude.Find()
	if err != nil {
		return claudeMsg{}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	version, err := claude.Client{Bin: bin}.Version(ctx)
	if err != nil {
		return claudeMsg{}
	}
	return claudeMsg{version: shortVersion(version)}
}

// shortVersion turns "2.1.309 (Claude Code)" into "2.1.309".
func shortVersion(v string) string {
	if i := strings.IndexByte(v, ' '); i > 0 {
		return v[:i]
	}
	return v
}

// Update routes messages to the top screen and handles navigation.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		a.w, a.h = m.Width, m.Height
		return a, nil
	case tea.KeyPressMsg:
		if m.String() == "ctrl+c" {
			return a, tea.Quit
		}
		a.flash = flashState{}
		return a.forward(msg)
	case tickMsg:
		a.frame++
		if a.env.Animate {
			return a, tick()
		}
		return a, nil
	case claudeMsg:
		a.claude = m.version
		return a, nil
	case usageTickMsg:
		a.usage.refresh()
		if a.env.Live {
			return a, usageTick()
		}
		return a, nil
	case openUsageMsg:
		a.usage.refresh()
		a.stack = append(a.stack, newUsageScreen(a.usage))
		return a, nil
	case tea.MouseMsg:
		return a.forward(msg)
	case tryDoneMsg:
		a.flash = flashState{}
		return a.Update(tryDone(m))
	case flashMsg:
		a.flash = flashState(m)
		return a, nil
	case pushMsg:
		a.stack = append(a.stack, m.s)
		return a, m.s.Init()
	case pushAllMsg:
		var cmds []tea.Cmd
		for _, s := range m.screens {
			a.stack = append(a.stack, s)
			cmds = append(cmds, s.Init())
		}
		return a, tea.Batch(cmds...)
	case popMsg:
		return a.pop(m)
	case reloadMsg:
		a.reloadColors()
		var cmds []tea.Cmd
		for i, s := range a.stack {
			next, cmd := s.Update(msg)
			a.stack[i] = next
			cmds = append(cmds, cmd)
		}
		return a, tea.Batch(cmds...)
	}
	return a.forward(msg)
}

func (a *App) forward(msg tea.Msg) (tea.Model, tea.Cmd) {
	if len(a.stack) == 0 {
		return a, tea.Quit
	}
	next, cmd := a.top().Update(msg)
	a.stack[len(a.stack)-1] = next
	return a, cmd
}

func (a *App) pop(m popMsg) (tea.Model, tea.Cmd) {
	n := min(m.n, len(a.stack))
	a.stack = a.stack[:len(a.stack)-n]
	// A screen that has served its purpose is not shown again.
	for len(a.stack) > 1 {
		if f, ok := a.top().(finisher); !ok || !f.finished() {
			break
		}
		a.stack = a.stack[:len(a.stack)-1]
	}
	if len(a.stack) == 0 {
		return a, tea.Quit
	}
	// The screen underneath may show stale data after what the closed one did.
	next, cmd := a.top().Update(reloadMsg{})
	a.stack[len(a.stack)-1] = next
	return a, tea.Batch(cmd, m.then)
}

// View draws the top screen above the footer.
func (a *App) View() tea.View {
	var content string
	switch {
	case a.w < 40 || a.h < 10:
		note := sWarn.Render("Terminal too small") + "\n" + sMuted.Render("squad-up needs 80×24")
		content = lipgloss.Place(a.w, a.h, lipgloss.Center, lipgloss.Center, note)
	default:
		ctx := viewCtx{w: a.w, h: a.h - 1, claude: a.claude, frame: a.frame, anim: a.env.Animate, usage: a.usage}
		var body string
		maxW, maxH := windowW, windowH
		if _, ok := a.top().(fullscreen); ok {
			maxW, maxH = homeMaxW, homeMaxH
		}
		ctx.w, ctx.h = min(a.w, maxW), min(a.h-1, maxH)
		ctx.ox, ctx.oy = (a.w-ctx.w)/2, (a.h-1-ctx.h)/2
		body = lipgloss.Place(a.w, a.h-1, lipgloss.Center, lipgloss.Center, a.top().View(ctx))
		content = body + "\n" + a.footer()
	}
	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.WindowTitle = "squad-up"
	return v
}

// footer shows the status message, or else as many key hints as fit.
func (a *App) footer() string {
	if a.flash.text != "" {
		style, icon := sOK, "✓ "
		if a.flash.err {
			style, icon = sErr, "✗ "
		}
		return " " + style.Render(truncate(icon+a.flash.text, a.w-2))
	}
	return " " + renderHints(a.top().Hints(), a.w-2)
}

// renderHints lays the hints out in one line. When they do not fit the ones
// in the middle go first; the last two (help and quit) stay.
func renderHints(hints []hint, w int) string {
	render := func(hs []hint) string {
		parts := make([]string, len(hs))
		for i, h := range hs {
			parts[i] = keycap(h.key) + " " + sMuted.Render(h.label)
		}
		return strings.Join(parts, "  ")
	}
	hs := append([]hint(nil), hints...)
	for ansi.StringWidth(render(hs)) > w && len(hs) > 3 {
		hs = append(hs[:len(hs)-3], hs[len(hs)-2:]...)
	}
	return truncate(render(hs), w)
}

// reloadColors learns the colors of the agents of the equipped preset, for the
// usage report.
func (a *App) reloadColors() {
	entries, err := a.ws.Entries()
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.Active && !e.Initial {
			if card, err := a.ws.Card(e.Name); err == nil {
				a.usage.setColors(card)
			}
			return
		}
	}
	a.usage.setColors(workspace.Card{})
}

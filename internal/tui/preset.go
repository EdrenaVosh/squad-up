package tui

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/EdrenaVosh/squad-up/internal/lint"
	"github.com/EdrenaVosh/squad-up/internal/models"
	"github.com/EdrenaVosh/squad-up/internal/preset"
	"github.com/EdrenaVosh/squad-up/internal/workspace"
)

type rowKind int

const (
	rowAgent rowKind = iota
	rowAdd
	rowModel
	rowEffort
	rowPerms
	rowMain
	rowFlow
	rowMode
)

type row struct {
	kind  rowKind
	agent int // index of the agent, for rowAgent
}

// presetScreen shows one preset: its agents as slots, and its settings. Every
// change is saved at once.
type presetScreen struct {
	ws       workspace.Workspace
	name     string
	bundle   *preset.Bundle // nil for Initial, which is read-only
	card     workspace.Card
	report   lint.Report
	cursor   int
	readOnly bool
	loadErr  string
}

func newPresetScreen(ws workspace.Workspace, name string) *presetScreen {
	p := &presetScreen{ws: ws, name: name, readOnly: name == "initial"}
	p.load()
	return p
}

func (p *presetScreen) load() {
	p.loadErr = ""
	if card, err := p.ws.Card(p.name); err != nil {
		p.loadErr = err.Error()
	} else {
		p.card = card
	}
	if report, err := p.ws.Lint(p.name); err == nil {
		p.report = report
	}
	if p.readOnly {
		return
	}
	b, err := p.ws.Store().Load(p.name)
	if err != nil {
		p.bundle, p.loadErr = nil, err.Error()
		return
	}
	p.bundle = b
	p.cursor = min(p.cursor, max(len(p.rows())-1, 0))
}

func (p *presetScreen) Init() tea.Cmd { return nil }

// rows lists what the cursor can stop on.
func (p *presetScreen) rows() []row {
	if p.bundle == nil {
		return nil
	}
	var rows []row
	for i := range p.bundle.Agents {
		rows = append(rows, row{kind: rowAgent, agent: i})
	}
	return append(rows, row{kind: rowAdd}, row{kind: rowModel}, row{kind: rowEffort}, row{kind: rowPerms}, row{kind: rowMain}, row{kind: rowFlow}, row{kind: rowMode})
}

func (p *presetScreen) current() (row, bool) {
	rows := p.rows()
	if p.cursor < 0 || p.cursor >= len(rows) {
		return row{}, false
	}
	return rows[p.cursor], true
}

func (p *presetScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch m := msg.(type) {
	case reloadMsg:
		p.load()
		return p, nil
	case tea.KeyPressMsg:
		return p.key(m)
	}
	return p, nil
}

func (p *presetScreen) key(m tea.KeyPressMsg) (screen, tea.Cmd) {
	switch m.String() {
	case "esc", "q", "left":
		return p, pop()
	case "e":
		return p, push(newEquip(p.ws, p.name))
	case "c":
		return p, push(cloneFlow(p.ws, p.name))
	case "t":
		if p.readOnly {
			return p, flashErr("Initial is your own setup: just run claude")
		}
		return p, tryCmd(p.ws, p.name)
	case "?":
		return p, push(newHelp())
	}
	if p.readOnly {
		return p, nil
	}
	rows := p.rows()
	switch m.String() {
	case "up", "k":
		p.cursor = max(p.cursor-1, 0)
	case "down", "j":
		p.cursor = min(p.cursor+1, len(rows)-1)
	case "a":
		return p, push(p.addAgentFlow())
	}
	r, ok := p.current()
	if !ok {
		return p, nil
	}
	switch m.String() {
	case "enter", "right", "l":
		return p.activate(r)
	case "x", "delete":
		if r.kind == rowAgent {
			return p, push(p.removeAgentFlow(r.agent))
		}
	case "K", "shift+up":
		return p, p.moveAgent(r, -1)
	case "J", "shift+down":
		return p, p.moveAgent(r, 1)
	}
	return p, nil
}

func (p *presetScreen) activate(r row) (screen, tea.Cmd) {
	switch r.kind {
	case rowAgent:
		return p, push(newAgentEditor(p.ws, p.bundle, r.agent))
	case rowAdd:
		return p, push(p.addAgentFlow())
	case rowModel:
		options, current := modelOptions(p.ws, false, p.bundle.Preset.Settings.Model, "Not set", "use my original setting")
		return p, push(newChoice("Default model", options, current, func(o option) tea.Cmd {
			p.bundle.Preset.Settings.Model = o.value
			return p.save()
		}))
	case rowEffort:
		options := []option{{label: "Not set", note: "use my original setting"}}
		for _, level := range preset.SettingsEffortLevels {
			options = append(options, option{label: level, value: level})
		}
		return p, push(newChoice("Effort", options, indexOfValue(options, p.bundle.Preset.Settings.EffortLevel), func(o option) tea.Cmd {
			p.bundle.Preset.Settings.EffortLevel = o.value
			return p.save()
		}))
	case rowPerms:
		return p, push(newPermsScreen(p.ws, p.bundle))
	case rowMain:
		options := []option{{label: "None", note: "Claude Code's own main prompt stays"}}
		for _, a := range p.bundle.Preset.Agents {
			options = append(options, option{label: a, value: a})
		}
		current := 0
		if p.bundle.Preset.Main != nil {
			current = max(indexOfValue(options, *p.bundle.Preset.Main), 0)
		}
		title := "Main agent"
		return p, push(newChoice(title, options, current, func(o option) tea.Cmd {
			if o.value == "" {
				p.bundle.Preset.Main = nil
				return p.save()
			}
			p.bundle.Preset.Main = &o.value
			return tea.Batch(p.save(), flash("The main agent's prompt replaces Claude Code's whole system prompt"))
		}))
	case rowFlow:
		return p, push(p.flowChoice())
	case rowMode:
		options := []option{
			{label: "replace", note: "your own agents are removed while it is equipped", value: preset.ModeReplace},
			{label: "extend", note: "your own agents stay next to it", value: preset.ModeExtend},
		}
		return p, push(newChoice("Mode", options, indexOfValue(options, p.bundle.Preset.Mode), func(o option) tea.Cmd {
			p.bundle.Preset.Mode = o.value
			return p.save()
		}))
	}
	return p, nil
}

// save writes the preset and refreshes what the screen shows.
func (p *presetScreen) save() tea.Cmd {
	if err := p.ws.Save(*p.bundle); err != nil {
		p.load()
		return flashErr(err.Error())
	}
	p.load()
	return nil
}

func (p *presetScreen) moveAgent(r row, delta int) tea.Cmd {
	if r.kind != rowAgent {
		return nil
	}
	to := r.agent + delta
	if to < 0 || to >= len(p.bundle.Agents) {
		return nil
	}
	p.bundle.Agents[r.agent], p.bundle.Agents[to] = p.bundle.Agents[to], p.bundle.Agents[r.agent]
	p.bundle.Preset.Agents[r.agent], p.bundle.Preset.Agents[to] = p.bundle.Preset.Agents[to], p.bundle.Preset.Agents[r.agent]
	p.cursor = to
	return p.save()
}

func (p *presetScreen) addAgentFlow() screen {
	check := func(name string) error {
		if !preset.ValidName(name) {
			return fmt.Errorf("agent names use lowercase letters, digits and dashes (up to 64 characters)")
		}
		if slices.Contains(p.bundle.Preset.Agents, name) {
			return fmt.Errorf("the preset already has an agent named %q", name)
		}
		return nil
	}
	return newPrompt("New agent", "What is it called? You write what it does next.", "", check, func(name string) tea.Cmd {
		data := fmt.Sprintf("---\nname: %s\ndescription: Describe what this agent does and when to use it.\ntools: Read, Grep, Glob\n---\nDescribe how this agent works.\n", name)
		p.bundle.Preset.Agents = append(p.bundle.Preset.Agents, name)
		p.bundle.Agents = append(p.bundle.Agents, preset.AgentFile{Name: name, Data: []byte(data)})
		p.cursor = len(p.bundle.Agents) - 1
		if cmd := p.save(); cmd != nil {
			return cmd
		}
		return push(newAgentEditor(p.ws, p.bundle, len(p.bundle.Agents)-1))
	})
}

func (p *presetScreen) removeAgentFlow(i int) screen {
	name := p.bundle.Agents[i].Name
	body := []string{fmt.Sprintf("Remove the agent %q from this preset?", name), "Its file is deleted."}
	return newConfirm("Remove agent", body, "Remove", true, func() tea.Cmd {
		p.bundle.Agents = slices.Delete(p.bundle.Agents, i, i+1)
		p.bundle.Preset.Agents = slices.DeleteFunc(p.bundle.Preset.Agents, func(a string) bool { return a == name })
		if p.bundle.Preset.Main != nil && *p.bundle.Preset.Main == name {
			p.bundle.Preset.Main = nil
		}
		p.bundle.Preset.PruneFlow()
		p.cursor = min(p.cursor, len(p.bundle.Agents))
		return p.save()
	})
}

func (p *presetScreen) Hints() []hint {
	if p.readOnly {
		return []hint{{"c", "copy to edit"}, {"e", "restore"}, {"esc", "back"}, {"?", "help"}}
	}
	return []hint{
		{"↑↓", "move"}, {"enter", "edit"}, {"e", "equip"}, {"t", "try"}, {"a", "add agent"},
		{"x", "remove"}, {"K/J", "reorder"}, {"c", "clone"}, {"esc", "back"}, {"?", "help"},
	}
}

func (p *presetScreen) View(c viewCtx) string {
	inner := c.w - 2
	title := p.card.Title
	if title == "" {
		title = p.name
	}
	right := p.name
	if p.card.Mode != "" {
		right += " · " + p.card.Mode
	}
	var lines []string
	add := func(l string) { lines = append(lines, l) }

	if p.loadErr != "" {
		add(" " + sErr.Render(truncate("✗ "+p.loadErr, inner-2)))
		return c.box(title, right, lines, 0)
	}
	add(" " + sMuted.Render(truncate(p.card.Description, inner-2)))
	where := "saved in " + tildePath(p.ws.Paths.PresetDir(p.name)) + " · every change is written at once"
	if p.readOnly {
		where = "your original setup, kept in " + tildePath(p.ws.Paths.InitialDir()) + " and never changed"
	}
	add(" " + fg(cDim).Render(truncate(where, inner-2)))
	add("")
	add(" " + sMuted.Render("AGENTS"))
	if len(p.card.Agents) > 0 {
		add(p.agentHeader(inner))
		if c.h >= 30 {
			add("   " + fg(cDim).Render(strings.Repeat("─", max(inner-6, 0))))
		}
	}
	for i, a := range p.card.Agents {
		add(p.slotLine(i, a, inner))
	}
	if len(p.card.Agents) == 0 && p.readOnly {
		add("   " + sMuted.Render("no agents"))
	}
	if !p.readOnly {
		add(p.line(row{kind: rowAdd}, sAccent.Render("+")+" add agent", inner))
	}
	add("")
	add(" " + sMuted.Render("SETTINGS"))
	for _, s := range []struct {
		kind  rowKind
		label string
		value string
	}{
		{rowModel, "Default model", modelOrDash(p.card.Model)},
		{rowEffort, "Effort", effortBars(p.card.Effort)},
		{rowPerms, "Permissions", orDash(p.card.Permissions, "—")},
		{rowMain, "Main agent", orDash(p.card.Main, "—")},
		{rowFlow, "Team flow", flowWords(p.card)},
		{rowMode, "Mode", modeWords(p.card.Mode)},
	} {
		add(p.line(row{kind: s.kind}, sMuted.Render(pad(s.label, 16))+s.value, inner))
	}
	if c.h >= 32 && !p.readOnly {
		add("")
		add(" " + sMuted.Render("ABOUT  ") + sBold.Render(p.aboutTitle()))
		for _, l := range wrapText(p.about(), inner-6) {
			add("   " + sMuted.Render(l))
		}
	}
	add("")
	add(" " + sMuted.Render("LINT  ") + lintWord(p.card))
	shown := 0
	findings := slices.Clone(p.report.Findings)
	severity := map[lint.Level]int{lint.LevelError: 2, lint.LevelWarn: 1}
	slices.SortStableFunc(findings, func(a, b lint.Finding) int { return severity[b.Level] - severity[a.Level] }) // errors first
	for _, f := range findings {
		if f.Level == lint.LevelInfo || shown == 3 {
			continue
		}
		shown++
		fix := f.Fix
		if c.h < 30 {
			fix = "" // no room for the hint
		}
		for _, l := range findingLines(f.Level, f.Message, fix, inner-6) {
			add("   " + l)
		}
	}
	return c.box(title, right, lines, 0)
}

// The columns of the agents table.
const (
	colSlot, colAgent, colModel, colEffort, colAccess, colStance = 8, 14, 13, 13, 14, 13
)

// agentHeader names the columns of the agents table.
func (p *presetScreen) agentHeader(inner int) string {
	head := pad("SLOT", colSlot) + pad("AGENT", colAgent) + pad("MODEL", colModel) + pad("EFFORT", colEffort) + pad("ACCESS", colAccess)
	if p.card.Team {
		head += pad("STANCE", colStance)
	}
	return truncate("   "+sMuted.Render(head), inner)
}

// slotLine draws one agent row, with the cursor if it is the current one.
func (p *presetScreen) slotLine(i int, a workspace.AgentInfo, inner int) string {
	text := pad(sMuted.Render(fmt.Sprintf("SLOT %d", i+1)), colSlot) +
		pad(swatch(a.Color)+" "+sBold.Render(truncate(a.Name, colAgent-2)), colAgent) +
		pad(modelPill(a.Model), colModel) + pad(effortBars(a.Effort), colEffort) + pad(permissionStrength(a.PermissionMode), colAccess)
	if p.card.Team {
		text += pad(independentChip(a.Independent), colStance)
	}
	return p.line(row{kind: rowAgent, agent: i}, text, inner)
}

// aboutTitle and about explain the row under the cursor.
func (p *presetScreen) aboutTitle() string {
	r, ok := p.current()
	if !ok {
		return ""
	}
	switch r.kind {
	case rowAgent:
		return p.card.Agents[r.agent].Name
	case rowAdd:
		return "Add agent"
	case rowModel:
		return "Default model"
	case rowEffort:
		return "Effort"
	case rowPerms:
		return "Permissions"
	case rowMain:
		return "Main agent"
	case rowFlow:
		return "Team flow"
	case rowMode:
		return "Mode"
	}
	return ""
}

func (p *presetScreen) about() string {
	r, ok := p.current()
	if !ok {
		return ""
	}
	switch r.kind {
	case rowAgent:
		a := p.card.Agents[r.agent]
		return orDash(a.Description, "This agent has no description yet: Claude cannot tell when to call it.")
	case rowAdd:
		return "Adds a new agent to the preset and opens its editor. Reorder the slots with K and J."
	case rowModel:
		return "The model every agent uses unless it names its own. Not set keeps the one from your original setup."
	case rowEffort:
		return "How hard the model thinks by default: low is quick and cheap, xhigh is slow and deep. An agent can set its own."
	case rowPerms:
		return "What Claude may do without asking: the permission mode and the allow, ask and deny rules."
	case rowMain:
		return "The agent that becomes the session itself: its prompt replaces Claude Code's own system prompt, and it leads the squad's flow. None keeps Claude Code's."
	case rowFlow:
		return "How the agents work as a team: separate helpers, in slot order, or in order with a review loop that sends defects back to whoever fixes them."
	case rowMode:
		return "Replace removes your own agents while the preset is equipped; extend keeps them next to the preset's."
	}
	return ""
}

// line prefixes a row with the cursor when the row is the current one.
func (p *presetScreen) line(r row, text string, inner int) string {
	prefix := "   "
	if cur, ok := p.current(); ok && cur == r {
		prefix = " " + selBar() + " "
	}
	return truncate(prefix+text, inner)
}

// flowWords says in a line how the agents of a card work together.
func flowWords(c workspace.Card) string {
	switch {
	case c.Flow == nil:
		return "off " + sMuted.Render("— separate helpers")
	case c.Flow.Loops():
		return fmt.Sprintf("review loop %s %s %s %s", sMuted.Render("—"), c.Flow.Gate, sMuted.Render("checks,"), c.Flow.Fix+sMuted.Render(fmt.Sprintf(" fixes · up to %d", c.Flow.MaxRounds())))
	}
	return "in order " + sMuted.Render("— down the slots, no loop")
}

// flowChoice asks how the agents work together. The review loop asks on: who
// checks, who fixes, and how many rounds.
func (p *presetScreen) flowChoice() screen {
	current := 0
	switch f := p.bundle.Preset.Flow; {
	case f.Loops():
		current = 2
	case f != nil:
		current = 1
	}
	options := []option{
		{label: "Off", note: "separate helpers: Claude calls them as it sees fit", value: "off"},
		{label: "In order", note: "the main agent hands work down the slots", value: "order"},
		{label: "Review loop", note: "the last check sends defects back to the one who fixes them", value: "loop"},
	}
	return newChoice("Team flow", options, current, func(o option) tea.Cmd {
		switch o.value {
		case "off":
			p.bundle.Preset.Flow = nil
			return p.save()
		case "order":
			p.bundle.Preset.Flow = &preset.Flow{}
			return tea.Batch(p.save(), p.flowNote())
		}
		if len(p.bundle.Preset.Agents) < 2 {
			return flashErr("A review loop needs two agents: one to check and one to fix")
		}
		return push(p.agentChoice("Who checks the work?", "the gate: runs last and reports defects", "", func(gate string) tea.Cmd {
			return push(p.agentChoice("Who fixes the defects?", "gets only the defects back, then the gate runs again", gate, func(fix string) tea.Cmd {
				return push(p.roundsChoice(gate, fix))
			}))
		}))
	})
}

func (p *presetScreen) agentChoice(title, note, skip string, then func(string) tea.Cmd) screen {
	var options []option
	for _, a := range p.bundle.Preset.Agents {
		if a != skip {
			options = append(options, option{label: a, note: note, value: a})
		}
	}
	return newChoice(title, options, 0, func(o option) tea.Cmd { return then(o.value) })
}

func (p *presetScreen) roundsChoice(gate, fix string) screen {
	var options []option
	for n := 1; n <= preset.MaxRounds; n++ {
		note := ""
		if n == preset.DefaultRounds {
			note = "the default"
		}
		options = append(options, option{label: fmt.Sprintf("%d", n), note: note, value: fmt.Sprintf("%d", n)})
	}
	return newChoice("How many rounds at most?", options, preset.DefaultRounds-1, func(o option) tea.Cmd {
		var n int
		_, _ = fmt.Sscanf(o.value, "%d", &n)
		p.bundle.Preset.Flow = &preset.Flow{Gate: gate, Fix: fix, Rounds: n}
		return tea.Batch(p.save(), p.flowNote())
	})
}

// flowNote tells where a flow runs, when no main agent carries it.
func (p *presetScreen) flowNote() tea.Cmd {
	if p.bundle.Preset.Main == nil {
		return flash("No main agent: `squad run` passes the flow to Claude Code; to equip it, set a main agent")
	}
	return nil
}

func modeWords(mode string) string {
	switch mode {
	case preset.ModeReplace:
		return "replace " + sMuted.Render("— your own agents are removed while equipped")
	case preset.ModeExtend:
		return "extend " + sMuted.Render("— your own agents stay")
	}
	return "—"
}

// findingLines is a finding as a mark and its message wrapped to w cells, the
// lines after the first indented under the text; a fix is added as a muted
// line that starts with an arrow. Nothing is cut off.
func findingLines(level lint.Level, message, fix string, w int) []string {
	var out []string
	for i, l := range wrapText(message, max(w-2, 8)) {
		if i == 0 {
			out = append(out, findingMark(level)+" "+l)
		} else {
			out = append(out, "  "+l)
		}
	}
	if fix != "" {
		for i, l := range wrapText(fix, max(w-4, 8)) {
			if i == 0 {
				out = append(out, "  "+sMuted.Render("→ "+l))
			} else {
				out = append(out, "    "+sMuted.Render(l))
			}
		}
	}
	return out
}

func findingMark(l lint.Level) string {
	switch l {
	case lint.LevelError:
		return sErr.Render("✗")
	case lint.LevelWarn:
		return sWarn.Render("!")
	}
	return sMuted.Render("·")
}

func indexOfValue(options []option, value string) int {
	for i, o := range options {
		if o.value == value {
			return i
		}
	}
	return 0
}

// modelOptions lists the models to pick from. Unavailable ones are shown
// greyed out with the reason. unsetLabel/unsetNote describe the first option,
// which clears the value; for agents the first option is "inherit" instead.
func modelOptions(ws workspace.Workspace, forAgent bool, current, unsetLabel, unsetNote string) ([]option, int) {
	policy, _ := ws.Policy()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	results := models.Evaluate(models.FullCatalog(), policy, ws.CachedProbe(ctx))

	var options []option
	if !forAgent {
		options = append(options, option{label: unsetLabel, note: unsetNote})
	}
	for _, r := range results {
		if (forAgent && !r.Agents) || (!forAgent && !r.Settings) || r.Name == "default" {
			continue
		}
		o := option{label: models.Title(r.Name), value: r.Name, note: statusNote(r)}
		if isRelease(r.Name) {
			o.note = releaseNote(r)
		}
		if r.Status == models.StatusUnavailable {
			o.disabled, o.reason, o.note = true, r.Reason, ""
		}
		options = append(options, o)
	}
	return options, indexOfValue(options, current)
}

// isRelease reports whether a catalogue name is a release's full ID.
func isRelease(name string) bool { return strings.HasPrefix(name, "claude-") }

// releaseNote is what the picker says about a release: its ID, and whether it
// is known to work.
func releaseNote(r models.Result) string {
	state := "not checked"
	switch r.Status {
	case models.StatusVerified:
		state = "available"
	case models.StatusAllowed:
		state = "allowed by policy"
	}
	return r.Name + " · " + state
}

func statusNote(r models.Result) string {
	if r.Name == "inherit" || r.Name == "default" {
		return r.Reason
	}
	parts := []string{}
	if r.Status != models.StatusUnknown {
		parts = append(parts, string(r.Status))
	}
	if r.ResolvedTo != "" {
		parts = append(parts, r.ResolvedTo)
	}
	return strings.Join(parts, " · ")
}

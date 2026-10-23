package tui

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/EdrenaVosh/squadup/internal/apply"
	"github.com/EdrenaVosh/squadup/internal/workspace"
)

// listWidth is the width of the preset list on the main screen.
const listWidth = 22

// home is the main screen: the presets on the left, the selected one on the right.
type home struct {
	ws        workspace.Workspace
	entries   []workspace.Entry
	cursor    int // index into visible()
	card      workspace.Card
	model     string // default model of the equipped preset, for the title bar
	drift     []apply.DriftItem
	query     string
	filter    textinput.Model
	filtering bool
	loadErr   string
}

func newHome(ws workspace.Workspace) *home {
	h := &home{ws: ws}
	h.filter = textinput.New()
	h.filter.Prompt = "/ "
	steady(&h.filter)
	h.reload("")
	return h
}

func (h *home) Init() tea.Cmd { return nil }

// visible returns the entries the filter lets through; Initial always stays.
func (h *home) visible() []workspace.Entry {
	q := strings.ToLower(h.query)
	var out []workspace.Entry
	for _, e := range h.entries {
		if e.Initial || q == "" || strings.Contains(strings.ToLower(e.Name+" "+e.Title), q) {
			out = append(out, e)
		}
	}
	return out
}

func (h *home) selected() *workspace.Entry {
	vis := h.visible()
	if h.cursor < 0 || h.cursor >= len(vis) {
		return nil
	}
	return &vis[h.cursor]
}

func (h *home) selectedName() string {
	if e := h.selected(); e != nil {
		return e.Name
	}
	return ""
}

// reload reads everything again and keeps the selection on the preset keep.
func (h *home) reload(keep string) {
	entries, err := h.ws.Entries()
	h.entries, h.loadErr = entries, ""
	if err != nil {
		h.loadErr = err.Error()
	}
	if state, err := apply.LoadState(h.ws.Paths); err == nil {
		h.drift, _ = apply.Drift(h.ws.Paths, state)
	}

	h.model = ""
	for _, e := range entries {
		if e.Active {
			if c, err := h.ws.Card(e.Name); err == nil {
				h.model = c.Model
			}
		}
	}

	vis := h.visible()
	h.cursor = min(h.cursor, max(len(vis)-1, 0))
	if keep == "" {
		// The first time: start on the equipped preset, if there is one.
		keep = h.equippedPreset()
	}
	if i := slices.IndexFunc(vis, func(e workspace.Entry) bool { return e.Name == keep }); i >= 0 {
		h.cursor = i
	}
	h.loadCard()
}

// equippedPreset is the name of the equipped preset, or "" for Initial.
func (h *home) equippedPreset() string {
	for _, e := range h.entries {
		if e.Active && !e.Initial {
			return e.Name
		}
	}
	return ""
}

func (h *home) loadCard() {
	e := h.selected()
	if e == nil {
		h.card = workspace.Card{}
		return
	}
	c, err := h.ws.Card(e.Name)
	if err != nil {
		c = workspace.Card{Entry: *e}
		c.Error = err.Error()
	}
	h.card = c
}

func (h *home) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch m := msg.(type) {
	case reloadMsg:
		h.reload(h.selectedName())
		return h, nil
	case tea.KeyPressMsg:
		if h.filtering {
			return h.updateFilter(m)
		}
		return h.updateKeys(m)
	}
	return h, nil
}

func (h *home) updateFilter(m tea.KeyPressMsg) (screen, tea.Cmd) {
	switch m.String() {
	case "esc":
		h.filtering, h.query = false, ""
		h.filter.SetValue("")
		h.filter.Blur()
		h.cursor = 0
		h.loadCard()
		return h, nil
	case "enter":
		h.filtering = false
		h.filter.Blur()
		return h, nil
	case "up", "down":
		return h.updateKeys(m)
	}
	var cmd tea.Cmd
	h.filter, cmd = h.filter.Update(m)
	h.query = h.filter.Value()
	h.cursor = 0
	h.loadCard()
	return h, cmd
}

func (h *home) updateKeys(m tea.KeyPressMsg) (screen, tea.Cmd) {
	sel := h.selected()
	switch m.String() {
	case "up", "k":
		h.move(-1)
	case "down", "j":
		h.move(1)
	case "q":
		return h, tea.Quit
	case "?":
		return h, push(newHelp())
	case "/":
		h.filtering = true
		return h, h.filter.Focus()
	case "esc":
		if h.query != "" {
			h.query = ""
			h.filter.SetValue("")
			h.cursor = 0
			h.loadCard()
		}
	case "n":
		var base *string
		if sel != nil {
			base = &sel.Name
		}
		return h, push(newCreateFlow(h.ws, base))
	}
	if sel == nil {
		return h, nil
	}
	switch m.String() {
	case "enter":
		return h, push(newPresetScreen(h.ws, sel.Name))
	case "e":
		return h, push(newEquip(h.ws, sel.Name))
	case "t":
		if sel.Initial {
			return h, flashErr("Initial is your own setup: just run claude")
		}
		return h, tryCmd(h.ws, sel.Name)
	case "c":
		return h, push(cloneFlow(h.ws, sel.Name))
	case "r":
		if sel.Initial {
			return h, flashErr(workspace.ErrProtected.Error())
		}
		return h, push(renameFlow(h.ws, sel.Name))
	case "d":
		if sel.Initial {
			return h, flashErr(workspace.ErrProtected.Error())
		}
		return h, push(removeFlow(h.ws, *sel))
	}
	return h, nil
}

func (h *home) move(delta int) {
	n := len(h.visible())
	if n == 0 {
		return
	}
	h.cursor = (h.cursor + delta + n) % n
	h.loadCard()
}

func (h *home) Hints() []hint {
	if h.filtering {
		return []hint{{"enter", "keep filter"}, {"esc", "clear"}}
	}
	return []hint{
		{"↑↓", "select"}, {"enter", "open"}, {"e", "equip"}, {"t", "try"}, {"n", "new"},
		{"c", "clone"}, {"r", "rename"}, {"d", "delete"}, {"/", "filter"}, {"?", "help"}, {"q", "quit"},
	}
}

func (h *home) View(c viewCtx) string {
	rows := c.h
	banner := ""
	if len(h.drift) > 0 {
		banner = sWarn.Render(truncate(" ⚠ Manual edits detected in the equipped preset ("+plural(len(h.drift), "file")+" or setting). They are asked about when you switch.", c.w)) + "\n"
		rows--
	}

	inner := c.w - 2
	label := ""
	if c.claude != "" {
		label = "claude " + c.claude
		if h.model != "" {
			label += " · " + h.model
		}
	}

	if c.w < 2*listWidth+10 {
		left := h.listLines(inner, rows-2)
		return banner + box("squadup", label, left, c.w, rows, 0)
	}
	left := h.listLines(listWidth, rows-2)
	right := cardLines(h.card, inner-listWidth-1, rows-2)
	return banner + box("squadup", label, panes(left, right, listWidth, inner), c.w, rows, listWidth)
}

// panes puts two columns side by side with a divider; every line comes out
// inner cells wide.
func panes(left, right []string, leftW, inner int) []string {
	n := max(len(left), len(right))
	out := make([]string, n)
	for i := range out {
		l, r := "", ""
		if i < len(left) {
			l = left[i]
		}
		if i < len(right) {
			r = right[i]
		}
		out[i] = pad(l, leftW) + sMuted.Render("│") + pad(r, inner-leftW-1)
	}
	return out
}

// listLines draws the preset list: presets first, Initial below a rule.
func (h *home) listLines(w, rows int) []string {
	lines := []string{" " + sMuted.Render("PRESETS"), ""}
	vis := h.visible()

	var presets []workspace.Entry
	var initialEntry *workspace.Entry
	for i := range vis {
		if vis[i].Initial {
			initialEntry = &vis[i]
		} else {
			presets = append(presets, vis[i])
		}
	}

	room := rows - len(lines) - 2 // the rule and Initial
	if h.filtering || h.query != "" {
		room--
	}
	room = max(room, 1)
	start := 0
	if h.cursor >= room {
		start = h.cursor - room + 1
	}
	for i := start; i < len(presets) && i < start+room; i++ {
		lines = append(lines, entryLine(presets[i], i == h.cursor, w))
	}
	if len(presets) == 0 {
		msg := "no presets yet — n to create"
		if h.query != "" {
			msg = "nothing matches"
		}
		lines = append(lines, " "+sMuted.Render(truncate(msg, w-2)))
	}

	for len(lines) < rows-2-btoi(h.filtering || h.query != "") {
		lines = append(lines, "")
	}
	lines = lines[:min(len(lines), max(rows-2-btoi(h.filtering || h.query != ""), 0))]
	lines = append(lines, " "+sMuted.Render(strings.Repeat("─", max(w-2, 0))))
	if initialEntry != nil {
		lines = append(lines, entryLine(*initialEntry, h.cursor == len(presets), w))
	}
	if h.filtering {
		lines = append(lines, " "+h.filter.View())
	} else if h.query != "" {
		lines = append(lines, " "+sMuted.Render("/ "+h.query))
	}
	return lines
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

func entryLine(e workspace.Entry, selected bool, w int) string {
	marker := "○"
	switch {
	case e.Active:
		marker = sAccent.Render("●")
	case e.Initial:
		marker = sMuted.Render("◇")
	}
	name := truncate(e.Name, w-5)
	if selected {
		return " " + sAccent.Render("▸") + " " + marker + " " + sBold.Render(name)
	}
	if e.Error != "" {
		name = sErr.Render(name)
	}
	return "   " + marker + " " + name
}

// ---- help ----

type help struct{}

func newHelp() *help { return &help{} }

func (h *help) Init() tea.Cmd { return nil }

func (h *help) Update(msg tea.Msg) (screen, tea.Cmd) {
	if _, ok := msg.(tea.KeyPressMsg); ok {
		return h, pop()
	}
	return h, nil
}

func (h *help) View(c viewCtx) string {
	row := func(k, d string) string { return " " + sAccent.Render(pad(k, 9)) + d }
	lines := []string{
		sBold.Render("Main screen"),
		row("↑↓ j k", "select a preset"),
		row("enter", "open it: agents, model, permissions"),
		row("e", "equip it (write it into ~/.claude, after a preview)"),
		row("t", "try it: start claude with it, writing nothing"),
		row("n c r d", "new, clone, rename, delete"),
		row("/", "filter the list"),
		"",
		sBold.Render("Editing"),
		row("esc", "go back; changes are saved as you make them"),
		row("ctrl+c", "quit from anywhere"),
		"",
		sMuted.Render("Your original setup (◇ initial) is never changed."),
		sMuted.Render("`squad restore` brings it back at any time."),
	}
	return modal(c, "Help", lines)
}

func (h *help) Hints() []hint { return []hint{{"any key", "close"}} }

// ---- flows shared by the screens ----

// presetNameCheck validates the name of a new preset.
func presetNameCheck(ws workspace.Workspace) func(string) error {
	return func(name string) error {
		if err := workspace.CheckName(name); err != nil {
			return err
		}
		if ws.Store().Exists(name) {
			return fmt.Errorf("a preset named %q already exists", name)
		}
		return nil
	}
}

// newCreateFlow asks for a name, then for what to start from. base is the
// preset selected when the flow began, offered as a template.
func newCreateFlow(ws workspace.Workspace, base *string) screen {
	return newPrompt("New preset", "Lowercase letters, digits and dashes.", "", presetNameCheck(ws), func(name string) tea.Cmd {
		options := []option{{label: "Blank preset", note: "no agents yet", value: "blank"}}
		if base != nil {
			label := fmt.Sprintf("Copy of %q", *base)
			if *base == "initial" {
				label = "Copy of your original setup"
			}
			options = append(options, option{label: label, value: "copy"})
		}
		options = append(options, option{label: "My current Claude Code setup", note: "what is in ~/.claude now", value: "current"})
		return push(newChoice("Start "+name+" from", options, 0, func(o option) tea.Cmd {
			return createPreset(ws, name, o.value, base)
		}))
	})
}

func createPreset(ws workspace.Workspace, name, from string, base *string) tea.Cmd {
	var err error
	switch from {
	case "copy":
		_, err = ws.Clone(*base, name)
	case "current":
		_, err = ws.Capture(name)
	default:
		_, err = ws.New(name)
	}
	if err != nil {
		return flashErr(err.Error())
	}
	return tea.Batch(reload(), push(newPresetScreen(ws, name)), flash(fmt.Sprintf("Created %q", name)))
}

func cloneFlow(ws workspace.Workspace, from string) screen {
	suggestion := from + "-copy"
	if from == "initial" {
		suggestion = "my-setup"
	}
	help := fmt.Sprintf("Copy of %q.", from)
	if from == "initial" {
		help = "An editable copy of your original setup; Initial itself stays as it is."
	}
	return newPrompt("Clone preset", help, suggestion, presetNameCheck(ws), func(name string) tea.Cmd {
		skipped, err := ws.Clone(from, name)
		if err != nil {
			return flashErr(err.Error())
		}
		text := fmt.Sprintf("Copied %q to %q", from, name)
		if len(skipped) > 0 {
			text += fmt.Sprintf(" (%d agent files could not be copied: run squad lint initial)", len(skipped))
		}
		return tea.Batch(reload(), flash(text))
	})
}

func renameFlow(ws workspace.Workspace, old string) screen {
	return newPrompt("Rename preset", fmt.Sprintf("New name for %q.", old), old, presetNameCheck(ws), func(name string) tea.Cmd {
		if name == old {
			return nil
		}
		if err := ws.Rename(old, name); err != nil {
			return flashErr(err.Error())
		}
		return tea.Batch(reload(), flash(fmt.Sprintf("Renamed %q to %q", old, name)))
	})
}

func removeFlow(ws workspace.Workspace, e workspace.Entry) screen {
	body := []string{fmt.Sprintf("Delete the preset %q?", e.Name), "Its agents are deleted with it."}
	if e.Active {
		body = append(body, "", sWarn.Render("It is equipped: its files stay in ~/.claude"), sWarn.Render("until you restore your original setup."))
	}
	return newConfirm("Delete preset", body, "Delete", true, func() tea.Cmd {
		if _, err := ws.Remove(e.Name, e.Active); err != nil {
			return flashErr(err.Error())
		}
		return tea.Batch(reload(), flash(fmt.Sprintf("Deleted %q", e.Name)))
	})
}

package tui

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/EdrenaVosh/squadup/internal/agentfile"
	"github.com/EdrenaVosh/squadup/internal/editor"
	"github.com/EdrenaVosh/squadup/internal/lint"
	"github.com/EdrenaVosh/squadup/internal/models"
	"github.com/EdrenaVosh/squadup/internal/preset"
	"github.com/EdrenaVosh/squadup/internal/workspace"
)

// builtinTools are the tools Claude Code ships with, offered as checkboxes.
var builtinTools = []string{
	"Read", "Write", "Edit", "Bash", "Grep", "Glob",
	"WebFetch", "WebSearch", "TodoWrite", "NotebookEdit",
}

type fieldID int

const (
	fName fieldID = iota
	fDescription
	fModel
	fEffort
	fPermission
	fTools
	fDisallowed
	fMaxTurns
	fColor
	fPrompt
)

var agentFields = []struct {
	id    fieldID
	label string
}{
	{fName, "Name"}, {fDescription, "Description"}, {fModel, "Model"}, {fEffort, "Effort"},
	{fPermission, "Permission mode"}, {fTools, "Tools"}, {fDisallowed, "Disallowed tools"},
	{fMaxTurns, "Max turns"}, {fColor, "Color"}, {fPrompt, "Prompt"},
}

// promptEditedMsg arrives when the external editor closes.
type promptEditedMsg struct {
	path string
	err  error
}

// agentEditor edits one agent of a preset. Every change is saved at once.
type agentEditor struct {
	ws      workspace.Workspace
	bundle  *preset.Bundle
	idx     int
	agent   *agentfile.Agent
	cursor  int
	preview bool
	report  lint.Report
	loadErr string

	scroll    int      // how far the inspector is scrolled
	modelOpts []option // the models, for the inspector; nil until needed
}

func newAgentEditor(ws workspace.Workspace, bundle *preset.Bundle, idx int) *agentEditor {
	e := &agentEditor{ws: ws, bundle: bundle, idx: idx}
	agent, err := agentfile.Parse(bundle.Agents[idx].Data)
	if err != nil {
		e.loadErr = err.Error()
		return e
	}
	e.agent = agent
	e.relint()
	return e
}

func (e *agentEditor) Init() tea.Cmd { return nil }

func (e *agentEditor) data() []byte {
	data, err := e.agent.Marshal()
	if err != nil {
		return e.bundle.Agents[e.idx].Data
	}
	return data
}

func (e *agentEditor) relint() {
	name := e.bundle.Agents[e.idx].Name
	e.report = lint.Run(lint.Input{Name: name, Agents: []lint.Agent{{File: name + ".md", Data: e.data()}}})
}

// persist writes the agent into the preset and saves the preset.
func (e *agentEditor) persist() tea.Cmd {
	e.bundle.Agents[e.idx].Data = e.data()
	if err := e.ws.Save(*e.bundle); err != nil {
		return flashErr(err.Error())
	}
	e.relint()
	e.modelOpts = nil
	return nil
}

func (e *agentEditor) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch m := msg.(type) {
	case promptEditedMsg:
		return e, e.promptEdited(m)
	case tea.MouseWheelMsg:
		switch m.Button {
		case tea.MouseWheelUp:
			e.scrollBy(-3)
		case tea.MouseWheelDown:
			e.scrollBy(3)
		}
		return e, nil
	case tea.KeyPressMsg:
		return e.key(m)
	}
	return e, nil
}

func (e *agentEditor) key(m tea.KeyPressMsg) (screen, tea.Cmd) {
	switch m.String() {
	case "esc", "q", "left":
		return e, pop()
	case "?":
		return e, push(newHelp())
	}
	if e.agent == nil {
		return e, nil
	}
	if e.handleScroll(m.String()) {
		return e, nil
	}
	switch m.String() {
	case "up", "k":
		e.cursor, e.scroll = max(e.cursor-1, 0), 0
	case "down", "j":
		e.cursor, e.scroll = min(e.cursor+1, len(agentFields)-1), 0
	case "p":
		e.preview, e.scroll = !e.preview, 0
	case "o":
		if agentFields[e.cursor].id == fPrompt {
			return e, e.editPrompt()
		}
	case "f":
		if agentFields[e.cursor].id == fPrompt {
			return e, e.promptFromFile()
		}
	case "enter", "right", "l":
		return e, e.edit(agentFields[e.cursor].id)
	}
	return e, nil
}

// edit opens the right control for a field.
func (e *agentEditor) edit(id fieldID) tea.Cmd {
	switch id {
	case fName:
		return push(newPrompt("Agent name", "Lowercase letters, digits and dashes.", e.agent.Name(), e.checkName, e.rename))
	case fDescription:
		return push(newArea("Description — what it does and when to call it", e.agent.Description(), func(v string) tea.Cmd {
			e.agent.Set("description", v)
			return e.persist()
		}))
	case fModel:
		return push(e.modelChoice())
	case fEffort:
		options := []option{{label: "Not set", note: "the model's default"}}
		for _, level := range preset.EffortLevels {
			options = append(options, option{label: level, value: level})
		}
		return push(newChoice("Effort", options, indexOfValue(options, e.str("effort")), e.setter("effort")))
	case fPermission:
		options := []option{{label: "default", note: "asks before most actions"}}
		for _, mode := range preset.PermissionModes {
			if mode == "default" {
				continue
			}
			o := option{label: mode, value: mode}
			if mode == "bypassPermissions" {
				o.note = "asks for nothing — use with care"
			}
			options = append(options, o)
		}
		return push(newChoice("Permission mode", options, indexOfValue(options, e.str("permissionMode")), e.setter("permissionMode")))
	case fTools:
		return push(e.toolsChoice("Tools", "tools"))
	case fDisallowed:
		return push(e.toolsChoice("Disallowed tools", "disallowedTools"))
	case fMaxTurns:
		check := func(v string) error {
			if v == "" {
				return nil
			}
			if n, err := strconv.Atoi(v); err != nil || n < 1 {
				return fmt.Errorf("use a whole number of 1 or more, or leave it empty")
			}
			return nil
		}
		return push(newPrompt("Max turns", "How many steps the agent may take. Empty means no limit.", e.str("maxTurns"), check, func(v string) tea.Cmd {
			if v == "" {
				e.agent.Set("maxTurns", "")
			} else {
				n, _ := strconv.Atoi(v)
				e.agent.Set("maxTurns", n)
			}
			return e.persist()
		}))
	case fColor:
		options := []option{{label: "Not set"}}
		for _, c := range preset.Accents {
			options = append(options, option{label: swatch(c) + " " + c, value: c})
		}
		return push(newChoice("Color", options, indexOfValue(options, e.str("color")), e.setter("color")))
	case fPrompt:
		return push(newPromptEditor(e))
	}
	return nil
}

// setter stores the chosen option's value in a frontmatter field; an empty
// value removes the field.
func (e *agentEditor) setter(key string) func(option) tea.Cmd {
	return func(o option) tea.Cmd {
		e.agent.Set(key, o.value)
		return e.persist()
	}
}

func (e *agentEditor) checkName(name string) error {
	if !preset.ValidName(name) {
		return fmt.Errorf("agent names use lowercase letters, digits and dashes (up to 64 characters)")
	}
	for i, other := range e.bundle.Preset.Agents {
		if i != e.idx && other == name {
			return fmt.Errorf("the preset already has an agent named %q", name)
		}
	}
	return nil
}

// rename gives the agent a new name everywhere the preset mentions it.
func (e *agentEditor) rename(name string) tea.Cmd {
	old := e.bundle.Agents[e.idx].Name
	if name == old {
		return nil
	}
	e.agent.Set("name", name)
	e.bundle.Agents[e.idx].Name = name
	e.bundle.Preset.Agents[e.idx] = name
	if e.bundle.Preset.Main != nil && *e.bundle.Preset.Main == old {
		e.bundle.Preset.Main = &name
	}
	return e.persist()
}

func (e *agentEditor) modelChoice() screen {
	current := e.str("model")
	options, _ := modelOptions(e.ws, true, current, "", "")
	options = append(options, option{label: "Other model ID…", note: "for example claude-sonnet-4-5", value: "\x00custom"})
	idx := indexOfValue(options, current)
	if current == "" {
		idx = indexOfValue(options, "inherit")
	}
	return newChoice("Model", options, idx, func(o option) tea.Cmd {
		switch o.value {
		case "\x00custom":
			return push(newPrompt("Model ID", "A full model ID.", "", func(v string) error {
				if !models.ValidAgentModel(v) {
					return fmt.Errorf("that does not look like a model ID (claude-…)")
				}
				return nil
			}, func(v string) tea.Cmd {
				e.agent.Set("model", v)
				return e.persist()
			}))
		case "inherit":
			e.agent.Set("model", "")
		default:
			e.agent.Set("model", o.value)
		}
		return e.persist()
	})
}

// toolsChoice lists the built-in tools as checkboxes, plus any other tool the
// agent already names (an MCP tool, for example), which stays checked.
func (e *agentEditor) toolsChoice(title, key string) screen {
	current := e.toolList(key)
	names := slices.Clone(builtinTools)
	for _, t := range current {
		if !slices.Contains(names, t) {
			names = append(names, t)
		}
	}
	options := make([]option, len(names))
	checked := map[int]bool{}
	for i, n := range names {
		options[i] = option{label: n, value: n}
		checked[i] = slices.Contains(current, n)
	}
	return newMulti(title, options, checked, func(picked []option) tea.Cmd {
		values := make([]string, len(picked))
		for i, o := range picked {
			values[i] = o.value
		}
		e.agent.Set(key, strings.Join(values, ", "))
		return e.persist()
	})
}

// toolList reads a tools field, written either as "Read, Grep" or as a list.
func (e *agentEditor) toolList(key string) []string {
	v, ok := e.agent.Get(key)
	if !ok || v == nil {
		return nil
	}
	if list, isList := v.([]any); isList {
		var out []string
		for _, item := range list {
			out = append(out, fmt.Sprint(item))
		}
		return out
	}
	var out []string
	for _, t := range strings.Split(fmt.Sprint(v), ",") {
		if t = strings.TrimSpace(t); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// str is a frontmatter value as text, "" when absent.
func (e *agentEditor) str(key string) string {
	if v, ok := e.agent.Get(key); ok && v != nil {
		return fmt.Sprint(v)
	}
	return ""
}

// editPrompt opens the prompt in $EDITOR on a temporary file.
func (e *agentEditor) editPrompt() tea.Cmd {
	file, err := os.CreateTemp("", "squad-prompt-*.md")
	if err != nil {
		return flashErr(err.Error())
	}
	_, werr := file.WriteString(e.agent.Prompt())
	if cerr := file.Close(); werr != nil || cerr != nil {
		_ = os.Remove(file.Name())
		return flashErr("could not prepare the prompt for the editor")
	}
	path := file.Name()
	return tea.ExecProcess(editor.Command(path), func(err error) tea.Msg { return promptEditedMsg{path: path, err: err} })
}

func (e *agentEditor) promptEdited(m promptEditedMsg) tea.Cmd {
	defer func() { _ = os.Remove(m.path) }()
	if m.err != nil {
		return flashErr("The editor failed: " + m.err.Error() + " — set $EDITOR to one that works here")
	}
	text, err := os.ReadFile(m.path)
	if err != nil {
		return flashErr(err.Error())
	}
	e.agent.SetBody(string(text))
	return e.persist()
}

func (e *agentEditor) Hints() []hint {
	if e.agent != nil && agentFields[e.cursor].id == fPrompt {
		return []hint{{"↑↓", "field"}, {"enter", "write"}, {"o", "$EDITOR"}, {"f", "from file"}, {"pgup/dn", "scroll"}, {"p", "file"}, {"esc", "back"}, {"?", "help"}}
	}
	return []hint{{"↑↓", "move"}, {"enter", "edit"}, {"p", "file preview"}, {"esc", "back"}, {"?", "help"}}
}

func (e *agentEditor) viewNarrow(c viewCtx) string {
	name := e.bundle.Agents[e.idx].Name
	inner := c.w - 2
	title := "Agent " + name
	if e.loadErr != "" {
		lines := []string{
			" " + sErr.Render(truncate("✗ "+e.loadErr, inner-2)),
			"",
			" " + sMuted.Render("The file cannot be read as an agent. Fix it in your editor:"),
			"   squad edit " + e.bundle.Preset.Name + " " + name + " --raw",
		}
		return c.box(title, e.bundle.Preset.Name, lines, 0)
	}

	form := e.formLines(inner, c)
	checks := e.checkLines(inner)
	right := e.bundle.Preset.Name + " · " + fmt.Sprintf("slot %d", e.idx+1)

	if e.preview {
		return c.box(title+" · preview", right, e.previewLines(inner), 0)
	}
	lines := append(form, "")
	lines = append(lines, checks...)
	return c.box(title, right, lines, 0)
}

// info is the agent as the roster shows it.
func (e *agentEditor) info() workspace.AgentInfo {
	return workspace.AgentInfo{
		Name: e.agent.Name(), Model: e.str("model"), Effort: e.str("effort"), PermissionMode: e.str("permissionMode"),
		Color: e.str("color"), Tools: len(e.toolList("tools")), Role: e.agent.Role(),
	}
}

func (e *agentEditor) formLines(w int, c viewCtx) []string {
	var lines []string
	for i, f := range agentFields {
		prefix, label := "   ", sMuted.Render(pad(f.label, 18))
		if i == e.cursor {
			bar := fg(pulse(c.frame, brandStops[0], brandStops[1])).Render("▌")
			if !c.anim {
				bar = fg(brandStops[0]).Render("▌")
			}
			prefix, label = " "+bar+" ", sBold.Render(pad(f.label, 18))
		}
		lines = append(lines, truncate(prefix+label+e.valueOf(f.id), w))
	}
	return lines
}

func (e *agentEditor) valueOf(id fieldID) string {
	switch id {
	case fName:
		return sBold.Render(e.agent.Name())
	case fDescription:
		first, _, _ := strings.Cut(e.agent.Description(), "\n")
		return first
	case fModel:
		return modelPill(e.str("model"))
	case fEffort:
		return effortBars(e.str("effort"))
	case fPermission:
		return permissionStrength(e.str("permissionMode"))
	case fTools:
		return toolChips(e.toolList("tools"), "all tools")
	case fDisallowed:
		return toolChips(e.toolList("disallowedTools"), "—")
	case fMaxTurns:
		return orDash(e.str("maxTurns"), "—")
	case fColor:
		if c := e.str("color"); c != "" {
			return swatch(c) + " " + c
		}
		return "—"
	case fPrompt:
		lines := strings.Count(strings.TrimRight(e.agent.Prompt(), "\n"), "\n") + 1
		if strings.TrimSpace(e.agent.Prompt()) == "" {
			lines = 0
		}
		word := "lines"
		if lines == 1 {
			word = "line"
		}
		return fmt.Sprintf("%d %s %s", lines, word, sMuted.Render("· enter"))
	}
	return ""
}

// toolChips lists tools as small tags; when there are none it shows the
// fallback.
func toolChips(tools []string, none string) string {
	if len(tools) == 0 {
		return sMuted.Render(none)
	}
	const shown = 4
	chips := make([]string, 0, shown+1)
	for i, t := range tools {
		if i == shown {
			chips = append(chips, sMuted.Render(fmt.Sprintf("+%d", len(tools)-shown)))
			break
		}
		chips = append(chips, fg(cAccent).Render("‹")+t+fg(cAccent).Render("›"))
	}
	return strings.Join(chips, " ")
}

// checkLines shows what the linter says about this agent.
func (e *agentEditor) checkLines(w int) []string {
	lines := []string{" " + sMuted.Render("CHECKS")}
	shown := 0
	for _, f := range e.report.Findings {
		if f.Level == lint.LevelInfo {
			continue
		}
		if shown == 4 {
			lines = append(lines, "   "+sMuted.Render("…"))
			break
		}
		shown++
		lines = append(lines, "   "+findingMark(f.Level)+" "+truncate(f.Message, w-6))
	}
	if shown == 0 {
		lines = append(lines, "   "+sOK.Render("✓ nothing to fix"))
	}
	return lines
}

// previewLines is the file as it will be saved, with the frontmatter colored
// and the prompt numbered.
func (e *agentEditor) previewLines(w int) []string {
	lines := []string{" " + sMuted.Render("PREVIEW — "+e.bundle.Agents[e.idx].Name+".md")}
	dashes, body := 0, 0
	for _, l := range strings.Split(strings.TrimRight(string(e.data()), "\n"), "\n") {
		l = strings.ReplaceAll(l, "\t", "  ")
		switch {
		case dashes < 2 && l == "---":
			dashes++
			lines = append(lines, " "+fg(cDim).Render(l))
		case dashes < 2:
			lines = append(lines, truncate(" "+highlightYAML(l), w-1))
		default:
			body++
			lines = append(lines, truncate(fmt.Sprintf(" %s %s", fg(cDim).Render(fmt.Sprintf("%2d", body)), l), w-1))
		}
	}
	return lines
}

// highlightYAML colors the key of a `key: value` line.
func highlightYAML(line string) string {
	key, value, ok := strings.Cut(line, ":")
	if !ok || strings.HasPrefix(strings.TrimSpace(key), "-") {
		return line
	}
	return fg(agentColors["cyan"]).Render(key+":") + value
}

package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// modalWidth is the width of the dialogs, narrower on small terminals.
func modalWidth(c viewCtx) int { return max(min(64, c.w-6), 30) }

// modal draws a dialog box with the title and lines in the middle of the area.
// A dialog that is taller than the area loses its last lines.
func modal(c viewCtx, title string, lines []string) string {
	return modalSized(c, title, lines, modalWidth(c))
}

// choiceWidth is the width of a list of choices, which has room for notes.
func choiceWidth(c viewCtx) int { return max(min(100, c.w-6), 30) }

// modalSized is modal with a width of its own.
func modalSized(c viewCtx, title string, lines []string, w int) string {
	if room := c.h - 4; len(lines) > room {
		lines = append(lines[:max(room-1, 0)], sMuted.Render("…"))
	}
	rows := make([]string, 0, len(lines)+2)
	rows = append(rows, "")
	for _, l := range lines {
		rows = append(rows, " "+pad(l, w-4)+" ")
	}
	rows = append(rows, "")
	frame := box(title, "", rows, w, len(rows)+2, 0)
	return lipgloss.Place(c.w, c.h, lipgloss.Center, lipgloss.Center, frame)
}

// ---- prompt ----

// promptScreen asks for a line of text.
type promptScreen struct {
	title    string
	help     string
	input    textinput.Model
	validate func(string) error
	done     func(string) tea.Cmd
	err      string
}

func newPrompt(title, help, value string, validate func(string) error, done func(string) tea.Cmd) *promptScreen {
	in := textinput.New()
	in.Prompt = ""
	steady(&in)
	in.SetValue(value)
	in.CursorEnd()
	return &promptScreen{title: title, help: help, input: in, validate: validate, done: done}
}

func (p *promptScreen) Init() tea.Cmd { return p.input.Focus() }

func (p *promptScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "esc":
			return p, pop()
		case "enter":
			value := strings.TrimSpace(p.input.Value())
			if p.validate != nil {
				if err := p.validate(value); err != nil {
					p.err = err.Error()
					return p, nil
				}
			}
			return p, popThen(p.done(value))
		}
		p.err = ""
	}
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	return p, cmd
}

func (p *promptScreen) View(c viewCtx) string {
	p.input.SetWidth(modalWidth(c) - 9)
	lines := []string{}
	if p.help != "" {
		for _, l := range wrapText(p.help, modalWidth(c)-4) {
			lines = append(lines, sMuted.Render(l))
		}
		lines = append(lines, "")
	}
	lines = append(lines, "› "+p.input.View())
	if p.err != "" {
		lines = append(lines, "")
		lines = append(lines, wrapStyled(p.err, sErr, modalWidth(c)-4)...)
	}
	return modal(c, p.title, lines)
}

func (p *promptScreen) Hints() []hint {
	return []hint{{"enter", "confirm"}, {"esc", "cancel"}}
}

// ---- text area ----

// areaScreen edits longer text; ctrl+s keeps it.
type areaScreen struct {
	title string
	area  textarea.Model
	done  func(string) tea.Cmd
}

func newArea(title, value string, done func(string) tea.Cmd) *areaScreen {
	a := textarea.New()
	a.ShowLineNumbers = false
	a.Prompt = ""
	styles := a.Styles()
	styles.Cursor.Blink = false
	a.SetStyles(styles)
	a.SetValue(value)
	return &areaScreen{title: title, area: a, done: done}
}

func (a *areaScreen) Init() tea.Cmd { return a.area.Focus() }

func (a *areaScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "esc":
			return a, pop()
		case "ctrl+s":
			return a, popThen(a.done(strings.TrimSpace(a.area.Value())))
		}
	}
	var cmd tea.Cmd
	a.area, cmd = a.area.Update(msg)
	return a, cmd
}

func (a *areaScreen) View(c viewCtx) string {
	a.area.SetWidth(modalWidth(c) - 6)
	a.area.SetHeight(max(min(8, c.h-8), 3))
	return modal(c, a.title, strings.Split(a.area.View(), "\n"))
}

func (a *areaScreen) Hints() []hint {
	return []hint{{"ctrl+s", "keep"}, {"esc", "cancel"}}
}

// ---- choice ----

// option is one entry of a choice list.
type option struct {
	label    string
	note     string // shown after the label
	value    string
	disabled bool   // cannot be picked
	reason   string // why, for a disabled option
}

// choiceScreen lets the user pick one option, or several.
type choiceScreen struct {
	title    string
	options  []option
	cursor   int
	multi    bool
	checked  map[int]bool
	err      string
	done     func(option) tea.Cmd
	doneMany func([]option) tea.Cmd
}

// newChoice picks one option; current is the index preselected (-1 for none).
func newChoice(title string, options []option, current int, done func(option) tea.Cmd) *choiceScreen {
	return &choiceScreen{title: title, options: options, cursor: max(current, 0), done: done}
}

// newMulti picks any number of options; checked holds the preselected ones.
func newMulti(title string, options []option, checked map[int]bool, done func([]option) tea.Cmd) *choiceScreen {
	if checked == nil {
		checked = map[int]bool{}
	}
	return &choiceScreen{title: title, options: options, multi: true, checked: checked, doneMany: done}
}

func (s *choiceScreen) Init() tea.Cmd { return nil }

func (s *choiceScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return s, nil
	}
	s.err = ""
	switch key.String() {
	case "esc", "q":
		return s, pop()
	case "up", "k":
		s.cursor = max(s.cursor-1, 0)
	case "down", "j":
		s.cursor = min(s.cursor+1, len(s.options)-1)
	case "space", " ", "x":
		if s.multi && len(s.options) > 0 {
			s.checked[s.cursor] = !s.checked[s.cursor]
		}
	case "enter":
		if len(s.options) == 0 {
			return s, pop()
		}
		if s.multi {
			var picked []option
			for i, o := range s.options {
				if s.checked[i] {
					picked = append(picked, o)
				}
			}
			return s, popThen(s.doneMany(picked))
		}
		o := s.options[s.cursor]
		if o.disabled {
			s.err = fmt.Sprintf("%s is unavailable: %s", o.label, o.reason)
			return s, nil
		}
		return s, popThen(s.done(o))
	}
	return s, nil
}

func (s *choiceScreen) View(c viewCtx) string {
	w := choiceWidth(c) - 4
	labelW, notes := 0, false
	for _, o := range s.options {
		labelW = max(labelW, lipgloss.Width(o.label))
		notes = notes || o.note != "" || o.reason != ""
	}
	if notes {
		labelW = min(labelW, w/2) // the notes get the rest and wrap
	} else {
		labelW = min(labelW, w-8)
	}
	prefixW := 2 + labelW + 2
	if s.multi {
		prefixW += 4
	}

	// Every option is a block of lines: its label and the first line of its
	// note, then the rest of the note under it. Nothing is cut off.
	blocks := make([][]string, len(s.options))
	for i, o := range s.options {
		mark := "  "
		if i == s.cursor {
			mark = selBar() + " "
		}
		box := ""
		if s.multi {
			box = "[ ] "
			if s.checked[i] {
				box = sAccent.Render("[x]") + " "
			}
		}
		label := truncate(o.label, labelW)
		switch {
		case o.disabled:
			label = sMuted.Render(label)
		case i == s.cursor:
			label = sBold.Render(label)
		}
		note := o.note
		if o.disabled && o.reason != "" {
			note = strings.TrimSpace(note + "  " + o.reason)
		}
		text := wrapText(note, max(w-prefixW, 10))
		if len(text) == 0 {
			text = []string{""}
		}
		for k, t := range text {
			if k == 0 {
				blocks[i] = append(blocks[i], mark+box+pad(label, labelW+2)+sMuted.Render(t))
			} else {
				blocks[i] = append(blocks[i], strings.Repeat(" ", prefixW)+sMuted.Render(t))
			}
		}
	}

	room := max(c.h-9, 3)
	start := 0
	for used := 0; ; start++ {
		used = 0
		for i := start; i <= s.cursor && i < len(blocks); i++ {
			used += len(blocks[i])
		}
		if used <= room || start >= s.cursor {
			break
		}
	}
	var lines []string
	end := start
	for end < len(blocks) && len(lines)+len(blocks[end]) <= room {
		lines = append(lines, blocks[end]...)
		end++
	}
	if end <= s.cursor && end < len(blocks) { // a block taller than the room still shows its start
		lines = append(lines, blocks[end][:min(len(blocks[end]), max(room-len(lines), 1))]...)
		end++
	}
	if end-start < len(s.options) {
		lines = append(lines, sMuted.Render(fmt.Sprintf("  %d of %d", s.cursor+1, len(s.options))))
	}
	if s.err != "" {
		lines = append(lines, "", sErr.Render(truncate(s.err, w)))
	}
	return modalSized(c, s.title, lines, choiceWidth(c))
}

func (s *choiceScreen) Hints() []hint {
	if s.multi {
		return []hint{{"↑↓", "move"}, {"space", "toggle"}, {"enter", "keep"}, {"esc", "cancel"}}
	}
	return []hint{{"↑↓", "move"}, {"enter", "pick"}, {"esc", "cancel"}}
}

// ---- confirm ----

// confirmScreen asks a yes/no question.
type confirmScreen struct {
	title  string
	body   []string
	yes    string
	danger bool
	onYes  bool // focus is on the yes button
	done   func() tea.Cmd
}

func newConfirm(title string, body []string, yes string, danger bool, done func() tea.Cmd) *confirmScreen {
	return &confirmScreen{title: title, body: body, yes: yes, danger: danger, onYes: !danger, done: done}
}

func (s *confirmScreen) Init() tea.Cmd { return nil }

func (s *confirmScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return s, nil
	}
	switch key.String() {
	case "y":
		return s, popThen(s.done())
	case "n", "esc", "q":
		return s, pop()
	case "left", "right", "tab", "h", "l":
		s.onYes = !s.onYes
	case "enter":
		if s.onYes {
			return s, popThen(s.done())
		}
		return s, pop()
	}
	return s, nil
}

func (s *confirmScreen) View(c viewCtx) string {
	yesStyle := sAccent
	if s.danger {
		yesStyle = sErr
	}
	yes, no := "[ "+s.yes+" ]", "[ Cancel ]"
	if s.onYes {
		yes, no = yesStyle.Bold(true).Render(yes), sMuted.Render(no)
	} else {
		yes, no = sMuted.Render(yes), sAccent.Render(no)
	}
	lines := append(append([]string{}, s.body...), "", yes+"   "+no)
	return modal(c, s.title, lines)
}

func (s *confirmScreen) Hints() []hint {
	return []hint{{"y", "yes"}, {"n", "no"}, {"←→", "switch"}, {"enter", "pick"}}
}

// ---- notice ----

// noticeScreen tells something worth reading twice, such as where a file went,
// and stays until a key is pressed. A hint in the footer would be gone too soon.
type noticeScreen struct {
	title string
	lines []string
}

func newNotice(title string, lines []string) *noticeScreen {
	return &noticeScreen{title: title, lines: lines}
}

func (s *noticeScreen) Init() tea.Cmd { return nil }

func (s *noticeScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	if _, ok := msg.(tea.KeyPressMsg); ok {
		return s, pop()
	}
	return s, nil
}

func (s *noticeScreen) View(c viewCtx) string {
	w := choiceWidth(c) - 4
	return modalSized(c, s.title, wrapLines(s.lines, w), choiceWidth(c))
}

func (s *noticeScreen) Hints() []hint { return []hint{{"any key", "close"}} }

// steady gives a text input a cursor that does not blink: it needs no timers,
// and the screen stays the same between key presses.
func steady(in *textinput.Model) {
	styles := in.Styles()
	styles.Cursor.Blink = false
	in.SetStyles(styles)
}

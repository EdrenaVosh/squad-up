package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"

	"github.com/EdrenaVosh/squadup/internal/workspace"
)

// promptFromFile asks for a Markdown file and, after showing what would
// change, makes its text the prompt.
func (e *agentEditor) promptFromFile() tea.Cmd {
	return push(loadFileScreen(func(text, path string) tea.Cmd {
		old := strings.Count(strings.TrimRight(e.agent.Prompt(), "\n"), "\n") + 1
		if strings.TrimSpace(e.agent.Prompt()) == "" {
			old = 0
		}
		body := []string{
			fmt.Sprintf("Replace the prompt (%s) with", plural(old, "line")),
			fmt.Sprintf("%s (%s)?", lastElem(path), plural(strings.Count(text, "\n"), "line")),
			"",
			sMuted.Render("The old prompt is gone once you confirm; `squad undo` does not bring it back."),
		}
		return push(newConfirm("Load the prompt", body, "Replace", old > 0, func() tea.Cmd {
			e.agent.SetBody(text)
			return tea.Batch(e.persist(), flash("Prompt loaded from "+lastElem(path)))
		}))
	}))
}

func lastElem(path string) string {
	path = strings.TrimRight(path, "/")
	if i := strings.LastIndexAny(path, `/\`); i >= 0 {
		return path[i+1:]
	}
	return path
}

// loadFileScreen asks for the path of a Markdown file, then hands its text on.
func loadFileScreen(then func(text, path string) tea.Cmd) screen {
	return newPrompt("Load the prompt from a file",
		"The path of a Markdown file. Drop the file onto the terminal, or type it; ~ works. Its text becomes the prompt (frontmatter, if any, is ignored).",
		"", func(v string) error {
			_, err := workspace.ReadPromptFile(v)
			return err
		}, func(path string) tea.Cmd {
			text, err := workspace.ReadPromptFile(path)
			if err != nil {
				return flashErr(err.Error())
			}
			return then(text, path)
		})
}

// promptEditor is a full-size editor for the prompt of an agent.
type promptEditor struct {
	e    *agentEditor
	area textarea.Model
	orig string
}

func newPromptEditor(e *agentEditor) *promptEditor {
	a := textarea.New()
	a.Prompt = ""
	a.ShowLineNumbers = true
	a.MaxHeight = 0
	a.CharLimit = 0
	styles := a.Styles()
	styles.Cursor.Blink = false
	a.SetStyles(styles)
	text := strings.TrimRight(e.agent.Prompt(), "\n")
	a.SetValue(text)
	return &promptEditor{e: e, area: a, orig: text}
}

func (p *promptEditor) fullscreen() bool { return true }

func (p *promptEditor) Init() tea.Cmd { return p.area.Focus() }

func (p *promptEditor) dirty() bool { return strings.TrimRight(p.area.Value(), "\n") != p.orig }

func (p *promptEditor) Update(msg tea.Msg) (screen, tea.Cmd) {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "esc":
			if !p.dirty() {
				return p, pop()
			}
			return p, push(newConfirm("Discard your changes?", []string{"The prompt was changed and not saved."}, "Discard", true, func() tea.Cmd {
				return pop()
			}))
		case "ctrl+s":
			p.e.agent.SetBody(strings.TrimRight(p.area.Value(), "\n") + "\n")
			return p, popThen(tea.Batch(p.e.persist(), flash("Prompt saved")))
		case "ctrl+l":
			return p, push(loadFileScreen(func(text, path string) tea.Cmd {
				p.area.SetValue(strings.TrimRight(text, "\n"))
				return flash("Loaded " + lastElem(path) + ": press ctrl+s to keep it")
			}))
		}
	}
	var cmd tea.Cmd
	p.area, cmd = p.area.Update(msg)
	return p, cmd
}

func (p *promptEditor) View(c viewCtx) string {
	inner := c.w - 2
	p.area.SetWidth(inner - 2)
	p.area.SetHeight(max(c.h-6, 3))

	value := strings.TrimRight(p.area.Value(), "\n")
	lines := 0
	if strings.TrimSpace(value) != "" {
		lines = strings.Count(value, "\n") + 1
	}
	state := sMuted.Render("saved")
	if p.dirty() {
		state = sWarn.Render("● unsaved")
	}
	head := " " + sMuted.Render(fmt.Sprintf("%s · ~%d tokens", plural(lines, "line"), (len([]rune(value))+3)/4)) + "  " + state
	body := []string{head, " " + fg(cDim).Render(strings.Repeat("─", inner-2))}
	for _, l := range strings.Split(p.area.View(), "\n") {
		body = append(body, " "+l)
	}
	name := p.e.bundle.Agents[p.e.idx].Name
	return frame("Prompt · "+name, c.shift(), p.e.bundle.Preset.Name, body, c.w, c.h, 0)
}

func (p *promptEditor) Hints() []hint {
	return []hint{{"ctrl+s", "save"}, {"ctrl+l", "load a file"}, {"esc", "back"}}
}

package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/EdrenaVosh/squadup/internal/fsx"
	"github.com/EdrenaVosh/squadup/internal/transfer"
	"github.com/EdrenaVosh/squadup/internal/workspace"
	"github.com/EdrenaVosh/squadup/templates"
)

// ---- import ----

// importFetchedMsg carries a document read for the import preview.
type importFetchedMsg struct {
	source string
	data   []byte
	err    error
}

// importFlow asks where to read a preset from.
func importFlow() screen {
	notEmpty := func(s string) error {
		if s == "" {
			return fmt.Errorf("enter a file, a link or gist:<id>")
		}
		return nil
	}
	return newPrompt(ImportPromptTitle, ImportPromptHelp, "", notEmpty, fetchImport)
}

func fetchImport(source string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		data, err := transfer.Fetcher{}.Fetch(ctx, source)
		return importFetchedMsg{source: source, data: data, err: err}
	}
}

// importScreen previews a document and stores its presets on confirmation.
type importScreen struct {
	ws       workspace.Workspace
	source   string
	data     []byte
	opts     transfer.Options
	imported []transfer.Imported
	scroll   int
}

func newImportScreen(ws workspace.Workspace, source string, data []byte) (*importScreen, error) {
	s := &importScreen{ws: ws, source: source, data: data}
	if err := s.parse(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *importScreen) parse() error {
	imported, err := transfer.Parse(s.data, s.opts)
	if err != nil {
		return err
	}
	s.imported = imported
	return nil
}

func (s *importScreen) Init() tea.Cmd { return nil }

func (s *importScreen) dangerous() bool {
	for _, imp := range s.imported {
		if imp.Dangerous() {
			return true
		}
	}
	return false
}

func (s *importScreen) hasHooks() bool {
	for _, imp := range s.imported {
		if imp.HasHooks {
			return true
		}
	}
	return false
}

func (s *importScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return s, nil
	}
	switch key.String() {
	case "esc", "q":
		return s, pop()
	case "up", "k":
		s.scroll = max(s.scroll-1, 0)
	case "down", "j":
		s.scroll++
	case "h":
		if s.hasHooks() {
			s.opts.AllowHooks = !s.opts.AllowHooks
			if err := s.parse(); err != nil {
				return s, flashErr(err.Error())
			}
		}
	case "enter":
		if s.dangerous() {
			return s, push(s.consent())
		}
		return s, s.finish()
	}
	return s, nil
}

// consent asks again, in red, before a preset that switches prompts off.
func (s *importScreen) consent() screen {
	body := []string{ImportAnywayWarning, ""}
	for _, imp := range s.imported {
		for _, f := range imp.Findings {
			if f.Level == transfer.LevelDanger {
				body = append(body, wrapText(f.Message, modalWidth(viewCtx{w: 80})-4)...)
			}
		}
	}
	return newConfirm(ImportAnywayTitle, body, "Import anyway", true, s.finish)
}

// finish stores the presets and closes the preview.
func (s *importScreen) finish() tea.Cmd {
	var names []string
	for _, imp := range s.imported {
		res, err := s.ws.Import(imp.Bundle, workspace.ImportOptions{})
		if err != nil {
			return flashErr(err.Error())
		}
		if res.Renamed {
			names = append(names, fmt.Sprintf("%q as %q", imp.Bundle.Preset.Name, res.Name))
		} else {
			names = append(names, fmt.Sprintf("%q", res.Name))
		}
	}
	text := "Imported " + strings.Join(names, ", ")
	if len(names) > 3 {
		text = fmt.Sprintf("Imported %d presets", len(names))
	}
	return popThen(tea.Batch(reload(), flash(text)))
}

func (s *importScreen) Hints() []hint {
	hints := []hint{{"enter", "import"}, {"↑↓", "scroll"}}
	if s.hasHooks() {
		hints = append(hints, hint{"h", "hooks"})
	}
	return append(hints, hint{"esc", "cancel"})
}

func (s *importScreen) View(c viewCtx) string {
	inner := c.w - 2
	var lines []string
	for i, imp := range s.imported {
		if i > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, previewLines(imp, inner)...)
	}
	lines = append(lines, "", " "+sMuted.Render(ImportOnlyStored))
	if s.hasHooks() {
		state := "removed (h keeps them)"
		if s.opts.AllowHooks {
			state = "kept (h removes them)"
		}
		lines = append(lines, " "+sMuted.Render("Hooks and MCP servers: "+state))
	}
	lines = lines[min(s.scroll, max(len(lines)-1, 0)):]
	return c.box(ImportTitle, truncate(sourceLabel(s.source), c.w/2), lines, 0)
}

// sourceLabel shortens a file path to its name; links and gists stay whole.
func sourceLabel(source string) string {
	if strings.HasPrefix(source, "https://") || strings.HasPrefix(source, "gist:") {
		return source
	}
	return filepath.Base(source)
}

// previewLines draws one imported preset: its header, agents and findings.
func previewLines(imp transfer.Imported, w int) []string {
	d := transfer.Describe(imp.Bundle)
	title := d.Name
	if d.Title != "" && d.Title != d.Name {
		title += " — " + d.Title
	}
	main := "—"
	if d.Main != "" {
		main = d.Main
	}
	lines := []string{
		" " + sBold.Render(truncate(title, w-2)),
		" " + sMuted.Render(truncate(fmt.Sprintf("%s mode · %s · main agent %s", d.Mode, AgentCount(len(d.Agents)), main), w-2)),
		" " + sMuted.Render(truncate("settings: "+d.Settings, w-2)),
		"",
	}
	for _, a := range d.Agents {
		lines = append(lines, truncate("   "+pad(a.Name, 16)+pad(orDash(a.Model, "inherit"), 9)+pad(orDash(a.Tools, "all tools"), 20)+sMuted.Render(a.Role), w))
	}
	if len(imp.Findings) > 0 {
		lines = append(lines, "")
	}
	for _, f := range imp.Findings {
		label, style := "warn  ", sWarn
		if f.Level == transfer.LevelDanger {
			label, style = "DANGER", sErr.Bold(true)
		}
		wrapped := wrapText(strings.Replace(f.Message, transfer.AllowHooksHint, " (press h to keep it)", 1), w-12)
		for i, l := range wrapped {
			prefix := strings.Repeat(" ", 9)
			if i == 0 {
				prefix = " " + style.Render(label) + "  "
			}
			lines = append(lines, prefix+l)
		}
	}
	return lines
}

// ---- export ----

// exportFlow asks for the file to write a preset to. The suggestion is a whole
// path in the folder squad was started in, so there is no doubt where the file
// lands.
func exportFlow(ws workspace.Workspace, name string) screen {
	free := func(path string) error {
		if path == "" {
			return fmt.Errorf("enter a file name")
		}
		full, err := userPath(path)
		if err != nil {
			return err
		}
		if _, err := os.Stat(full); err == nil {
			return fmt.Errorf("already exists: %s", full)
		}
		if _, err := os.Stat(filepath.Dir(full)); err != nil {
			return fmt.Errorf("no such folder: %s", filepath.Dir(full))
		}
		return nil
	}
	cwd, _ := os.Getwd()
	help := "Saved in " + tildePath(cwd) + " unless you type a path (~/… works). Nothing is uploaded. The preset itself stays in " + tildePath(ws.Paths.PresetDir(name)) + "."
	return newPrompt(ExportPromptTitle, help, name+".squad.json", free, func(path string) tea.Cmd {
		path, _ = userPath(path)
		b, err := ws.Store().Load(name)
		if err != nil {
			return flashErr(err.Error())
		}
		secrets := transfer.ScanSecrets(*b)
		if len(secrets) == 0 {
			return writeExport(ws, name, path)
		}
		body := []string{"Remove them first, or export anyway.", ""}
		for i, sec := range secrets {
			if i == 3 {
				body = append(body, fmt.Sprintf("… and %d more", len(secrets)-3))
				break
			}
			body = append(body, truncate(fmt.Sprintf("%s.md line %d: %s", sec.Agent, sec.Line, sec.Snippet), modalWidth(viewCtx{w: 80})-4))
		}
		return push(newConfirm(ExportSecretsTitle, body, "Export anyway", true, func() tea.Cmd {
			return writeExport(ws, name, path)
		}))
	})
}

// userPath turns what was typed into an absolute path: a leading ~ is the home
// folder, anything relative starts in the working folder.
func userPath(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		p = filepath.Join(home, strings.TrimPrefix(p, "~"))
	}
	return filepath.Abs(p)
}

// tildePath writes a path under the home folder as ~/…, the way a person says it.
func tildePath(p string) string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if rest, ok := strings.CutPrefix(p, home+string(filepath.Separator)); ok {
			return "~" + string(filepath.Separator) + rest
		}
	}
	return p
}

func writeExport(ws workspace.Workspace, name, path string) tea.Cmd {
	b, err := ws.Store().Load(name)
	if err != nil {
		return flashErr(err.Error())
	}
	data, err := transfer.Export(*b, "squadup", ws.Now())
	if err != nil {
		return flashErr(err.Error())
	}
	if err := fsx.WriteFileAtomic(path, data, 0o644); err != nil {
		return flashErr(err.Error())
	}
	return push(newNotice("Exported", []string{
		sBold.Render(fmt.Sprintf("%q", name)) + " is saved as a file:",
		"",
		"  " + sAccent.Render(path),
		sMuted.Render(fmt.Sprintf("  %d bytes · nothing was uploaded", len(data))),
		"",
		sMuted.Render("Send the file to someone: they import it with `squad import " + filepath.Base(path) + "` or the i key here."),
		sMuted.Render("Your preset itself stays in " + tildePath(ws.Paths.PresetDir(name)) + "."),
	}))
}

// ---- templates ----

func templateOptions() []option {
	list, err := templates.List()
	if err != nil {
		return nil
	}
	out := make([]option, len(list))
	for i, t := range list {
		out[i] = option{label: t.Title, note: strings.Join(t.Agents, ", "), value: t.ID}
	}
	return out
}

// browseTemplates lists the starters, then asks for the name of the new preset.
func browseTemplates(ws workspace.Workspace) screen {
	return newChoice(TemplatesTitle, templateOptions(), 0, func(t option) tea.Cmd {
		return push(newPrompt(TemplateNameTitle, TemplateNameHelp, t.value, presetNameCheck(ws), func(name string) tea.Cmd {
			return createFromTemplate(ws, name, t)
		}))
	})
}

func createFromTemplate(ws workspace.Workspace, name string, t option) tea.Cmd {
	if err := ws.NewFromTemplate(t.value, name); err != nil {
		return flashErr(err.Error())
	}
	return tea.Batch(reload(), push(newPresetScreen(ws, name)), flash(fmt.Sprintf("Created %q from %s", name, t.label)))
}

package tui

import (
	"context"
	"fmt"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/EdrenaVosh/squadup/internal/apply"
	"github.com/EdrenaVosh/squadup/internal/workspace"
)

type equipState int

const (
	equipReview equipState = iota // the plan is shown, waiting for the decision
	equipDone                     // it was applied
	equipFailed                   // it could not be applied
)

// equipScreen shows what equipping a preset would change and applies it after
// the user agrees. For Initial it is the restore.
type equipScreen struct {
	ws       workspace.Workspace
	name     string
	opts     apply.Options
	plan     *apply.Plan
	warnings []string
	planErr  string
	onEquip  bool // focus is on the Equip button
	state    equipState
	result   *apply.Result
	failure  string
}

func newEquip(ws workspace.Workspace, name string) *equipScreen {
	e := &equipScreen{ws: ws, name: name, onEquip: true}
	e.replan()
	return e
}

func (e *equipScreen) replan() {
	e.plan, e.planErr = nil, ""
	plan, err := e.ws.Engine().PlanUse(e.name, e.opts)
	if err != nil {
		e.planErr = err.Error()
		return
	}
	e.plan = plan
	e.warnings = nil
	if e.name != "initial" {
		if b, err := e.ws.Store().Load(e.name); err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			e.warnings = e.ws.ModelWarnings(ctx, b)
		}
	}
	if m := os.Getenv("ANTHROPIC_MODEL"); m != "" && changesModel(plan) {
		e.warnings = append(e.warnings, fmt.Sprintf("ANTHROPIC_MODEL is set (%s): it overrides the model of every preset", m))
	}
}

func changesModel(p *apply.Plan) bool {
	for _, s := range p.Settings {
		if s.Key == "model" {
			return true
		}
	}
	return false
}

func (e *equipScreen) Init() tea.Cmd { return nil }

func (e *equipScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return e, nil
	}
	if e.state != equipReview || e.planErr != "" || (e.plan != nil && e.plan.NoOp) {
		return e, e.close()
	}
	switch key.String() {
	case "esc", "n", "q":
		return e, pop()
	case "left", "right", "tab", "h", "l":
		e.onEquip = !e.onEquip
	case "s":
		if e.plan != nil && len(e.plan.Drift) > 0 {
			e.opts.Edits = apply.EditsSave
		}
	case "d":
		if e.plan != nil && len(e.plan.Drift) > 0 {
			e.opts.Edits = apply.EditsDiscard
		}
	case "f":
		if e.plan != nil && len(e.plan.Conflicts) > 0 {
			e.opts.Force = true
			e.replan()
		}
	case "y":
		return e, e.apply()
	case "enter":
		if e.onEquip {
			return e, e.apply()
		}
		return e, pop()
	}
	return e, nil
}

// apply carries the plan out, unless something still needs a decision.
func (e *equipScreen) apply() tea.Cmd {
	switch {
	case e.plan == nil:
		return nil
	case e.plan.NoOp:
		return pop()
	case len(e.plan.Conflicts) > 0:
		return flashErr("Files are in the way: press f to overwrite them (a backup is taken first)")
	case len(e.plan.Drift) > 0 && e.opts.Edits == apply.EditsAsk:
		return flashErr("You edited files of the equipped preset: press s to save the edits into it, or d to discard them")
	}
	res, err := e.ws.Engine().Execute(e.plan, e.opts)
	if err != nil {
		e.state, e.failure = equipFailed, err.Error()
		return nil
	}
	e.state, e.result = equipDone, res
	return nil
}

// close leaves the screen after a result and refreshes the ones below.
func (e *equipScreen) close() tea.Cmd {
	if e.state == equipDone {
		text := fmt.Sprintf("Equipped %q — undo with `squad undo`", e.name)
		if e.name == "initial" {
			text = "Restored your original setup — undo with `squad undo`"
		}
		return popThen(tea.Batch(reload(), flash(text)))
	}
	return pop()
}

func (e *equipScreen) Hints() []hint {
	if e.state != equipReview || e.planErr != "" || (e.plan != nil && e.plan.NoOp) {
		return []hint{{"any key", "close"}}
	}
	hints := []hint{{"enter", "choose"}, {"←→", "switch"}, {"y", "equip"}}
	if e.plan != nil && len(e.plan.Drift) > 0 {
		hints = append(hints, hint{"s", "save edits"}, hint{"d", "discard"})
	}
	if e.plan != nil && len(e.plan.Conflicts) > 0 {
		hints = append(hints, hint{"f", "overwrite"})
	}
	return append(hints, hint{"esc", "cancel"}, hint{"n", "cancel"})
}

func (e *equipScreen) View(c viewCtx) string {
	inner := c.w - 2
	verb := "Equip"
	if e.name == "initial" {
		verb = "Restore"
	}
	title := fmt.Sprintf("%s %s", verb, e.name)
	if e.name == "initial" {
		title = "Restore your original setup"
	}

	var lines []string
	add := func(l ...string) { lines = append(lines, l...) }

	switch {
	case e.planErr != "":
		add("", " "+sErr.Render("✗ Cannot equip this preset:"), "")
		for _, l := range wrapText(e.planErr, inner-4) {
			add("   " + l)
		}
		return box(title, "", lines, c.w, c.h, 0)
	case e.state == equipDone:
		add("", " "+sOK.Render("✓ Done."), "")
		add(" " + sMuted.Render("A backup of the previous state was taken: "+e.result.BackupID))
		add(" " + sMuted.Render("`squad undo` goes back, `squad restore` returns your original setup."))
		for _, n := range e.plan.Notes {
			add("", " "+sWarn.Render(truncate("! "+n, inner-2)))
		}
		return box(title, "", lines, c.w, c.h, 0)
	case e.state == equipFailed:
		add("", " "+sErr.Render("✗ Nothing was changed."), "")
		for _, l := range wrapText(e.failure, inner-4) {
			add("   " + l)
		}
		return box(title, "", lines, c.w, c.h, 0)
	case e.plan.NoOp:
		add("", " "+sOK.Render("✓ "+nothingToChange(e.name)))
		return box(title, "", lines, c.w, c.h, 0)
	}

	add("")
	if len(e.plan.Files) == 0 && len(e.plan.Settings) == 0 {
		add(" " + sMuted.Render("No file changes: only the equipped preset changes."))
	}
	if len(e.plan.Files) > 0 {
		add(" " + sMuted.Render("AGENTS  ~/.claude/agents"))
		for _, op := range e.plan.Files {
			add("   " + planMark(op.Kind) + " " + trimPrefix(op.Path, "agents/"))
		}
		add("")
	}
	if len(e.plan.Settings) > 0 {
		add(" " + sMuted.Render("SETTINGS  settings.json"))
		for _, op := range e.plan.Settings {
			for _, l := range settingLines(op) {
				add(truncate("   "+l, inner))
			}
		}
		add("")
	}
	if len(e.plan.Drift) > 0 {
		add(" " + sWarn.Render("Manual edits detected in the equipped preset:"))
		for _, d := range e.plan.Drift {
			add(truncate(fmt.Sprintf("   ~ %s (%s)", d.Path, d.Kind), inner))
		}
		add(" "+e.editsChoice(), "")
	}
	if len(e.plan.Conflicts) > 0 {
		add(" " + sErr.Render("Files are in the way:"))
		for _, cf := range e.plan.Conflicts {
			add(truncate("   ! "+cf, inner))
		}
		add("")
	}
	for _, w := range append(append([]string{}, e.plan.Warnings...), e.warnings...) {
		add(truncate(" "+sWarn.Render("! "+w), inner))
	}
	for _, n := range e.plan.Notes {
		add(truncate(" "+sMuted.Render("• "+n), inner))
	}
	lines = fit(lines, inner, max(c.h-4, 1))
	lines = append(lines, "", e.buttons(inner))
	return box(title, e.plan.Mode, lines, c.w, c.h, 0)
}

func (e *equipScreen) editsChoice() string {
	switch e.opts.Edits {
	case apply.EditsSave:
		return sOK.Render("→ the edits will be saved into the preset first")
	case apply.EditsDiscard:
		return sOK.Render("→ the edits will be overwritten")
	}
	return sMuted.Render("s keeps them (saved into the preset), d overwrites them")
}

func (e *equipScreen) buttons(inner int) string {
	equip, cancel := "[ Equip ]", "[ Cancel ]"
	if e.name == "initial" {
		equip = "[ Restore ]"
	}
	if e.onEquip {
		equip, cancel = sAccent.Render(equip), sMuted.Render(cancel)
	} else {
		equip, cancel = sMuted.Render(equip), sAccent.Render(cancel)
	}
	return center(equip+"   "+cancel, inner)
}

func nothingToChange(name string) string {
	if name == "initial" {
		return "Already on your original setup — nothing to change."
	}
	return fmt.Sprintf("%q is already equipped — nothing to change.", name)
}

func planMark(k apply.OpKind) string {
	switch k {
	case apply.OpAdd:
		return sOK.Render("+")
	case apply.OpRemove:
		return sErr.Render("−")
	}
	return sWarn.Render("~")
}

// settingLines draws one change of settings.json. A long change, such as the
// permissions summary, goes on three lines instead of being cut off.
func settingLines(op apply.SettingOp) []string {
	head := planMark(op.Kind) + " " + op.Key
	switch op.Kind {
	case apply.OpAdd:
		return []string{head + ": " + sOK.Render(op.New)}
	case apply.OpRemove:
		return []string{head + ": " + sErr.Render(op.Old)}
	}
	if line := head + ": " + sErr.Render(op.Old) + " → " + sOK.Render(op.New); lipglossWidth(line) <= 70 {
		return []string{line}
	}
	return []string{head, "    " + sErr.Render(op.Old), "  → " + sOK.Render(op.New)}
}

func trimPrefix(s, prefix string) string {
	if len(s) > len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):]
	}
	return s
}

// wrapText breaks s into lines of at most w cells. A word that is longer than
// a line, such as a path, keeps its end, which is the part that says what it is.
func wrapText(s string, w int) []string {
	var out []string
	line := ""
	for _, word := range splitWords(s) {
		if lipglossWidth(word) > w {
			word = ansi.TruncateLeft(word, lipglossWidth(word)-(w-1), "…")
		}
		switch {
		case line == "":
			line = word
		case lipglossWidth(line)+1+lipglossWidth(word) <= w:
			line += " " + word
		default:
			out = append(out, line)
			line = word
		}
	}
	if line != "" {
		out = append(out, line)
	}
	return out
}

func splitWords(s string) []string {
	var words []string
	word := ""
	for _, r := range s {
		if r == ' ' || r == '\n' {
			if word != "" {
				words = append(words, word)
			}
			word = ""
			continue
		}
		word += string(r)
	}
	if word != "" {
		words = append(words, word)
	}
	return words
}

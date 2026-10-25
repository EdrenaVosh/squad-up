package tui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/EdrenaVosh/squadup/internal/preset"
	"github.com/EdrenaVosh/squadup/internal/team"
)

// The agent editor uses a wide layout from this size: the fields on the left,
// and on the right an inspector that shows the selected field in full.
const (
	wideEditorW = 100
	wideEditorH = 24
)

func (e *agentEditor) fullscreen() bool { return true }

// View draws the wide workbench, or the plain form on a small terminal.
func (e *agentEditor) View(c viewCtx) string {
	if e.loadErr == "" && c.w >= wideEditorW && c.h >= wideEditorH {
		return e.viewWide(c)
	}
	return e.viewNarrow(c)
}

func (e *agentEditor) viewWide(c viewCtx) string {
	inner := c.w - 2
	leftW := min(max(inner*36/100, 44), 58)
	rightW := inner - leftW - 1

	left := []string{" " + sMuted.Render("FIELDS"), ""}
	left = append(left, e.formLines(leftW, c)...)
	left = append(left, "")
	for _, l := range agentTile(e.info(), e.idx+1, tileOpts{big: true, w: min(leftW-4, bigTileW+8), stage: 2, team: e.together()}) {
		left = append(left, "  "+l)
	}
	left = append(left, "")
	left = append(left, e.checkLines(leftW)...)

	right := e.inspector(rightW, c.h-2, c)
	name := e.bundle.Agents[e.idx].Name
	label := fmt.Sprintf("%s · slot %d", e.bundle.Preset.Name, e.idx+1)
	return frame("Agent "+name, c.shift(), label, panes(left, right, leftW, inner), c.w, c.h, leftW)
}

// inspector is the right side: the selected field in full, or the file.
func (e *agentEditor) inspector(w, h int, c viewCtx) []string {
	f := agentFields[e.cursor]
	title, hint := f.label, e.fieldHint(f.id)
	var body []string
	switch {
	case e.preview:
		title, hint = "File preview", "p · back to the field"
		body = e.previewLines(w)[1:]
	default:
		body = e.fieldBody(f.id, w-2)
	}
	head := spread(" "+sBold.Render(title), sMuted.Render(hint)+" ", w)
	lines := []string{head, " " + fg(cDim).Render(strings.Repeat("─", w-2)), ""}

	room := max(h-len(lines), 1)
	scrolls := e.preview || f.id == fPrompt
	start := 0
	if scrolls {
		e.scroll = min(max(e.scroll, 0), max(len(body)-room, 0))
		start = e.scroll
	}
	end := min(start+room, len(body))
	for _, l := range body[start:end] {
		lines = append(lines, " "+l)
	}
	if scrolls && len(body) > room {
		note := fmt.Sprintf("lines %d–%d of %d · pgup/pgdn scroll", start+1, end, len(body))
		for len(lines) < h-1 {
			lines = append(lines, "")
		}
		lines = append(lines[:h-1], " "+sMuted.Render(note))
	}
	return lines
}

func (e *agentEditor) fieldHint(id fieldID) string {
	switch id {
	case fPrompt:
		return "enter write here · o $EDITOR · f from a file"
	case fName:
		return "enter · rename"
	case fTools, fDisallowed:
		return "enter · check tools"
	case fShare:
		return "enter or space · switch"
	}
	return "enter · change"
}

// fieldBody explains the selected field and shows all of its value.
func (e *agentEditor) fieldBody(id fieldID, w int) []string {
	switch id {
	case fName:
		return append([]string{sBold.Render(e.agent.Name()), ""},
			wrapAll(sMuted.Render("The name Claude sees, and the name of the file. Lowercase letters, digits and dashes, up to 64 characters; it must be unique in the preset."), w)...)
	case fDescription:
		return e.descriptionBody(w)
	case fShare:
		return e.shareBody(w)
	case fModel:
		return e.modelBody()
	case fEffort:
		return e.effortBody()
	case fPermission:
		return e.permissionBody()
	case fTools:
		return e.toolsBody("tools", "✓", w)
	case fDisallowed:
		return e.toolsBody("disallowedTools", "✗", w)
	case fMaxTurns:
		v := orDash(e.str("maxTurns"), "no limit")
		return append([]string{sBold.Render(v), ""}, wrapAll(sMuted.Render("How many steps the agent may take before it stops and reports. Leave it empty for no limit."), w)...)
	case fColor:
		return e.colorBody()
	case fPrompt:
		return e.promptBody(w)
	}
	return nil
}

func wrapAll(s string, w int) []string { return wrapText(s, w) }

func (e *agentEditor) descriptionBody(w int) []string {
	var out []string
	for _, l := range wrapText(e.agent.Description(), w-2) {
		out = append(out, fg(brandStops[1]).Render("▎ ")+l)
	}
	out = append(out, "", sMuted.Render("Claude delegates by reading this. Say what the agent does and when to call it (\"Use when…\")."), "")
	tips := 0
	for _, f := range e.report.Findings {
		if f.Rule == "short-description" || f.Rule == "no-trigger" || f.Rule == "similar-description" {
			out = append(out, findingLines(f.Level, f.Message, f.Fix, w)...)
			tips++
		}
	}
	if tips == 0 {
		out = append(out, sOK.Render("✓ it says what the agent does and when to use it"))
	}
	return out
}

// shareBody is the switch for the agent's context, what each side means, what
// this agent can do with the notes, and what they cost.
func (e *agentEditor) shareBody(w int) []string {
	shared := team.Shared(e.agent)
	on, off := sMuted.Render("○ shared"), sMuted.Render("○ isolated")
	if shared {
		on = sharePill(true)
	} else {
		off = sharePill(false)
	}
	out := []string{on + "   " + off, ""}

	choiceLine := func(selected bool, name, text string) string {
		mark := "  "
		if selected {
			mark = selBar() + " "
		}
		return mark + pad(sBold.Render(name), 11) + sMuted.Render(text)
	}
	out = append(out,
		choiceLine(shared, "Shared", "reads "+team.NotesFile+" first and leaves what it learned last"),
		choiceLine(!shared, "Isolated", "sees only the task it is given: for an independent review"),
		"")

	switch read, write := team.Access(e.agent); {
	case !e.together():
		out = append(out, sMuted.Render("Alone in this preset there is no one to share with. It starts to matter at two agents."))
	case !shared:
		out = append(out, sWarn.Render("⊘")+" It neither reads nor writes the notes, and the main agent hands it the task or the diff only.")
	case !read:
		out = append(out, sWarn.Render("!")+" Its tools cannot read files, so it gets no notes. Check Read in Tools.")
	case !write:
		out = append(out, sWarn.Render("!")+" Its tools can read but not write: it reads the notes and hands its own lines to the main agent.")
	default:
		out = append(out, sOK.Render("✓")+" It can read and write the notes.")
	}
	if shared && e.together() {
		out = append(out, sMuted.Render(fmt.Sprintf("Costs about %d tokens in its prompt, plus the notes themselves (kept to 25 short lines).", team.NotesTokens())))
	}
	out = append(out, "", sMuted.Render("What the notes look like"))
	for _, l := range team.ExampleNotes {
		out = append(out, "  "+highlightNote(truncate(l, w-4)))
	}
	return append(out, "", sMuted.Render("Why this format: only what git cannot tell — decisions and why, what failed, what is still broken. Fixed items are deleted, so no ! line means the work is clean."))
}

// highlightNote colors the tag of a line of the notes.
func highlightNote(l string) string {
	tag, rest, ok := strings.Cut(l, " ")
	if !ok {
		return l
	}
	switch tag {
	case "D":
		return sOK.Render(tag) + " " + rest
	case "X":
		return sMuted.Render(tag) + " " + sMuted.Render(rest)
	case "!":
		return sErr.Render(tag) + " " + rest
	case ">":
		return fg(cShared).Render(tag) + " " + rest
	}
	return sBold.Render(tag) + " " + rest
}

func (e *agentEditor) modelBody() []string {
	if e.modelOpts == nil {
		e.modelOpts, _ = modelOptions(e.ws, true, e.str("model"), "", "")
	}
	current := e.str("model")
	if current == "" {
		current = "inherit"
	}
	var out []string
	for _, o := range e.modelOpts {
		mark := "  "
		if o.value == current {
			mark = selBar() + " "
		}
		line := mark + pad(modelPill(o.label), 14)
		switch {
		case o.disabled:
			line += sErr.Render("unavailable: " + o.reason)
		case o.note != "":
			line += sMuted.Render(o.note)
		}
		out = append(out, line)
	}
	return append(out, "", sMuted.Render("inherit uses the model of the session. An alias follows the newest model of its family."))
}

var effortNotes = map[string]string{
	"low": "fastest and cheapest; for lookups and mechanical edits", "medium": "the balance for everyday work",
	"high": "careful; for reviews and planning", "xhigh": "slower and deeper; for hard problems", "max": "the most thorough the model offers",
}

func (e *agentEditor) effortBody() []string {
	current := e.str("effort")
	out := []string{}
	for _, level := range preset.EffortLevels {
		mark := "  "
		if level == current {
			mark = selBar() + " "
		}
		out = append(out, mark+pad(effortBars(level), 14)+sMuted.Render(effortNotes[level]))
	}
	note := "Not set: the model's own default."
	if current != "" {
		note = "Set to " + current + "."
	}
	return append(out, "", sMuted.Render(note))
}

var permissionNotes = map[string]string{
	"plan": "read-only: plans, never edits", "default": "asks before most actions", "acceptEdits": "edits files freely, asks for the rest",
	"auto": "a classifier approves the safe actions", "dontAsk": "never asks; anything not allowed is denied", "bypassPermissions": "asks for nothing — use with care",
}

func (e *agentEditor) permissionBody() []string {
	current := e.str("permissionMode")
	if current == "" {
		current = "default"
	}
	out := []string{}
	for _, mode := range []string{"plan", "default", "acceptEdits", "dontAsk", "auto", "bypassPermissions"} {
		mark := "  "
		if mode == current {
			mark = selBar() + " "
		}
		out = append(out, mark+pad(permissionStrength(mode), 24)+sMuted.Render(permissionNotes[mode]))
	}
	return out
}

func (e *agentEditor) toolsBody(key, mark string, w int) []string {
	current := e.toolList(key)
	names := slices.Clone(builtinTools)
	for _, t := range current {
		if !slices.Contains(names, t) {
			names = append(names, t)
		}
	}
	col := 18
	perRow := max(w/col, 1)
	var out []string
	for i := 0; i < len(names); i += perRow {
		var row strings.Builder
		for _, n := range names[i:min(i+perRow, len(names))] {
			cell := sMuted.Render("· ") + sMuted.Render(n)
			if slices.Contains(current, n) {
				cell = fg(cAccent).Render(mark+" ") + sBold.Render(n)
			}
			row.WriteString(pad(cell, col))
		}
		out = append(out, row.String())
	}
	note := "Nothing is checked: the agent can use every tool."
	if key == "disallowedTools" {
		note = "Nothing is blocked."
	}
	if len(current) > 0 {
		note = fmt.Sprintf("%d of %d checked.", len(current), len(names))
	}
	return append(out, "", sMuted.Render(note))
}

func (e *agentEditor) colorBody() []string {
	current := e.str("color")
	var out []string
	for _, c := range preset.Accents {
		mark := "  "
		if c == current {
			mark = selBar() + " "
		}
		out = append(out, mark+swatch(c)+" "+c)
	}
	return append(out, "", sMuted.Render("The color of the agent in Claude Code, and of its tile here."))
}

// promptBody shows the whole prompt, numbered, after a line of what to do.
func (e *agentEditor) promptBody(w int) []string {
	prompt := strings.TrimRight(e.agent.Prompt(), "\n")
	lines := strings.Split(prompt, "\n")
	if strings.TrimSpace(prompt) == "" {
		lines = nil
	}
	tokens := (len([]rune(prompt)) + 3) / 4
	out := []string{
		keycap("enter") + " write here   " + keycap("o") + " open in $EDITOR   " + keycap("f") + " load from a file",
		sMuted.Render(fmt.Sprintf("%s · %d characters · about %d tokens", plural(len(lines), "line"), len([]rune(prompt)), tokens)),
		"",
	}
	if len(lines) == 0 {
		return append(out, sWarn.Render("The prompt is empty: the agent has no instructions."),
			sMuted.Render("Write one here, or load a Markdown file with f."))
	}
	for i, l := range lines {
		l = strings.ReplaceAll(l, "\t", "  ")
		wrapped := highlightPrompt(strings.TrimRight(l, " "), w-6)
		for j, wl := range wrapped {
			num := "   "
			if j == 0 {
				num = fmt.Sprintf("%3d", i+1)
			}
			out = append(out, fg(cDim).Render(num)+" "+wl)
		}
	}
	return out
}

// scrollBy moves the inspector by delta lines.
func (e *agentEditor) scrollBy(delta int) { e.scroll = max(e.scroll+delta, 0) }

// handleScroll takes the keys that scroll the inspector.
func (e *agentEditor) handleScroll(key string) bool {
	switch key {
	case "pgdown", "ctrl+d":
		e.scrollBy(10)
	case "pgup", "ctrl+u":
		e.scrollBy(-10)
	case "home":
		e.scroll = 0
	case "end":
		e.scroll = 1 << 20
	default:
		return false
	}
	return true
}

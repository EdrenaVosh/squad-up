package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/EdrenaVosh/squadup/internal/apply"
	"github.com/EdrenaVosh/squadup/internal/workspace"
)

// panel draws lines inside a small box with its title in the top edge. The
// box is exactly w×h cells. Panels hold the squad's own settings, so they have
// square corners and a quiet edge; agents are rounded, colored tiles.
func panel(title string, lines []string, w, h int) []string {
	inner := w - 2
	edge := fg(cDim)
	head := " " + sMuted.Render(title) + " "
	fill := max(inner-1-ansi.StringWidth(head), 0)
	out := []string{edge.Render("┌─") + head + edge.Render(strings.Repeat("─", fill)+"┐")}
	for _, l := range fit(lines, inner-2, h-2) {
		out = append(out, edge.Render("│")+" "+l+" "+edge.Render("│"))
	}
	return append(out, edge.Render("└"+strings.Repeat("─", inner)+"┘"))
}

// sideBySide joins blocks of lines of the same height with a gap.
func sideBySide(gap int, blocks ...[]string) []string {
	if len(blocks) == 0 {
		return nil
	}
	h := 0
	for _, b := range blocks {
		h = max(h, len(b))
	}
	out := make([]string, h)
	for i := range out {
		parts := make([]string, len(blocks))
		for j, b := range blocks {
			if i < len(b) {
				parts[j] = b[i]
			} else {
				parts[j] = strings.Repeat(" ", ansi.StringWidth(b[0]))
			}
		}
		out[i] = strings.Join(parts, strings.Repeat(" ", gap))
	}
	return out
}

const panelH = 8 // the least height of the row of panels

// settingsPanel is the settings the preset sets.
func settingsPanel(c workspace.Card, w, h int) []string {
	row := func(label, value string) string { return sMuted.Render(pad(label, 12)) + value }
	perms := sMuted.Render("—")
	if c.Permissions != "" {
		mode, _, _ := strings.Cut(c.Permissions, " ")
		perms = permissionStrength(mode)
	}
	_, rules, _ := strings.Cut(c.Permissions, " · ")
	return panel("SETTINGS", []string{
		row("Model", modelOrDash(c.Model)),
		row("Effort", effortBars(c.Effort)),
		row("Permissions", perms),
		row("Main agent", orDash(c.Main, "—")),
		row("Mode", orDash(c.Mode, "—")),
		row("Rules", sMuted.Render(rules)),
	}, w, h)
}

// usagePanel charts the tokens each agent of the latest session has spent.
func usagePanel(f *usageFeed, w, h int) []string {
	rep := f.shown()
	title := "TOKENS · LIVE"
	if !rep.Found {
		return panel(title, []string{sMuted.Render("no session in this folder yet"), "", sMuted.Render("start `squad run` here and every agent's tokens show up as it works")}, w, h)
	}
	inner := w - 4
	nameW := min(12, max(inner/3, 6))
	numW := 6
	barW := max(inner-nameW-numW-3, 4)
	var lines []string
	total := max(rep.Total.Total(), 1)
	for i, a := range rep.Agents {
		if len(lines) >= h-5 {
			lines = append(lines, sMuted.Render(fmt.Sprintf("… +%d more", len(rep.Agents)-i)))
			break
		}
		mark := " "
		if a.Running > 0 {
			mark = fg(cClaude).Render("●")
		}
		lines = append(lines, mark+pad(truncate(a.Name, nameW), nameW)+" "+shareBar(float64(a.Tokens.Total())/float64(total), barW, f.tint(a.Name))+" "+fmt.Sprintf("%*s", numW, tokensShort(a.Tokens.Total())))
	}
	lines = append(lines, "", sBold.Render(tokensShort(rep.Total.Total()))+sMuted.Render(" tokens · u opens the report"))
	return panel(title, lines, w, h)
}

// healthPanel is what the linter says.
func healthPanel(c workspace.Card, w, h int) []string {
	lines := []string{lintWord(c)}
	for i, f := range c.Findings {
		wrapped := findingLines(f.Level, f.Message, f.Fix, w-4)
		if len(lines)+len(wrapped) > h-3 {
			lines = append(lines, sMuted.Render(fmt.Sprintf("… +%d more", len(c.Findings)-i)))
			break
		}
		lines = append(lines, wrapped...)
	}
	if len(c.Findings) == 0 {
		lines = append(lines, sMuted.Render("every agent says what it does and when to call it"))
	}
	return panel("HEALTH", lines, w, h)
}

// activityPanel lists the last operations that changed the config.
func activityPanel(entries []apply.JournalEntry, w, h int) []string {
	var lines []string
	for i := len(entries) - 1; i >= 0 && len(lines) < h-2; i-- {
		e := entries[i]
		lines = append(lines, truncate(fmt.Sprintf("%s  %s %s", sMuted.Render(e.Time.Local().Format("Jan 2 15:04")), pad(e.Command, 8), e.Preset), w-4))
	}
	if len(lines) == 0 {
		lines = []string{sMuted.Render("nothing yet — equip a preset")}
	}
	return panel("ACTIVITY", lines, w, h)
}

// previewPanel says what equipping the preset would change.
func previewPanel(plan *apply.Plan, planErr string, w, h int) []string {
	var lines []string
	switch {
	case planErr != "":
		lines = []string{sErr.Render(truncate("✗ "+planErr, w-4))}
	case plan == nil:
		lines = []string{sMuted.Render("—")}
	case plan.NoOp:
		lines = []string{sOK.Render("✓ already in place"), sMuted.Render("nothing would change")}
	default:
		for _, op := range plan.Files {
			lines = append(lines, truncate(planMark(op.Kind)+" "+trimPrefix(op.Path, "agents/"), w-4))
		}
		for _, op := range plan.Settings {
			for _, l := range settingLines(op) {
				lines = append(lines, truncate(l, w-4))
			}
		}
		if len(plan.Drift) > 0 {
			lines = append(lines, sWarn.Render(truncate(fmt.Sprintf("⚠ %s of yours would be asked about", plural(len(plan.Drift), "edit")), w-4)))
		}
		if len(plan.Conflicts) > 0 {
			lines = append(lines, sErr.Render(truncate(fmt.Sprintf("! %s in the way", plural(len(plan.Conflicts), "file")), w-4)))
		}
	}
	if len(lines) > h-2 {
		lines = append(lines[:h-3], sMuted.Render(fmt.Sprintf("… +%d more", len(lines)-h+3)))
	}
	return panel("IF YOU EQUIP IT · e", lines, w, h)
}

// panelsRow lays the panels out across w cells: as many as fit, each at least
// 34 cells wide.
func panelsRow(c workspace.Card, f *usageFeed, activity []apply.JournalEntry, plan *apply.Plan, planErr string, w, h int) []string {
	builders := []func(int) []string{
		func(pw int) []string { return settingsPanel(c, pw, h) },
		func(pw int) []string { return usagePanel(f, pw, h) },
		func(pw int) []string { return previewPanel(plan, planErr, pw, h) },
		func(pw int) []string { return healthPanel(c, pw, h) },
		func(pw int) []string { return activityPanel(activity, pw, h) },
	}
	n := max(min((w+1)/35, len(builders)), 1)
	pw := (w - (n - 1)) / n
	blocks := make([][]string, n)
	for i := 0; i < n; i++ {
		width := pw
		if i == n-1 {
			width = w - (n-1)*(pw+1)
		}
		blocks[i] = builders[i](width)
	}
	return sideBySide(1, blocks...)
}

// highlightPrompt colors a line of a prompt: headings, list marks and
// `**labels:**` stand out. Long lines wrap.
func highlightPrompt(line string, w int) []string {
	switch {
	case strings.TrimSpace(line) == "":
		return []string{""}
	case strings.HasPrefix(line, "#"):
		return []string{sBold.Render(fg(agentColors["cyan"]).Render(truncate(line, w)))}
	}
	var out []string
	for i, l := range wrapText(line, w-2) {
		l = strings.ReplaceAll(l, "**", "")
		if i == 0 && (strings.HasPrefix(strings.TrimSpace(line), "- ") || len(line) > 1 && line[0] >= '1' && line[0] <= '9' && line[1] == '.') {
			out = append(out, fg(brandStops[1]).Render("•")+" "+strings.TrimPrefix(strings.TrimPrefix(l, "- "), "-"))
			continue
		}
		out = append(out, "  "+l)
	}
	return out
}

// recent returns the last n journal entries, oldest first.
func recent(entries []apply.JournalEntry, n int) []apply.JournalEntry {
	if len(entries) > n {
		return entries[len(entries)-n:]
	}
	return entries
}

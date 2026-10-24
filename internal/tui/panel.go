package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/EdrenaVosh/squadup/internal/apply"
	"github.com/EdrenaVosh/squadup/internal/workspace"
)

// panel draws lines inside a small rounded box with its title in the top
// edge. The box is exactly w×h cells.
func panel(title string, lines []string, w, h int) []string {
	inner := w - 2
	edge := fg(cDim)
	head := " " + sMuted.Render(title) + " "
	fill := max(inner-1-ansi.StringWidth(head), 0)
	out := []string{edge.Render("╭─") + head + edge.Render(strings.Repeat("─", fill)+"╮")}
	for _, l := range fit(lines, inner-2, h-2) {
		out = append(out, edge.Render("│")+" "+l+" "+edge.Render("│"))
	}
	return append(out, edge.Render("╰"+strings.Repeat("─", inner)+"╯"))
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

// costPanel charts how many tokens each agent adds when it is called.
func costPanel(c workspace.Card, w, h int) []string {
	inner := w - 4
	maxTokens := 1
	for _, a := range c.Costs {
		maxTokens = max(maxTokens, a.Prompt)
	}
	nameW := min(14, max(inner/3, 6))
	barW := max(inner-nameW-6, 4)
	var lines []string
	for i, a := range c.Costs {
		if i == h-4 {
			lines = append(lines, sMuted.Render(fmt.Sprintf("… +%d more", len(c.Costs)-i)))
			break
		}
		filled := max(a.Prompt*barW/maxTokens, 1)
		colors := gradient(max(barW, 2))
		var bar strings.Builder
		for x := 0; x < barW; x++ {
			if x < filled {
				bar.WriteString(fg(colors[x]).Render("█"))
			} else {
				bar.WriteString(fg(cDim).Render("░"))
			}
		}
		lines = append(lines, pad(truncate(a.Name, nameW), nameW)+" "+bar.String()+" "+fmt.Sprintf("%4d", a.Prompt))
	}
	if len(c.Costs) == 0 {
		lines = append(lines, sMuted.Render("no agents"))
	}
	lines = append(lines, "", sMuted.Render(fmt.Sprintf("~%d tokens per session", c.SessionTokens)))
	return panel("CONTEXT COST · TOKENS/CALL", lines, w, h)
}

// healthPanel is what the linter says.
func healthPanel(c workspace.Card, w, h int) []string {
	lines := []string{lintWord(c)}
	for i, f := range c.Findings {
		if i == h-4 {
			lines = append(lines, sMuted.Render(fmt.Sprintf("… +%d more", len(c.Findings)-i)))
			break
		}
		lines = append(lines, truncate(findingMark(f.Level)+" "+f.Message, w-4))
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
func panelsRow(c workspace.Card, activity []apply.JournalEntry, plan *apply.Plan, planErr string, w, h int) []string {
	builders := []func(int) []string{
		func(pw int) []string { return settingsPanel(c, pw, h) },
		func(pw int) []string { return costPanel(c, pw, h) },
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

// agentDetail shows the selected agent in full: what it is for, its tools and
// the start of its prompt, as much as fits in h lines.
func agentDetail(a workspace.AgentInfo, w, h int) []string {
	if h < 3 {
		return nil
	}
	inner := w - 4
	var body []string
	if a.Description != "" {
		body = append(body, wrapText(a.Description, inner)...)
	}
	chips := "all tools"
	if len(a.ToolNames) > 0 {
		chips = toolChips(a.ToolNames, "all tools")
		if len(a.ToolNames) > 4 {
			chips = strings.Join(chipList(a.ToolNames), " ")
		}
	}
	body = append(body, sMuted.Render("Tools  ")+chips, "")
	for _, l := range strings.Split(a.Prompt, "\n") {
		body = append(body, highlightPrompt(strings.TrimRight(l, " \t"), inner)...)
	}
	title := "◆ " + a.Name
	return panel(title+" · PROMPT", body, w, min(h, len(body)+2))
}

func chipList(tools []string) []string {
	out := make([]string, len(tools))
	for i, t := range tools {
		out[i] = fg(cAccent).Render("‹") + t + fg(cAccent).Render("›")
	}
	return out
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

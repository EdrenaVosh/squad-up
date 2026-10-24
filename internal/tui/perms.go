package tui

import (
	"fmt"
	"slices"

	tea "charm.land/bubbletea/v2"

	"github.com/EdrenaVosh/squadup/internal/preset"
)

// permProfile is a ready-made set of permissions, so that nobody has to write
// the rules by hand.
type permProfile struct {
	name  string
	note  string
	build func() *preset.Permissions
}

var permProfiles = []permProfile{
	{
		name: "Careful",
		note: "asks before most actions and never reads your .env files",
		build: func() *preset.Permissions {
			return &preset.Permissions{DefaultMode: "default", Deny: []string{"Read(./.env)", "Read(./.env.*)"}}
		},
	},
	{
		name: "Balanced",
		note: "edits files freely, still asks before git push",
		build: func() *preset.Permissions {
			return &preset.Permissions{DefaultMode: "acceptEdits", Ask: []string{"Bash(git push *)"}}
		},
	},
	{
		name: "Autopilot",
		note: "a classifier approves safe actions, so there are few prompts",
		build: func() *preset.Permissions {
			return &preset.Permissions{DefaultMode: "auto"}
		},
	},
}

// profileOf names the profile that exactly matches the permissions: "" for
// none set, "Custom" for anything else.
func profileOf(p *preset.Permissions) string {
	if p == nil {
		return ""
	}
	for _, profile := range permProfiles {
		if samePermissions(p, profile.build()) {
			return profile.name
		}
	}
	return "Custom"
}

func samePermissions(a, b *preset.Permissions) bool {
	norm := func(l []string) []string {
		if len(l) == 0 {
			return nil
		}
		return l
	}
	return a.DefaultMode == b.DefaultMode &&
		slices.Equal(norm(a.Allow), norm(b.Allow)) && slices.Equal(norm(a.Ask), norm(b.Ask)) &&
		slices.Equal(norm(a.Deny), norm(b.Deny)) && slices.Equal(norm(a.AdditionalDirectories), norm(b.AdditionalDirectories)) &&
		len(a.Extra) == len(b.Extra)
}

// permsScreen picks a permission profile for a preset.
type permsScreen struct {
	ws     workspaceSaver
	bundle *preset.Bundle
	cursor int
}

// workspaceSaver is the part of the workspace the permission screens need.
type workspaceSaver interface {
	Save(preset.Bundle) error
}

func newPermsScreen(ws workspaceSaver, bundle *preset.Bundle) *permsScreen {
	s := &permsScreen{ws: ws, bundle: bundle}
	switch current := profileOf(bundle.Preset.Settings.Permissions); current {
	case "":
		s.cursor = 0
	case "Custom":
		s.cursor = len(permProfiles) + 1
	default:
		for i, p := range permProfiles {
			if p.name == current {
				s.cursor = i + 1
			}
		}
	}
	return s
}

func (s *permsScreen) Init() tea.Cmd { return nil }

func (s *permsScreen) count() int { return len(permProfiles) + 2 } // not set, profiles, custom

func (s *permsScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return s, nil
	}
	switch key.String() {
	case "esc", "q", "left":
		return s, pop()
	case "up", "k":
		s.cursor = max(s.cursor-1, 0)
	case "down", "j":
		s.cursor = min(s.cursor+1, s.count()-1)
	case "enter", "right", "l":
		return s, s.pick()
	}
	return s, nil
}

func (s *permsScreen) pick() tea.Cmd {
	switch {
	case s.cursor == 0:
		s.bundle.Preset.Settings.Permissions = nil
	case s.cursor <= len(permProfiles):
		s.bundle.Preset.Settings.Permissions = permProfiles[s.cursor-1].build()
	default:
		return push(newCustomPerms(s.ws, s.bundle))
	}
	if err := s.ws.Save(*s.bundle); err != nil {
		return flashErr(err.Error())
	}
	return pop()
}

func (s *permsScreen) View(c viewCtx) string {
	inner := c.w - 2
	current := profileOf(s.bundle.Preset.Settings.Permissions)
	names := []string{"Not set", "Careful", "Balanced", "Autopilot", "Custom…"}
	notes := []string{"use the permissions of my original setup"}
	for _, p := range permProfiles {
		notes = append(notes, p.note)
	}
	notes = append(notes, "write the allow, ask and deny rules myself")

	var lines []string
	lines = append(lines, "", " "+sMuted.Render("PERMISSION PROFILE"), "")
	for i, name := range names {
		mark := "  "
		if i == s.cursor {
			mark = selBar() + " "
		}
		tag := ""
		if (i == 0 && current == "") || name == current || (i == len(names)-1 && current == "Custom") {
			tag = sOK.Render(" (current)")
		}
		label := name
		if i == s.cursor {
			label = sBold.Render(name)
		}
		lines = append(lines, truncate("  "+mark+pad(label, 11)+sMuted.Render(notes[i])+tag, inner))
	}

	lines = append(lines, "", " "+sMuted.Render("WHAT IT SETS"))
	var shown *preset.Permissions
	switch {
	case s.cursor >= 1 && s.cursor <= len(permProfiles):
		shown = permProfiles[s.cursor-1].build()
	case s.cursor == len(permProfiles)+1:
		shown = s.bundle.Preset.Settings.Permissions
	}
	if shown == nil {
		lines = append(lines, "   "+sMuted.Render("nothing: Claude Code uses the permissions of your original setup"))
	} else {
		lines = append(lines, permissionLines(shown, inner)...)
	}
	return box("Permissions", s.bundle.Preset.Name, lines, c.w, c.h, 0)
}

// permissionLines lists a permission set, one rule per line.
func permissionLines(p *preset.Permissions, w int) []string {
	mode := p.DefaultMode
	if mode == "" {
		mode = "default"
	}
	lines := []string{"   " + sMuted.Render(pad("defaultMode", 13)) + styledMode(mode)}
	for _, list := range []struct {
		name  string
		rules []string
	}{{"allow", p.Allow}, {"ask", p.Ask}, {"deny", p.Deny}} {
		if len(list.rules) == 0 {
			continue
		}
		for i, r := range list.rules {
			label := ""
			if i == 0 {
				label = list.name
			}
			lines = append(lines, truncate("   "+sMuted.Render(pad(label, 13))+r, w))
		}
	}
	return lines
}

func styledMode(mode string) string {
	if mode == "bypassPermissions" {
		return sErr.Render(mode)
	}
	return mode
}

func (s *permsScreen) Hints() []hint {
	return []hint{{"↑↓", "move"}, {"enter", "pick"}, {"esc", "back"}}
}

// ---- custom permissions ----

// customPerms edits the mode and the allow, ask and deny rules by hand.
type customPerms struct {
	ws     workspaceSaver
	bundle *preset.Bundle
	work   *preset.Permissions // the rules being edited; they join the preset on the first save
	cursor int
}

func newCustomPerms(ws workspaceSaver, bundle *preset.Bundle) *customPerms {
	work := bundle.Preset.Settings.Permissions
	if work == nil {
		work = &preset.Permissions{DefaultMode: "default"}
	}
	return &customPerms{ws: ws, bundle: bundle, work: work}
}

type permRow struct {
	list string // "mode", "allow", "ask" or "deny"
	idx  int    // index of the rule, or -1 for the "add" row
}

func (c *customPerms) perms() *preset.Permissions { return c.work }

func (c *customPerms) rules(list string) *[]string {
	switch list {
	case "allow":
		return &c.perms().Allow
	case "ask":
		return &c.perms().Ask
	}
	return &c.perms().Deny
}

func (c *customPerms) rows() []permRow {
	rows := []permRow{{list: "mode", idx: -1}}
	for _, list := range []string{"allow", "ask", "deny"} {
		for i := range *c.rules(list) {
			rows = append(rows, permRow{list: list, idx: i})
		}
		rows = append(rows, permRow{list: list, idx: -1})
	}
	return rows
}

func (c *customPerms) Init() tea.Cmd { return nil }

func (c *customPerms) Update(msg tea.Msg) (screen, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return c, nil
	}
	rows := c.rows()
	c.cursor = min(c.cursor, len(rows)-1)
	switch key.String() {
	case "esc", "q", "left":
		return c, pop()
	case "up", "k":
		c.cursor = max(c.cursor-1, 0)
	case "down", "j":
		c.cursor = min(c.cursor+1, len(rows)-1)
	case "enter", "right", "l":
		return c, c.edit(rows[c.cursor])
	case "x", "delete":
		if r := rows[c.cursor]; r.list != "mode" && r.idx >= 0 {
			list := c.rules(r.list)
			*list = slices.Delete(*list, r.idx, r.idx+1)
			return c, c.save()
		}
	}
	return c, nil
}

func (c *customPerms) save() tea.Cmd {
	c.bundle.Preset.Settings.Permissions = c.work
	if err := c.ws.Save(*c.bundle); err != nil {
		return flashErr(err.Error())
	}
	return nil
}

func (c *customPerms) edit(r permRow) tea.Cmd {
	if r.list == "mode" {
		options := []option{}
		for _, mode := range preset.PermissionModes {
			o := option{label: mode, value: mode}
			if mode == "bypassPermissions" {
				o.note = "asks for nothing — dangerous"
			}
			options = append(options, o)
		}
		return push(newChoice("Default mode", options, indexOfValue(options, c.perms().DefaultMode), func(o option) tea.Cmd {
			apply := func() tea.Cmd {
				c.perms().DefaultMode = o.value
				return c.save()
			}
			if o.value != "bypassPermissions" {
				return apply()
			}
			body := []string{"bypassPermissions lets Claude Code run every tool", "without asking, including commands that delete files."}
			return push(newConfirm("Turn off permission prompts?", body, "Allow everything", true, apply))
		}))
	}
	value := ""
	if r.idx >= 0 {
		value = (*c.rules(r.list))[r.idx]
	}
	title := fmt.Sprintf("%s rule", r.list)
	return push(newPrompt(title, "For example Bash(npm run *), Read(./.env) or WebFetch(domain:example.com).", value, nonEmpty, func(v string) tea.Cmd {
		list := c.rules(r.list)
		if r.idx >= 0 {
			(*list)[r.idx] = v
		} else {
			*list = append(*list, v)
		}
		return c.save()
	}))
}

func nonEmpty(v string) error {
	if v == "" {
		return fmt.Errorf("a rule cannot be empty")
	}
	return nil
}

func (c *customPerms) View(v viewCtx) string {
	inner := v.w - 2
	p := c.perms()
	c.cursor = min(c.cursor, len(c.rows())-1)
	var lines []string
	cursorLine := func(i int) string {
		if i == c.cursor {
			return " " + selBar() + " "
		}
		return "   "
	}
	index := 0
	lines = append(lines, cursorLine(index)+sMuted.Render(pad("Default mode", 14))+styledMode(orDash(p.DefaultMode, "default")))
	index++
	for _, list := range []string{"allow", "ask", "deny"} {
		lines = append(lines, "", " "+sMuted.Render(listTitle(list)))
		for _, rule := range *c.rules(list) {
			lines = append(lines, truncate(cursorLine(index)+rule, inner))
			index++
		}
		lines = append(lines, cursorLine(index)+sAccent.Render("+")+" "+sMuted.Render("add rule"))
		index++
	}
	return box("Custom permissions", c.bundle.Preset.Name, lines, v.w, v.h, 0)
}

func listTitle(list string) string {
	switch list {
	case "allow":
		return "ALLOW — runs without asking"
	case "ask":
		return "ASK — always asks first"
	}
	return "DENY — never allowed"
}

func (c *customPerms) Hints() []hint {
	return []hint{{"↑↓", "move"}, {"enter", "edit"}, {"x", "delete rule"}, {"esc", "back"}}
}

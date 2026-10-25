package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/EdrenaVosh/squad-up/internal/agentfile"
	"github.com/EdrenaVosh/squad-up/internal/initial"
	"github.com/EdrenaVosh/squad-up/internal/lint"
	"github.com/EdrenaVosh/squad-up/internal/preset"
	"github.com/EdrenaVosh/squad-up/internal/settings"
	"github.com/EdrenaVosh/squad-up/internal/team"
)

// AgentInfo is what a slot of a preset shows about its agent.
type AgentInfo struct {
	Name           string
	Model          string
	Effort         string
	PermissionMode string
	Color          string
	Tools          int // how many tools it is limited to; 0 means all of them
	ToolNames      []string
	Role           string // the first sentence of its description
	Description    string
	Prompt         string // the system prompt, trimmed
	Independent    bool   // judges without knowing how the work was made (squadIndependent)
}

// Card is everything the terminal interface shows about one preset.
type Card struct {
	Entry
	Mode        string
	Main        string
	Model       string // the default model the preset sets
	Effort      string
	Permissions string // "acceptEdits · 4 allow · 1 ask · 2 deny", or "" when not set
	Agents      []AgentInfo
	Flow        *preset.Flow   // how the agents work as a team; nil when they do not
	Team        bool           // the preset has two agents or more, so what each is to the others matters
	Findings    []lint.Finding // errors and warnings; notes are only counted
	Errors      int
	Warnings    int
	Infos       int
}

// Card describes a preset, or "initial". A preset that cannot be read still
// gets a card; its Entry.Error says why.
func (w Workspace) Card(name string) (Card, error) {
	entries, err := w.Entries()
	if err != nil {
		return Card{}, err
	}
	idx := slices.IndexFunc(entries, func(e Entry) bool { return e.Name == name })
	if idx < 0 {
		return Card{}, fmt.Errorf("%w: %s", preset.ErrNotFound, name)
	}
	c := Card{Entry: entries[idx], Agents: []AgentInfo{}}

	report, err := w.Lint(name)
	if err != nil {
		return Card{}, err
	}
	c.Errors, c.Warnings, c.Infos = report.Count(lint.LevelError), report.Count(lint.LevelWarn), report.Count(lint.LevelInfo)
	for _, f := range report.Findings {
		if f.Level != lint.LevelInfo {
			c.Findings = append(c.Findings, f)
		}
	}

	switch {
	case c.Error != "":
	case name == "initial":
		err = w.fillInitial(&c)
	default:
		err = w.fillPreset(&c)
	}
	return c, err
}

func (w Workspace) fillPreset(c *Card) error {
	p, err := w.Store().LoadPreset(c.Name)
	if err != nil {
		return err
	}
	c.Mode, c.Model, c.Effort = p.Mode, p.Settings.Model, p.Settings.EffortLevel
	if p.Main != nil {
		c.Main = *p.Main
	}
	c.Permissions = p.Settings.Permissions.Summary()
	c.Flow = p.Flow
	c.Team = len(p.Agents) >= 2
	for _, name := range p.Agents {
		info := AgentInfo{Name: name}
		if data, err := os.ReadFile(filepath.Join(w.Paths.PresetDir(c.Name), "agents", name+".md")); err == nil {
			info = agentInfo(name, data)
		}
		c.Agents = append(c.Agents, info)
	}
	return nil
}

func (w Workspace) fillInitial(c *Card) error {
	meta, err := initial.Load(w.Paths)
	if err != nil {
		return err
	}
	managed := settings.ManagedFromRaw(meta.Managed)
	c.Mode = preset.ModeReplace
	c.Model, c.Effort, c.Main = managed.String("model"), managed.String("effortLevel"), managed.String("agent")
	c.Permissions = managed.PermissionsSummary()
	for _, entry := range meta.Agents {
		if entry.Issue != "" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(w.Paths.InitialDir(), "agents", filepath.FromSlash(entry.Path)))
		if err != nil {
			return err
		}
		c.Agents = append(c.Agents, agentInfo(entry.Name, data))
	}
	slices.SortFunc(c.Agents, func(a, b AgentInfo) int {
		switch {
		case a.Name < b.Name:
			return -1
		case a.Name > b.Name:
			return 1
		}
		return 0
	})
	return nil
}

func agentInfo(name string, data []byte) AgentInfo {
	info := AgentInfo{Name: name}
	a, err := agentfile.Parse(data)
	if err != nil {
		return info
	}
	info.Independent = team.Independent(a)
	get := func(key string) string {
		if v, ok := a.Get(key); ok && v != nil {
			return fmt.Sprint(v)
		}
		return ""
	}
	info.Model, info.Effort, info.PermissionMode, info.Color = get("model"), get("effort"), get("permissionMode"), get("color")
	if v, ok := a.Get("tools"); ok {
		switch list := v.(type) {
		case []any:
			for _, t := range list {
				info.ToolNames = append(info.ToolNames, fmt.Sprint(t))
			}
		case string:
			for _, t := range strings.Split(list, ",") {
				if t = strings.TrimSpace(t); t != "" {
					info.ToolNames = append(info.ToolNames, t)
				}
			}
		}
		info.Tools = len(info.ToolNames)
	}
	info.Role, info.Description, info.Prompt = a.Role(), a.Description(), strings.TrimSpace(a.Prompt())
	return info
}

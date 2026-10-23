package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/EdrenaVosh/squadup/internal/agentfile"
	"github.com/EdrenaVosh/squadup/internal/initial"
	"github.com/EdrenaVosh/squadup/internal/lint"
	"github.com/EdrenaVosh/squadup/internal/preset"
	"github.com/EdrenaVosh/squadup/internal/settings"
)

// AgentInfo is what a slot of a preset shows about its agent.
type AgentInfo struct {
	Name           string
	Model          string
	Effort         string
	PermissionMode string
	Color          string
}

// Card is everything the terminal interface shows about one preset.
type Card struct {
	Entry
	Mode          string
	Main          string
	Model         string // the default model the preset sets
	Effort        string
	Permissions   string // "acceptEdits · 4 allow · 1 ask · 2 deny", or "" when not set
	Agents        []AgentInfo
	Errors        int
	Warnings      int
	Infos         int
	SessionTokens int // estimated cost of the agents' names and descriptions
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
	c.SessionTokens = report.Cost.SessionTokens

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
	get := func(key string) string {
		if v, ok := a.Get(key); ok && v != nil {
			return fmt.Sprint(v)
		}
		return ""
	}
	info.Model, info.Effort, info.PermissionMode, info.Color = get("model"), get("effort"), get("permissionMode"), get("color")
	return info
}

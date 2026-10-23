package preset

import (
	"fmt"
	"slices"

	"github.com/EdrenaVosh/squadup/internal/agentfile"
	"github.com/EdrenaVosh/squadup/internal/paths"
	"github.com/EdrenaVosh/squadup/internal/settings"
)

// Skipped is an agent file that could not become part of a preset.
type Skipped struct {
	Path   string // relative to the agents dir
	Reason string
}

// InspectAgent returns the agent's name when content is a valid agent file,
// otherwise a short reason why it is not.
func InspectAgent(content []byte) (name, issue string) {
	a, err := agentfile.Parse(content)
	if err != nil {
		return "", err.Error()
	}
	switch {
	case a.Name() == "":
		return "", "missing name"
	case !ValidName(a.Name()):
		return "", fmt.Sprintf("invalid name %q", a.Name())
	case a.Description() == "":
		return "", "missing description"
	}
	return a.Name(), ""
}

// SettingsFromManaged converts the managed settings.json keys into preset
// settings and the main agent (nil when the `agent` key is absent). An
// unreadable permissions object is reported but the other values are kept.
func SettingsFromManaged(m settings.Managed) (Settings, *string, error) {
	s := Settings{Model: m.String("model"), EffortLevel: m.String("effortLevel")}
	var main *string
	if agent := m.String("agent"); agent != "" {
		main = &agent
	}
	var err error
	if m.Has("permissions") {
		s.Permissions, err = PermissionsFromJSON(m.Raw("permissions"))
	}
	return s, main, err
}

// FromConfig builds a preset named name from the live Claude Code setup:
// every valid agent file and the managed settings. Agent files are kept byte
// for byte; unusable ones are returned in Skipped instead of failing.
func FromConfig(p paths.Paths, name string) (Bundle, []Skipped, error) {
	files, _, err := agentfile.ScanDir(p.AgentsDir())
	if err != nil {
		return Bundle{}, nil, err
	}
	managed, _, err := settings.ReadManaged(p.SettingsFile())
	if err != nil {
		return Bundle{}, nil, err
	}
	settingsView, main, err := SettingsFromManaged(managed)
	if err != nil {
		return Bundle{}, nil, fmt.Errorf("%s: %w", p.SettingsFile(), err)
	}

	b := Bundle{Preset: Preset{
		Schema:      SchemaVersion,
		Name:        name,
		Title:       name,
		Description: "Captured from your Claude Code setup",
		Mode:        ModeReplace,
		Main:        main,
		Agents:      []string{},
		Settings:    settingsView,
	}}

	var skipped []Skipped
	taken := map[string]string{}
	for _, f := range files {
		agentName, issue := InspectAgent(f.Data)
		if issue == "" {
			if first, dup := taken[agentName]; dup {
				issue = fmt.Sprintf("duplicate name %q, already used by %s", agentName, first)
			}
		}
		if issue != "" {
			skipped = append(skipped, Skipped{Path: f.Path, Reason: issue})
			continue
		}
		taken[agentName] = f.Path
		b.Agents = append(b.Agents, AgentFile{Name: agentName, Data: f.Data})
	}
	slices.SortFunc(b.Agents, func(a, c AgentFile) int {
		switch {
		case a.Name < c.Name:
			return -1
		case a.Name > c.Name:
			return 1
		}
		return 0
	})
	for _, a := range b.Agents {
		b.Preset.Agents = append(b.Preset.Agents, a.Name)
	}
	return b, skipped, nil
}

package tui

import (
	"github.com/EdrenaVosh/squad-up/internal/workspace"
)

// newEditStart builds the screens `squad edit` opens: the preset, and above it
// the agent editor when an agent was named. A preset or agent that does not
// exist gives nil, so the interface opens on the main screen instead.
func newEditStart(ws workspace.Workspace, start Start) []screen {
	if start.Preset == "" || start.Preset == "initial" || !ws.Store().Exists(start.Preset) {
		return nil
	}
	p := newPresetScreen(ws, start.Preset)
	screens := []screen{p}
	if start.Agent == "" || p.bundle == nil {
		return screens
	}
	for i, a := range p.bundle.Agents {
		if a.Name == start.Agent {
			p.cursor = i
			return append(screens, newAgentEditor(ws, p.bundle, i))
		}
	}
	return screens
}

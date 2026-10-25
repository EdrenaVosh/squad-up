package workspace

import (
	"fmt"

	"github.com/EdrenaVosh/squadup/internal/agentfile"
	"github.com/EdrenaVosh/squadup/internal/preset"
	"github.com/EdrenaVosh/squadup/internal/team"
)

// SetIndependent makes one agent of a stored preset independent (it judges
// without knowing how the work was made), or an ordinary agent again.
func (w Workspace) SetIndependent(presetName, agent string, independent bool) error {
	if presetName == "initial" {
		return ErrProtected
	}
	b, err := w.Store().Load(presetName)
	if err != nil {
		return err
	}
	for i, file := range b.Agents {
		if file.Name != agent {
			continue
		}
		a, err := agentfile.Parse(file.Data)
		if err != nil {
			return fmt.Errorf("agent %s: %w", agent, err)
		}
		team.SetIndependent(a, independent)
		data, err := a.Marshal()
		if err != nil {
			return err
		}
		b.Agents[i].Data = data
		return w.Save(*b)
	}
	return fmt.Errorf("%w: preset %s has no agent %s", ErrNoAgent, presetName, agent)
}

// SetFlow sets the flow of a stored preset, or removes it when flow is nil.
func (w Workspace) SetFlow(presetName string, flow *preset.Flow) error {
	if presetName == "initial" {
		return ErrProtected
	}
	b, err := w.Store().Load(presetName)
	if err != nil {
		return err
	}
	b.Preset.Flow = flow
	return w.Save(*b)
}

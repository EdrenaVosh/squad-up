// Package templates holds the starter presets that ship inside the binary.
package templates

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/EdrenaVosh/squadup/internal/preset"
)

//go:embed all:bug-hunters all:feature-creator
var files embed.FS

// ErrUnknown means no template has the given id.
var ErrUnknown = errors.New("unknown template")

// order is how the templates are listed.
var order = []string{"bug-hunters", "feature-creator"}

// Template is a starter preset as shown in a list.
type Template struct {
	ID          string
	Title       string
	Description string
	Agents      []string
}

// List returns the starter templates in display order.
func List() ([]Template, error) {
	out := make([]Template, 0, len(order))
	for _, id := range order {
		p, err := loadPreset(id)
		if err != nil {
			return nil, err
		}
		out = append(out, Template{ID: id, Title: p.Title, Description: p.Description, Agents: p.Agents})
	}
	return out, nil
}

func loadPreset(id string) (*preset.Preset, error) {
	data, err := files.ReadFile(path.Join(id, "preset.yaml"))
	if err != nil {
		return nil, fmt.Errorf("template %s: %w", id, err)
	}
	p, err := preset.Unmarshal(data)
	if err != nil {
		return nil, fmt.Errorf("template %s: %w", id, err)
	}
	return p, nil
}

// Load returns a template as a bundle named after its id. Every call returns
// fresh data that the caller may change.
func Load(id string) (preset.Bundle, error) {
	known := false
	for _, o := range order {
		known = known || o == id
	}
	if !known {
		return preset.Bundle{}, fmt.Errorf("%w %q (available: %s)", ErrUnknown, id, strings.Join(order, ", "))
	}
	p, err := loadPreset(id)
	if err != nil {
		return preset.Bundle{}, err
	}
	b := preset.Bundle{Preset: *p}
	for _, name := range p.Agents {
		data, err := fs.ReadFile(files, path.Join(id, "agents", name+".md"))
		if err != nil {
			return preset.Bundle{}, fmt.Errorf("template %s: %w", id, err)
		}
		b.Agents = append(b.Agents, preset.AgentFile{Name: name, Data: data})
	}
	return b, nil
}

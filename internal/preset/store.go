package preset

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/EdrenaVosh/squadup/internal/fsx"
)

var (
	// ErrExists means a preset with that name is already stored.
	ErrExists = errors.New("preset already exists")
	// ErrNotFound means no preset with that name is stored.
	ErrNotFound = errors.New("preset not found")
)

// AgentFile is one agent of a preset, stored as agents/<Name>.md exactly as
// Claude Code expects it.
type AgentFile struct {
	Name string
	Data []byte
}

// Bundle is a preset together with its agent files.
type Bundle struct {
	Preset Preset
	Agents []AgentFile
}

// Store keeps presets as folders under Dir.
type Store struct {
	Dir string
}

// Names lists stored presets in alphabetical order.
func (s Store) Names() ([]string, error) {
	entries, err := os.ReadDir(s.Dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", s.Dir, err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// Exists reports whether a preset folder with this name exists.
func (s Store) Exists(name string) bool {
	if !ValidName(name) {
		return false
	}
	info, err := os.Stat(filepath.Join(s.Dir, name))
	return err == nil && info.IsDir()
}

// LoadPreset reads only preset.yaml, without the agent files.
func (s Store) LoadPreset(name string) (*Preset, error) {
	if !ValidName(name) {
		return nil, fmt.Errorf("%q is not a valid preset name (%s)", name, nameRE)
	}
	data, err := os.ReadFile(filepath.Join(s.Dir, name, "preset.yaml"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, name)
	}
	if err != nil {
		return nil, fmt.Errorf("read preset %s: %w", name, err)
	}
	p, err := Unmarshal(data)
	if err != nil {
		return nil, fmt.Errorf("preset %s: %w", name, err)
	}
	return p, nil
}

// Load reads preset.yaml and the agent files it lists. It reads presets in
// the flat agents/<name>.md layout; Initial keeps its own layout.
func (s Store) Load(name string) (*Bundle, error) {
	p, err := s.LoadPreset(name)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(s.Dir, name)

	b := &Bundle{Preset: *p}
	for _, agent := range p.Agents {
		if !ValidName(agent) {
			return nil, fmt.Errorf("preset %s lists invalid agent name %q", name, agent)
		}
		path := filepath.Join(dir, "agents", agent+".md")
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("preset %s: read agent file %s: %w", name, path, err)
		}
		b.Agents = append(b.Agents, AgentFile{Name: agent, Data: content})
	}
	return b, nil
}

// Save validates the bundle and writes it as a new preset. It never replaces
// an existing preset, and a failure leaves no partial folder behind.
func (s Store) Save(b Bundle) error {
	if err := b.Preset.Validate(); err != nil {
		return fmt.Errorf("invalid preset: %w", err)
	}
	if err := checkAgentsMatch(b); err != nil {
		return err
	}
	if s.Exists(b.Preset.Name) {
		return fmt.Errorf("%w: %s", ErrExists, b.Preset.Name)
	}

	yamlData, err := Marshal(b.Preset)
	if err != nil {
		return fmt.Errorf("encode preset.yaml: %w", err)
	}

	tmp := filepath.Join(s.Dir, "."+b.Preset.Name+".tmp")
	if err := fsx.RemoveAll(tmp); err != nil {
		return err
	}
	write := func() error {
		if err := fsx.WriteFileAtomic(filepath.Join(tmp, "preset.yaml"), yamlData, 0o600); err != nil {
			return err
		}
		for _, agent := range b.Agents {
			if err := fsx.WriteFileAtomic(filepath.Join(tmp, "agents", agent.Name+".md"), agent.Data, 0o600); err != nil {
				return err
			}
		}
		return fsx.Rename(tmp, filepath.Join(s.Dir, b.Preset.Name))
	}
	if err := write(); err != nil {
		_ = fsx.RemoveAll(tmp)
		return fmt.Errorf("save preset %s: %w", b.Preset.Name, err)
	}
	return nil
}

func checkAgentsMatch(b Bundle) error {
	have := map[string]bool{}
	for _, a := range b.Agents {
		if !ValidName(a.Name) {
			return fmt.Errorf("agent file name %q is not valid (%s)", a.Name, nameRE)
		}
		have[a.Name] = true
	}
	for _, name := range b.Preset.Agents {
		if !have[name] {
			return fmt.Errorf("agent %q is listed in the preset but has no file", name)
		}
	}
	for name := range have {
		if !slices.Contains(b.Preset.Agents, name) {
			return fmt.Errorf("agent file %q is not listed in the preset", name)
		}
	}
	return nil
}

// Replace overwrites an existing preset with b. The new version is built
// beside the old one and swapped in, so a failure leaves the old preset alone.
func (s Store) Replace(b Bundle) error {
	if err := b.Preset.Validate(); err != nil {
		return fmt.Errorf("invalid preset: %w", err)
	}
	if err := checkAgentsMatch(b); err != nil {
		return err
	}
	if !s.Exists(b.Preset.Name) {
		return fmt.Errorf("%w: %s", ErrNotFound, b.Preset.Name)
	}

	final := filepath.Join(s.Dir, b.Preset.Name)
	old := filepath.Join(s.Dir, "."+b.Preset.Name+".old")
	if err := fsx.RemoveAll(old); err != nil {
		return err
	}
	if err := fsx.Rename(final, old); err != nil {
		return fmt.Errorf("replace preset %s: %w", b.Preset.Name, err)
	}
	if err := s.Save(b); err != nil {
		if restoreErr := fsx.Rename(old, final); restoreErr != nil {
			return fmt.Errorf("%w (and the previous version is left in %s: %w)", err, old, restoreErr)
		}
		return err
	}
	return fsx.RemoveAll(old)
}

// Package apply changes the Claude Code config: it keeps the record of what
// was written and detects manual edits.
package apply

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"time"

	"github.com/EdrenaVosh/squadup/internal/fsx"
	"github.com/EdrenaVosh/squadup/internal/paths"
)

const stateSchema = 1

// State is the content of state.json: which preset is equipped and the
// sha256 of every file squad wrote for it.
type State struct {
	Schema    int               `json:"schema"`
	Active    string            `json:"active"`
	AppliedAt time.Time         `json:"appliedAt"`
	BackupID  string            `json:"backupId"`
	Manifest  map[string]string `json:"manifest"`
}

// ActivePreset is the equipped preset; before anything is applied that is
// Initial.
func (s State) ActivePreset() string {
	if s.Active == "" {
		return "initial"
	}
	return s.Active
}

// LoadState reads state.json. A missing file means nothing was applied yet.
func LoadState(p paths.Paths) (State, error) {
	data, existed, err := fsx.ReadFileIfExists(p.StateFile())
	if err != nil {
		return State{}, fmt.Errorf("read %s: %w", p.StateFile(), err)
	}
	if !existed {
		return State{}, nil
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return State{}, fmt.Errorf("%s is corrupt: %w", p.StateFile(), err)
	}
	return s, nil
}

// SaveState writes state.json atomically.
func SaveState(p paths.Paths, s State) error {
	if s.Schema == 0 {
		s.Schema = stateSchema
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}
	return fsx.WriteFileAtomic(p.StateFile(), append(data, '\n'), 0o600)
}

// DriftKind says how a file squad wrote differs from the disk.
type DriftKind string

// Kinds of drift.
const (
	DriftModified DriftKind = "modified"
	DriftMissing  DriftKind = "missing"
)

// DriftItem is one file that no longer matches the manifest.
type DriftItem struct {
	Path string // relative to the Claude config dir, slash-separated
	Kind DriftKind
}

// Drift compares the manifest of written files with the disk. Files that are
// not in the manifest are never reported: they are not squad's business.
func Drift(p paths.Paths, s State) ([]DriftItem, error) {
	rels := make([]string, 0, len(s.Manifest))
	for rel := range s.Manifest {
		rels = append(rels, rel)
	}
	slices.Sort(rels)

	var items []DriftItem
	for _, rel := range rels {
		data, existed, err := fsx.ReadFileIfExists(filepath.Join(p.ClaudeDir, filepath.FromSlash(rel)))
		if err != nil {
			return nil, err
		}
		switch {
		case !existed:
			items = append(items, DriftItem{Path: rel, Kind: DriftMissing})
		case fsx.SHA256Hex(data) != s.Manifest[rel]:
			items = append(items, DriftItem{Path: rel, Kind: DriftModified})
		}
	}
	return items, nil
}

// Package initial captures the user's original Claude Code setup so that
// squad can always put it back. The snapshot is stored byte for byte.
package initial

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/EdrenaVosh/squad-up/internal/agentfile"
	"github.com/EdrenaVosh/squad-up/internal/fsx"
	"github.com/EdrenaVosh/squad-up/internal/paths"
	"github.com/EdrenaVosh/squad-up/internal/preset"
	"github.com/EdrenaVosh/squad-up/internal/settings"
)

const (
	metaFileName         = "initial.json"
	originalSettingsName = "settings.orig.json"
	tmpDirName           = ".initial.tmp"
	metaSchema           = 1
	readOnly             = 0o400
)

var (
	// ErrExists means Initial was already captured.
	ErrExists = errors.New("initial snapshot already exists")
	// ErrNotCaptured means Initial has not been captured yet.
	ErrNotCaptured = errors.New("initial snapshot has not been captured yet")
)

// AgentEntry describes one captured agent file.
type AgentEntry struct {
	Path   string `json:"path"` // relative to the agents dir, slash-separated
	SHA256 string `json:"sha256"`
	Name   string `json:"name,omitempty"`  // set only for a valid agent
	Issue  string `json:"issue,omitempty"` // why the file is not a valid agent
}

// Meta is the record kept next to the copied files (initial.json).
type Meta struct {
	Schema              int               `json:"schema"`
	CapturedAt          time.Time         `json:"capturedAt"`
	ClaudeDir           string            `json:"claudeDir"`
	AgentsDirExisted    bool              `json:"agentsDirExisted"`
	SettingsFileExisted bool              `json:"settingsFileExisted"`
	SettingsInvalid     bool              `json:"settingsInvalid,omitempty"`
	SettingsSHA256      string            `json:"settingsSha256,omitempty"`
	Agents              []AgentEntry      `json:"agents"`
	Managed             map[string]string `json:"managed"` // managed key → raw JSON text, absent keys omitted
}

// Summary is what the first-run screen shows.
type Summary struct {
	Clean           bool // nothing to capture: Initial is Claude Code's defaults
	AgentNames      []string
	Model           string
	Effort          string
	Permissions     string
	Main            string
	Issues          int  // agent files (and settings.json) that are not valid
	SettingsInvalid bool // settings.json could not be parsed; counted in Issues
}

// Exists reports whether Initial has been captured.
func Exists(p paths.Paths) bool {
	_, err := os.Stat(filepath.Join(p.InitialDir(), metaFileName))
	return err == nil
}

// Load reads the record of the captured Initial.
func Load(p paths.Paths) (*Meta, error) {
	path := filepath.Join(p.InitialDir(), metaFileName)
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNotCaptured
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var m Meta
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("%s is corrupt: %w", path, err)
	}
	if m.Managed == nil {
		m.Managed = map[string]string{}
	}
	return &m, nil
}

// OriginalSettings returns the settings.json exactly as it was captured. The
// bool is false when the user had no settings.json.
func OriginalSettings(p paths.Paths) ([]byte, bool, error) {
	return fsx.ReadFileIfExists(filepath.Join(p.InitialDir(), originalSettingsName))
}

// Summary condenses the snapshot for display.
func (m *Meta) Summary() Summary {
	managed := settings.ManagedFromRaw(m.Managed)
	s := Summary{
		Model:       managed.String("model"),
		Effort:      managed.String("effortLevel"),
		Main:        managed.String("agent"),
		Permissions: managed.PermissionsSummary(),
		AgentNames:  m.validAgentNames(),
		Clean:       len(m.Agents) == 0 && len(m.Managed) == 0 && !m.SettingsInvalid,

		SettingsInvalid: m.SettingsInvalid,
	}
	for _, a := range m.Agents {
		if a.Issue != "" {
			s.Issues++
		}
	}
	if m.SettingsInvalid {
		s.Issues++
	}
	return s
}

func (m *Meta) validAgentNames() []string {
	var names []string
	for _, a := range m.Agents {
		if a.Issue == "" {
			names = append(names, a.Name)
		}
	}
	slices.Sort(names)
	return slices.Compact(names)
}

// Capture snapshots the current Claude Code setup into presets/initial. It
// only reads the config directory. It refuses to run when Initial exists.
func Capture(p paths.Paths, now time.Time) (*Meta, error) {
	if Exists(p) {
		return nil, ErrExists
	}

	meta := &Meta{
		Schema:     metaSchema,
		CapturedAt: now.UTC(),
		ClaudeDir:  p.ClaudeDir,
		Agents:     []AgentEntry{},
		Managed:    map[string]string{},
	}
	agentData, err := readAgents(p, meta)
	if err != nil {
		return nil, err
	}
	settingsData, err := readSettings(p, meta)
	if err != nil {
		return nil, err
	}

	view, err := presetView(meta)
	if err != nil {
		return nil, err
	}
	metaJSON, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode %s: %w", metaFileName, err)
	}

	tmp := filepath.Join(p.PresetsDir(), tmpDirName)
	write := func() error {
		if err := fsx.RemoveAll(tmp); err != nil {
			return err
		}
		for rel, data := range agentData {
			if err := fsx.WriteFileAtomic(filepath.Join(tmp, "agents", filepath.FromSlash(rel)), data, readOnly); err != nil {
				return err
			}
		}
		if meta.SettingsFileExisted {
			if err := fsx.WriteFileAtomic(filepath.Join(tmp, originalSettingsName), settingsData, readOnly); err != nil {
				return err
			}
		}
		if err := fsx.WriteFileAtomic(filepath.Join(tmp, "preset.yaml"), view, readOnly); err != nil {
			return err
		}
		if err := fsx.WriteFileAtomic(filepath.Join(tmp, metaFileName), append(metaJSON, '\n'), readOnly); err != nil {
			return err
		}
		return fsx.Rename(tmp, p.InitialDir())
	}
	if err := write(); err != nil {
		_ = fsx.RemoveAll(tmp)
		return nil, fmt.Errorf("capture initial snapshot: %w", err)
	}
	return meta, nil
}

// Recapture moves the existing Initial into backups and captures the current
// setup in its place. It returns the folder holding the old snapshot.
func Recapture(p paths.Paths, now time.Time) (string, *Meta, error) {
	if !Exists(p) {
		return "", nil, ErrNotCaptured
	}
	backup := filepath.Join(p.BackupsDir(), now.UTC().Format("2006-01-02T15-04-05")+"-initial")
	if _, err := os.Stat(backup); err == nil {
		return "", nil, fmt.Errorf("backup %s already exists; try again in a second", backup)
	}
	if err := fsx.MkdirAll(p.BackupsDir(), 0o700); err != nil {
		return "", nil, err
	}
	if err := fsx.Rename(p.InitialDir(), backup); err != nil {
		return "", nil, fmt.Errorf("back up the old snapshot: %w", err)
	}
	meta, err := Capture(p, now)
	if err != nil {
		if restoreErr := fsx.Rename(backup, p.InitialDir()); restoreErr != nil {
			return "", nil, fmt.Errorf("%w (and the old snapshot is left in %s: %w)", err, backup, restoreErr)
		}
		return "", nil, err
	}
	return backup, meta, nil
}

// readAgents loads every agent file into memory and records it in meta.
// Files are never repaired: broken ones are kept as they are and flagged.
func readAgents(p paths.Paths, meta *Meta) (map[string][]byte, error) {
	files, existed, err := agentfile.ScanDir(p.AgentsDir())
	if err != nil {
		return nil, err
	}
	meta.AgentsDirExisted = existed
	data := map[string][]byte{}
	for _, f := range files {
		name, issue := preset.InspectAgent(f.Data)
		meta.Agents = append(meta.Agents, AgentEntry{Path: f.Path, SHA256: fsx.SHA256Hex(f.Data), Name: name, Issue: issue})
		data[f.Path] = f.Data
	}
	return data, nil
}

// readSettings records the managed keys and returns the file bytes.
func readSettings(p paths.Paths, meta *Meta) ([]byte, error) {
	data, existed, err := fsx.ReadFileIfExists(p.SettingsFile())
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", p.SettingsFile(), err)
	}
	if !existed {
		return nil, nil
	}
	meta.SettingsFileExisted = true
	meta.SettingsSHA256 = fsx.SHA256Hex(data)

	managed, _, err := settings.ReadManaged(p.SettingsFile())
	if err != nil {
		meta.SettingsInvalid = true
		return data, nil
	}
	for _, key := range managed.Keys() {
		meta.Managed[key] = string(managed.Raw(key))
	}
	return data, nil
}

// presetView renders preset.yaml: Initial described as an ordinary preset so
// that it can be listed, shown and copied like any other. A permissions object
// that cannot be converted is left out of the view; the raw text in Meta is
// what restore uses.
func presetView(meta *Meta) ([]byte, error) {
	view, main, _ := preset.SettingsFromManaged(settings.ManagedFromRaw(meta.Managed))
	v := preset.Preset{
		Schema:      preset.SchemaVersion,
		Name:        "initial",
		Title:       "Initial",
		Description: "Your original Claude Code setup",
		Mode:        preset.ModeReplace,
		Main:        main,
		Agents:      meta.validAgentNames(),
		Settings:    view,
	}
	if v.Agents == nil {
		v.Agents = []string{}
	}
	data, err := preset.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("encode preset.yaml: %w", err)
	}
	return data, nil
}

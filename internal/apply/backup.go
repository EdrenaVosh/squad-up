package apply

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/EdrenaVosh/squadup/internal/fsx"
)

// backupsKept is how many operation backups are retained.
const backupsKept = 20

var backupIDRE = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2}(-\d+)?$`)

// BackupMeta describes one backup: the state of the config just before an
// operation changed it.
type BackupMeta struct {
	ID               string    `json:"id"`
	CreatedAt        time.Time `json:"createdAt"`
	Command          string    `json:"command"`
	Preset           string    `json:"preset"`
	AgentsDirExisted bool      `json:"agentsDirExisted"`
	SettingsExisted  bool      `json:"settingsExisted"`
	StateExisted     bool      `json:"stateExisted"`
}

// backup is a loaded backup.
type backup struct {
	meta     BackupMeta
	agents   map[string][]byte // every file of the agents dir, slash-separated relative path
	settings []byte
	state    []byte
}

// createBackup copies the agents dir (all files), settings.json and state.json
// into backups/<id>/. The folder appears atomically.
func (e Engine) createBackup(command, preset string, now time.Time) (BackupMeta, error) {
	base := now.UTC().Format("2006-01-02T15-04-05")
	id := base
	for n := 2; pathExists(filepath.Join(e.Paths.BackupsDir(), id)); n++ {
		id = fmt.Sprintf("%s-%d", base, n)
	}
	meta := BackupMeta{ID: id, CreatedAt: now.UTC(), Command: command, Preset: preset}

	tmp := filepath.Join(e.Paths.BackupsDir(), "."+id+".tmp")
	if err := fsx.RemoveAll(tmp); err != nil {
		return BackupMeta{}, err
	}
	write := func() error {
		if err := fsx.MkdirAll(tmp, 0o700); err != nil {
			return err
		}
		files, agentsDirExisted, err := readTree(e.Paths.AgentsDir())
		if err != nil {
			return err
		}
		meta.AgentsDirExisted = agentsDirExisted
		for rel, data := range files {
			if err := fsx.WriteFileAtomic(filepath.Join(tmp, "agents", filepath.FromSlash(rel)), data, 0o600); err != nil {
				return err
			}
		}
		for _, item := range []struct {
			src, name string
			flag      *bool
		}{
			{e.Paths.SettingsFile(), "settings.json", &meta.SettingsExisted},
			{e.Paths.StateFile(), "state.json", &meta.StateExisted},
		} {
			data, existed, err := fsx.ReadFileIfExists(item.src)
			if err != nil {
				return err
			}
			*item.flag = existed
			if existed {
				if err := fsx.WriteFileAtomic(filepath.Join(tmp, item.name), data, 0o600); err != nil {
					return err
				}
			}
		}
		metaJSON, err := json.MarshalIndent(meta, "", "  ")
		if err != nil {
			return err
		}
		if err := fsx.WriteFileAtomic(filepath.Join(tmp, "backup.json"), append(metaJSON, '\n'), 0o600); err != nil {
			return err
		}
		return fsx.Rename(tmp, filepath.Join(e.Paths.BackupsDir(), id))
	}
	if err := write(); err != nil {
		_ = fsx.RemoveAll(tmp)
		return BackupMeta{}, fmt.Errorf("back up your config to %s: %w", e.Paths.BackupsDir(), err)
	}
	return meta, nil
}

// Backups lists operation backups, oldest first. Folders without backup.json
// (for example the old Initial kept by a recapture) are not operation backups.
func (e Engine) Backups() ([]BackupMeta, error) {
	entries, err := os.ReadDir(e.Paths.BackupsDir())
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", e.Paths.BackupsDir(), err)
	}
	var metas []BackupMeta
	for _, entry := range entries {
		if !entry.IsDir() || !backupIDRE.MatchString(entry.Name()) {
			continue
		}
		data, existed, err := fsx.ReadFileIfExists(filepath.Join(e.Paths.BackupsDir(), entry.Name(), "backup.json"))
		if err != nil || !existed {
			continue
		}
		var meta BackupMeta
		if json.Unmarshal(data, &meta) != nil || meta.ID != entry.Name() {
			continue
		}
		metas = append(metas, meta)
	}
	slices.SortFunc(metas, func(a, b BackupMeta) int { return strings.Compare(a.ID, b.ID) })
	return metas, nil
}

func (e Engine) loadBackup(id string) (*backup, error) {
	if !backupIDRE.MatchString(id) {
		return nil, fmt.Errorf("%w: %q is not a backup id (see `squad log`)", ErrNoBackup, id)
	}
	dir := filepath.Join(e.Paths.BackupsDir(), id)
	metaData, existed, err := fsx.ReadFileIfExists(filepath.Join(dir, "backup.json"))
	if err != nil {
		return nil, fmt.Errorf("read backup %s: %w", id, err)
	}
	if !existed {
		return nil, fmt.Errorf("%w: %s (backups older than the last %d are removed)", ErrNoBackup, id, backupsKept)
	}
	b := &backup{}
	if err := json.Unmarshal(metaData, &b.meta); err != nil {
		return nil, fmt.Errorf("backup %s is damaged: %w", id, err)
	}
	if b.agents, _, err = readTree(filepath.Join(dir, "agents")); err != nil {
		return nil, fmt.Errorf("read backup %s: %w", id, err)
	}
	if b.settings, _, err = fsx.ReadFileIfExists(filepath.Join(dir, "settings.json")); err != nil {
		return nil, fmt.Errorf("read backup %s: %w", id, err)
	}
	if b.state, _, err = fsx.ReadFileIfExists(filepath.Join(dir, "state.json")); err != nil {
		return nil, fmt.Errorf("read backup %s: %w", id, err)
	}
	return b, nil
}

// pruneBackups removes all but the newest backupsKept operation backups.
func (e Engine) pruneBackups() error {
	metas, err := e.Backups()
	if err != nil {
		return err
	}
	for len(metas) > backupsKept {
		if err := fsx.RemoveAll(filepath.Join(e.Paths.BackupsDir(), metas[0].ID)); err != nil {
			return err
		}
		metas = metas[1:]
	}
	return nil
}

// readTree reads every regular file below root (following links to files),
// keyed by slash-separated relative path. The bool reports whether root is a
// directory.
func readTree(root string) (map[string][]byte, bool, error) {
	info, err := os.Stat(root)
	if errors.Is(err, fs.ErrNotExist) || (err == nil && !info.IsDir()) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	files := map[string][]byte{}
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			if target, err := os.Stat(path); err != nil || target.IsDir() {
				return nil
			}
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = data
		return nil
	})
	return files, true, err
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

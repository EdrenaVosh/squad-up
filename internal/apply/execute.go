package apply

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/EdrenaVosh/squad-up/internal/fsx"
	"github.com/EdrenaVosh/squad-up/internal/preset"
	"github.com/EdrenaVosh/squad-up/internal/settings"
)

// Result describes a finished operation.
type Result struct {
	// BackupID is the backup taken just before; empty when nothing was done.
	BackupID string
	Plan     *Plan
}

// Execute carries out a plan: lock, re-check, back up, write, record. Any
// failure while writing puts every touched file back. The plan is rebuilt
// under the lock and must match the one that was shown.
func (e Engine) Execute(plan *Plan, opts Options) (*Result, error) {
	if plan.NoOp {
		return &Result{Plan: plan}, nil
	}
	unlock, err := fsx.Lock(e.Paths.LockFile())
	if err != nil {
		return nil, err
	}
	defer unlock()

	fresh, err := e.replan(plan, opts)
	if err != nil {
		return nil, err
	}
	if fresh.signature() != plan.signature() {
		return nil, ErrStale
	}
	if len(fresh.Conflicts) > 0 {
		return nil, fmt.Errorf("%w:\n  %s", ErrConflicts, strings.Join(fresh.Conflicts, "\n  "))
	}
	if len(fresh.Drift) > 0 && opts.Edits == EditsAsk {
		return nil, fmt.Errorf("%w in %s", ErrDrift, driftList(fresh.Drift))
	}

	now := e.now()
	meta, err := e.createBackup(fresh.Command, fresh.Target, now)
	if err != nil {
		return nil, err
	}

	done := &progress{}
	if err := e.write(fresh, opts, meta, now, done); err != nil {
		if rbErr := e.rollback(fresh, done); rbErr != nil {
			return nil, fmt.Errorf("%w; restoring your files also failed (%w): they are saved in %s",
				err, rbErr, filepath.Join(e.Paths.BackupsDir(), meta.ID))
		}
		return nil, fmt.Errorf("%w (everything was put back as it was)", err)
	}
	_ = e.pruneBackups()
	return &Result{BackupID: meta.ID, Plan: fresh}, nil
}

func (e Engine) replan(plan *Plan, opts Options) (*Plan, error) {
	if plan.undo {
		return e.PlanUndo(plan.BackupID, opts)
	}
	return e.PlanUse(plan.name, opts)
}

func driftList(items []DriftItem) string {
	parts := make([]string, len(items))
	for i, d := range items {
		parts[i] = fmt.Sprintf("%s (%s)", d.Path, d.Kind)
	}
	return strings.Join(parts, ", ")
}

// progress records which steps completed, so that a rollback only reverts
// what was really done.
type progress struct {
	files    []FileOp
	settings bool
	state    bool
}

func (e Engine) abs(rel string) string {
	return filepath.Join(e.Paths.ClaudeDir, filepath.FromSlash(rel))
}

func (e Engine) write(p *Plan, opts Options, meta BackupMeta, now time.Time, done *progress) error {
	if len(p.Drift) > 0 && opts.Edits == EditsSave {
		if err := e.saveEdits(p); err != nil {
			return err
		}
	}

	var removed []string
	for _, op := range p.Files {
		if op.Kind != OpRemove {
			continue
		}
		if err := fsx.Remove(e.abs(op.Path)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("remove %s: %w", op.Path, err)
		}
		done.files = append(done.files, op)
		removed = append(removed, op.Path)
	}
	for _, op := range p.Files {
		if op.Kind == OpRemove {
			continue
		}
		if err := fsx.WriteFileAtomic(e.abs(op.Path), op.data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", op.Path, err)
		}
		done.files = append(done.files, op)
	}
	e.pruneEmptyDirs(removed)
	if p.removeAgentsDir {
		_ = fsx.Remove(e.Paths.AgentsDir()) // fails, harmlessly, unless it is empty
	}

	if err := e.writeSettings(p); err != nil {
		return err
	}
	done.settings = true

	if err := e.writeState(p, meta, now); err != nil {
		return err
	}
	done.state = true

	return e.appendJournal(JournalEntry{Time: now.UTC(), Command: p.Command, Preset: p.Target, Backup: meta.ID})
}

func (e Engine) writeSettings(p *Plan) error {
	path := e.Paths.SettingsFile()
	switch p.settingsAct {
	case settingsWrite:
		mode := os.FileMode(0o600)
		if info, err := os.Stat(path); err == nil {
			mode = info.Mode().Perm()
		}
		if err := fsx.WriteFileAtomic(path, p.settingsData, mode); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	case settingsRemove:
		if err := fsx.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("remove %s: %w", path, err)
		}
	}
	return nil
}

func (e Engine) writeState(p *Plan, meta BackupMeta, now time.Time) error {
	if p.undo {
		if p.stateWasAbsent {
			if err := fsx.Remove(e.Paths.StateFile()); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return err
			}
			return nil
		}
		return fsx.WriteFileAtomic(e.Paths.StateFile(), p.stateBytes, 0o600)
	}
	ns := *p.newState
	ns.AppliedAt = now.UTC()
	ns.BackupID = meta.ID
	return SaveState(e.Paths, ns)
}

// pruneEmptyDirs removes the directories that removing files left empty,
// stopping at the agents dir.
func (e Engine) pruneEmptyDirs(removedRel []string) {
	root := e.Paths.AgentsDir()
	for _, rel := range removedRel {
		dir := filepath.Dir(e.abs(rel))
		for dir != root && strings.HasPrefix(dir, root+string(filepath.Separator)) {
			if err := fsx.Remove(dir); err != nil {
				break
			}
			dir = filepath.Dir(dir)
		}
	}
}

// rollback puts back everything write completed.
func (e Engine) rollback(p *Plan, done *progress) error {
	var errs []error
	if done.state {
		if p.prevStateExisted {
			errs = append(errs, fsx.WriteFileAtomic(e.Paths.StateFile(), p.prevState, 0o600))
		} else {
			errs = append(errs, ignoreMissing(fsx.Remove(e.Paths.StateFile())))
		}
	}
	if done.settings && p.settingsAct != settingsKeep {
		if p.prevSettingsExisted {
			errs = append(errs, fsx.WriteFileAtomic(e.Paths.SettingsFile(), p.prevSettings, 0o600))
		} else {
			errs = append(errs, ignoreMissing(fsx.Remove(e.Paths.SettingsFile())))
		}
	}
	var added []string
	for i := len(done.files) - 1; i >= 0; i-- {
		op := done.files[i]
		if op.Kind == OpAdd {
			errs = append(errs, ignoreMissing(fsx.Remove(e.abs(op.Path))))
			added = append(added, op.Path)
			continue
		}
		errs = append(errs, fsx.WriteFileAtomic(e.abs(op.Path), op.old, 0o644))
	}
	e.pruneEmptyDirs(added)
	if p.agentsDirExisted {
		errs = append(errs, fsx.MkdirAll(e.Paths.AgentsDir(), 0o755))
	} else {
		_ = fsx.Remove(e.Paths.AgentsDir())
	}
	return errors.Join(errs...)
}

func ignoreMissing(err error) error {
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

// saveEdits stores the manual edits found in the active preset, so that they
// survive the switch.
func (e Engine) saveEdits(p *Plan) error {
	name := p.activePreset
	store := preset.Store{Dir: e.Paths.PresetsDir()}
	b, err := store.Load(name)
	if err != nil {
		return fmt.Errorf("save your edits into preset %q: %w", name, err)
	}
	for _, d := range p.Drift {
		switch {
		case strings.HasPrefix(d.Path, "agents/"):
			if err := e.saveAgentEdit(b, d); err != nil {
				return err
			}
		case strings.HasPrefix(d.Path, "settings.json:"):
			if err := e.saveSettingEdit(b, strings.TrimPrefix(d.Path, "settings.json:")); err != nil {
				return err
			}
		}
	}
	if err := store.Replace(*b); err != nil {
		return fmt.Errorf("save your edits into preset %q: %w", name, err)
	}
	return nil
}

func (e Engine) saveAgentEdit(b *preset.Bundle, d DriftItem) error {
	name := strings.TrimSuffix(strings.TrimPrefix(d.Path, "agents/"), ".md")
	if d.Kind == DriftMissing {
		b.Preset.Agents = removeString(b.Preset.Agents, name)
		b.Preset.PruneFlow()
		for i, a := range b.Agents {
			if a.Name == name {
				b.Agents = append(b.Agents[:i], b.Agents[i+1:]...)
				break
			}
		}
		return nil
	}
	data, err := os.ReadFile(e.abs(d.Path))
	if err != nil {
		return err
	}
	if got, issue := preset.InspectAgent(data); issue != "" || got != name {
		return fmt.Errorf("cannot save your edit of %s into the preset: the file no longer defines the agent %q (%s)", d.Path, name, issue)
	}
	for i := range b.Agents {
		if b.Agents[i].Name == name {
			b.Agents[i].Data = data
		}
	}
	return nil
}

func (e Engine) saveSettingEdit(b *preset.Bundle, key string) error {
	current, _, err := settings.ReadManaged(e.Paths.SettingsFile())
	if err != nil {
		return err
	}
	s := &b.Preset.Settings
	switch key {
	case "model":
		s.Model = current.String("model")
	case "effortLevel":
		s.EffortLevel = current.String("effortLevel")
	case "agent":
		b.Preset.Main = nil
		if agent := current.String("agent"); agent != "" {
			b.Preset.Main = &agent
		}
	case "permissions":
		s.Permissions = nil
		if current.Has("permissions") {
			perms, err := preset.PermissionsFromJSON(current.Raw("permissions"))
			if err != nil {
				return fmt.Errorf("cannot save your edit of permissions: %w", err)
			}
			s.Permissions = perms
		}
	}
	return nil
}

func removeString(list []string, s string) []string {
	out := list[:0:0]
	for _, item := range list {
		if item != s {
			out = append(out, item)
		}
	}
	return out
}

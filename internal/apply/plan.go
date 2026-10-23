package apply

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/EdrenaVosh/squadup/internal/agentfile"
	"github.com/EdrenaVosh/squadup/internal/fsx"
	"github.com/EdrenaVosh/squadup/internal/initial"
	"github.com/EdrenaVosh/squadup/internal/paths"
	"github.com/EdrenaVosh/squadup/internal/preset"
	"github.com/EdrenaVosh/squadup/internal/settings"
)

// Engine changes the Claude Code config. All writes go through Execute.
type Engine struct {
	Paths paths.Paths
	Now   func() time.Time // defaults to time.Now
}

func (e Engine) now() time.Time {
	if e.Now != nil {
		return e.Now()
	}
	return time.Now()
}

// EditsPolicy says what to do with files squad wrote that were edited by hand.
type EditsPolicy int

// What to do with manual edits.
const (
	EditsAsk     EditsPolicy = iota // stop and let the user decide
	EditsSave                       // store the edits in the current preset first
	EditsDiscard                    // overwrite them
)

// Options tune planning and execution.
type Options struct {
	// Force overwrites files at the target paths even if squad did not write
	// them. A backup is taken first.
	Force bool
	Edits EditsPolicy
}

// OpKind is the kind of a change.
type OpKind string

// Kinds of change.
const (
	OpAdd    OpKind = "add"
	OpModify OpKind = "modify"
	OpRemove OpKind = "remove"
)

// FileOp is one change to an agent file. Path is relative to the Claude
// config dir ("agents/planner.md").
type FileOp struct {
	Kind OpKind
	Path string

	data []byte // new content (add, modify)
	old  []byte // previous content (modify, remove)
}

// SettingOp is one change to a managed settings.json key. Old and New are
// compact JSON; empty means "absent".
type SettingOp struct {
	Key  string
	Kind OpKind
	Old  string
	New  string
}

type settingsAction int

const (
	settingsKeep settingsAction = iota
	settingsWrite
	settingsRemove
)

// Plan is everything an operation is going to do, computed without writing.
type Plan struct {
	Command   string // use, restore or undo
	Target    string // the preset equipped afterwards
	Mode      string // replace or extend, for presets
	Files     []FileOp
	Settings  []SettingOp
	Drift     []DriftItem // manual edits found; Execute needs a policy for them
	Conflicts []string    // reasons Execute refuses to run
	Warnings  []string
	Notes     []string // what to do after: restart claude, new sessions
	NoOp      bool

	// BackupID is the backup an undo returns to.
	BackupID string

	undo bool
	name string // preset to use

	activePreset string // Active in state.json before the operation

	settingsAct  settingsAction
	settingsData []byte

	prevSettings        []byte
	prevSettingsExisted bool
	prevState           []byte
	prevStateExisted    bool
	agentsDirExisted    bool
	removeAgentsDir     bool // remove the agents dir if the operation leaves it empty

	newState       *State // use and restore
	stateBytes     []byte // undo: exact state.json to put back
	stateWasAbsent bool   // undo: state.json must not exist afterwards
}

// signature identifies what a plan would do, so that Execute can tell whether
// the config changed between planning and writing.
func (p *Plan) signature() string {
	h := sha256.New()
	pr := func(args ...any) { _, _ = fmt.Fprintln(h, args...) }
	pr(p.Command, p.Target, p.NoOp)
	for _, f := range p.Files {
		pr(f.Kind, f.Path, fsx.SHA256Hex(f.data), fsx.SHA256Hex(f.old))
	}
	for _, s := range p.Settings {
		pr(s.Key, s.Kind, s.Old, s.New)
	}
	for _, d := range p.Drift {
		pr(d.Path, d.Kind)
	}
	for _, c := range p.Conflicts {
		pr(c)
	}
	pr(p.settingsAct, fsx.SHA256Hex(p.settingsData))
	return hex.EncodeToString(h.Sum(nil))
}

// desired is the state an operation moves the config to.
type desired struct {
	command string
	label   string // preset equipped afterwards ("initial" for Initial)
	mode    string

	files     map[string][]byte // agents-relative slash path → content
	presetRel map[string]string // paths that come from the preset → agent name
	conflicts []string
	checkName bool // agents of presets must not duplicate another agent's name

	settings map[string]string // every managed key that should be present → raw JSON
	explicit map[string]string // the subset the preset sets itself

	exact           []byte // settings.json to put back byte for byte, if untouched since squad wrote it
	haveExact       bool
	settingsExisted bool // whether settings.json exists in the target state

	agentsDirExisted bool

	undo        bool
	backupID    string
	name        string
	stateBytes  []byte
	stateAbsent bool
}

func (e Engine) base() (State, *initial.Meta, error) {
	st, err := LoadState(e.Paths)
	if err != nil {
		return State{}, nil, err
	}
	meta, err := initial.Load(e.Paths)
	if err != nil {
		return State{}, nil, fmt.Errorf("your original setup is not saved yet: %w (run any other squad command once to capture it)", err)
	}
	return st, meta, nil
}

// PlanUse plans equipping the preset, or restoring Initial for "initial".
// Problems the user can resolve (conflicts, manual edits) are reported in the
// plan, not as errors, so that --dry-run can show them.
func (e Engine) PlanUse(name string, opts Options) (*Plan, error) {
	st, meta, err := e.base()
	if err != nil {
		return nil, err
	}
	var d *desired
	if name == "initial" {
		d, err = e.desiredInitial(meta)
	} else {
		d, err = e.desiredPreset(name, meta)
	}
	if err != nil {
		return nil, err
	}
	d.name = name
	return e.buildPlan(d, st, meta, opts)
}

// PlanUndo plans returning to the state before the operation with backup id;
// an empty id means the most recent operation.
func (e Engine) PlanUndo(id string, opts Options) (*Plan, error) {
	st, meta, err := e.base()
	if err != nil {
		return nil, err
	}
	if id == "" {
		journal, err := e.Journal()
		if err != nil {
			return nil, err
		}
		for i := len(journal) - 1; i >= 0 && id == ""; i-- {
			id = journal[i].Backup
		}
		if id == "" {
			return nil, ErrNothingToUndo
		}
	}
	b, err := e.loadBackup(id)
	if err != nil {
		return nil, err
	}

	d := &desired{
		command:          "undo",
		label:            "initial",
		files:            map[string][]byte{},
		settings:         map[string]string{},
		exact:            b.settings,
		haveExact:        true,
		settingsExisted:  b.meta.SettingsExisted,
		agentsDirExisted: b.meta.AgentsDirExisted,
		undo:             true,
		backupID:         id,
		stateBytes:       b.state,
		stateAbsent:      !b.meta.StateExisted,
	}
	for rel, data := range b.agents {
		if strings.HasSuffix(rel, ".md") {
			d.files[rel] = data
		}
	}
	if b.meta.StateExisted {
		var past State
		if err := json.Unmarshal(b.state, &past); err != nil {
			return nil, fmt.Errorf("backup %s holds an unreadable state.json: %w", id, err)
		}
		d.label = past.ActivePreset()
	}
	if b.meta.SettingsExisted {
		restored, err := settings.ParseManaged(b.settings)
		if err != nil {
			return nil, fmt.Errorf("backup %s holds an unreadable settings.json: %w", id, err)
		}
		for _, key := range restored.Keys() {
			d.settings[key] = string(restored.Raw(key))
		}
	}
	return e.buildPlan(d, st, meta, opts)
}

func (e Engine) desiredInitial(meta *initial.Meta) (*desired, error) {
	files, err := e.initialFiles(meta)
	if err != nil {
		return nil, err
	}
	orig, existed, err := initial.OriginalSettings(e.Paths)
	if err != nil {
		return nil, err
	}
	return &desired{
		command:          "restore",
		label:            "initial",
		files:            files,
		settings:         maps(meta.Managed),
		exact:            orig,
		haveExact:        true,
		settingsExisted:  existed,
		agentsDirExisted: meta.AgentsDirExisted,
	}, nil
}

// loadValidBundle loads a preset and checks that it can be applied: the
// preset is valid and every agent file defines the agent it is listed as.
func (e Engine) loadValidBundle(name string) (*preset.Bundle, error) {
	b, err := preset.Store{Dir: e.Paths.PresetsDir()}.Load(name)
	if err != nil {
		return nil, err
	}
	if err := b.Preset.Validate(); err != nil {
		return nil, fmt.Errorf("preset %q is invalid: %w", name, err)
	}
	for _, a := range b.Agents {
		got, issue := preset.InspectAgent(a.Data)
		switch {
		case issue != "":
			return nil, fmt.Errorf("preset %q: agent %q has issues: %s", name, a.Name, issue)
		case got != a.Name:
			return nil, fmt.Errorf("preset %q: the file for agent %q declares the name %q", name, a.Name, got)
		}
	}
	return b, nil
}

// Resolve loads a preset and works out the managed settings it stands for:
// what the preset sets itself, and Initial's value for every key it leaves
// out. Keys are the raw JSON text. It writes nothing.
func (e Engine) Resolve(name string) (*preset.Bundle, map[string]string, error) {
	meta, err := initial.Load(e.Paths)
	if err != nil {
		return nil, nil, fmt.Errorf("your original setup is not saved yet: %w (run any other squad command once to capture it)", err)
	}
	b, err := e.loadValidBundle(name)
	if err != nil {
		return nil, nil, err
	}
	d, err := e.desiredFromBundle(b, meta)
	if err != nil {
		return nil, nil, err
	}
	return b, d.settings, nil
}

func (e Engine) desiredPreset(name string, meta *initial.Meta) (*desired, error) {
	b, err := e.loadValidBundle(name)
	if err != nil {
		return nil, err
	}
	return e.desiredFromBundle(b, meta)
}

func (e Engine) desiredFromBundle(b *preset.Bundle, meta *initial.Meta) (*desired, error) {
	name := b.Preset.Name
	var err error
	d := &desired{
		command:          "use",
		label:            name,
		mode:             b.Preset.Mode,
		files:            map[string][]byte{},
		presetRel:        map[string]string{},
		checkName:        true,
		settings:         maps(meta.Managed),
		explicit:         map[string]string{},
		agentsDirExisted: meta.AgentsDirExisted,
	}
	if b.Preset.Mode == preset.ModeExtend {
		if d.files, err = e.initialFiles(meta); err != nil {
			return nil, err
		}
	}
	for _, a := range b.Agents {
		rel := a.Name + ".md"
		if _, taken := d.files[rel]; taken {
			d.conflicts = append(d.conflicts, fmt.Sprintf("preset agent %q collides with your Initial agent agents/%s (mode extend); rename the agent or use mode replace", a.Name, rel))
		}
		d.files[rel] = a.Data
		d.presetRel[rel] = a.Name
	}

	set := func(key, raw string) {
		d.settings[key] = raw
		d.explicit[key] = raw
	}
	s := b.Preset.Settings
	if s.Model != "" {
		set("model", jsonString(s.Model))
	}
	if s.EffortLevel != "" {
		set("effortLevel", jsonString(s.EffortLevel))
	}
	if b.Preset.Main != nil {
		set("agent", jsonString(*b.Preset.Main))
	}
	if s.Permissions != nil {
		set("permissions", permissionsJSON(s.Permissions))
	}
	return d, nil
}

// initialFiles reads the Initial snapshot's agent files, verifying each hash.
func (e Engine) initialFiles(meta *initial.Meta) (map[string][]byte, error) {
	files := map[string][]byte{}
	for _, a := range meta.Agents {
		path := filepath.Join(e.Paths.InitialDir(), "agents", filepath.FromSlash(a.Path))
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read Initial snapshot file %s: %w", path, err)
		}
		if fsx.SHA256Hex(data) != a.SHA256 {
			return nil, fmt.Errorf("the Initial snapshot file %s was modified; it is read-only and must not change (run `squad initial --recapture` only if your current setup is the original one)", path)
		}
		files[a.Path] = data
	}
	return files, nil
}

func (e Engine) buildPlan(d *desired, st State, meta *initial.Meta, opts Options) (*Plan, error) {
	p := &Plan{
		Command:      d.command,
		Target:       d.label,
		Mode:         d.mode,
		undo:         d.undo,
		name:         d.name,
		BackupID:     d.backupID,
		activePreset: st.Active,
	}

	if d.command == "restore" && st.ActivePreset() == "initial" {
		p.NoOp = true
		return p, nil
	}

	curFiles, agentsDirExists, err := agentfile.ScanDir(e.Paths.AgentsDir())
	if err != nil {
		return nil, err
	}
	cur := map[string][]byte{}
	for _, f := range curFiles {
		cur[f.Path] = f.Data
	}
	p.agentsDirExisted = agentsDirExists
	p.removeAgentsDir = !d.agentsDirExisted

	initialSHA := map[string]string{}
	for _, a := range meta.Agents {
		initialSHA[a.Path] = a.SHA256
	}
	owned := func(rel string) bool { _, ok := st.Manifest["agents/"+rel]; return ok }
	pristine := func(rel string) bool {
		want, ok := initialSHA[rel]
		data, have := cur[rel]
		return ok && have && fsx.SHA256Hex(data) == want
	}
	removable := func(rel string) bool { return owned(rel) || pristine(rel) }

	p.Conflicts = append(p.Conflicts, d.conflicts...)

	rels := map[string]bool{}
	for rel := range d.files {
		rels[rel] = true
	}
	for rel := range cur {
		rels[rel] = true
	}
	sorted := make([]string, 0, len(rels))
	for rel := range rels {
		sorted = append(sorted, rel)
	}
	slices.Sort(sorted)

	final := map[string][]byte{}
	opFor := map[string]OpKind{}
	for _, rel := range sorted {
		want, wantOK := d.files[rel]
		have, haveOK := cur[rel]
		path := "agents/" + rel
		switch {
		case wantOK && !haveOK:
			p.Files = append(p.Files, FileOp{Kind: OpAdd, Path: path, data: want})
			opFor[rel] = OpAdd
			final[rel] = want
		case wantOK && bytes.Equal(want, have):
			final[rel] = have
		case wantOK && (removable(rel) || opts.Force):
			p.Files = append(p.Files, FileOp{Kind: OpModify, Path: path, data: want, old: have})
			opFor[rel] = OpModify
			final[rel] = want
		case wantOK:
			p.Conflicts = append(p.Conflicts, fmt.Sprintf("%s exists and squad did not write it (use --force to replace it; a backup is taken first)", path))
			final[rel] = have
		case haveOK && removable(rel):
			p.Files = append(p.Files, FileOp{Kind: OpRemove, Path: path, old: have})
			opFor[rel] = OpRemove
		case haveOK:
			final[rel] = have
			if _, wasInitial := initialSHA[rel]; wasInitial && !owned(rel) {
				p.Warnings = append(p.Warnings, fmt.Sprintf("kept %s: it differs from your original setup (edited by hand)", path))
			}
		}
	}

	if d.checkName {
		p.Conflicts = append(p.Conflicts, nameConflicts(d, final)...)
	}

	if err := e.planSettings(p, d, st); err != nil {
		return nil, err
	}

	if p.Drift, err = Drift(e.Paths, st); err != nil {
		return nil, err
	}

	// What is recorded after the operation.
	switch d.command {
	case "use", "restore":
		manifest := map[string]string{}
		for rel := range d.presetRel {
			data, ok := final[rel]
			if !ok {
				continue
			}
			if _, changed := opFor[rel]; changed || owned(rel) {
				manifest["agents/"+rel] = fsx.SHA256Hex(data)
			}
		}
		ns := &State{Schema: stateSchema, Active: d.label, Manifest: manifest}
		if d.label == "initial" {
			ns.Active = ""
		}
		if d.command == "use" {
			ns.Settings = &SettingsRecord{SHA256: sha(p.finalSettings()), Managed: maps(d.explicit)}
		}
		p.newState = ns
	case "undo":
		p.stateBytes = d.stateBytes
		p.stateWasAbsent = d.stateAbsent
	}

	if d.command == "use" && d.label == st.ActivePreset() && len(p.Files) == 0 && len(p.Settings) == 0 &&
		len(p.Drift) == 0 && len(p.Conflicts) == 0 && p.settingsAct == settingsKeep {
		p.NoOp = true
		return p, nil
	}

	p.Notes = notes(p, agentsDirExists)
	return p, nil
}

// finalSettings is settings.json as it will be after the operation.
func (p *Plan) finalSettings() []byte {
	switch p.settingsAct {
	case settingsWrite:
		return p.settingsData
	case settingsRemove:
		return nil
	}
	return p.prevSettings
}

func sha(data []byte) string {
	if data == nil {
		return ""
	}
	return fsx.SHA256Hex(data)
}

func notes(p *Plan, agentsDirExists bool) []string {
	var out []string
	if !agentsDirExists {
		for _, f := range p.Files {
			if f.Kind == OpAdd {
				out = append(out, "Claude Code does not pick up the first agent in a new agents folder until it restarts: restart claude.")
				break
			}
		}
	}
	for _, s := range p.Settings {
		if s.Key == "model" || s.Key == "agent" {
			out = append(out, "Model and main-agent changes apply to new sessions only.")
			break
		}
	}
	return out
}

// nameConflicts finds agents that would exist twice under the same name,
// which makes Claude Code load one of them at random.
func nameConflicts(d *desired, final map[string][]byte) []string {
	var out []string
	names := map[string]string{} // agent name → path, for files other than the preset's
	var rels []string
	for rel := range final {
		rels = append(rels, rel)
	}
	slices.Sort(rels)
	for _, rel := range rels {
		if _, fromPreset := d.presetRel[rel]; fromPreset {
			continue
		}
		if a, err := agentfile.Parse(final[rel]); err == nil && a.Name() != "" {
			if _, seen := names[a.Name()]; !seen {
				names[a.Name()] = rel
			}
		}
	}
	var presetRels []string
	for rel := range d.presetRel {
		presetRels = append(presetRels, rel)
	}
	slices.Sort(presetRels)
	for _, rel := range presetRels {
		name := d.presetRel[rel]
		if other, clash := names[name]; clash {
			out = append(out, fmt.Sprintf("agent %q is also defined by agents/%s; Claude Code would load only one of them", name, other))
		}
	}
	return out
}

// planSettings works out the settings.json changes: which managed keys change
// (for display) and the exact bytes to write.
func (e Engine) planSettings(p *Plan, d *desired, st State) error {
	path := e.Paths.SettingsFile()
	curData, existed, err := fsx.ReadFileIfExists(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	p.prevSettings, p.prevSettingsExisted = curData, existed
	if p.prevState, p.prevStateExisted, err = fsx.ReadFileIfExists(e.Paths.StateFile()); err != nil {
		return fmt.Errorf("read %s: %w", e.Paths.StateFile(), err)
	}
	current, _, err := settings.ReadManaged(path)
	if err != nil {
		return fmt.Errorf("%w; fix or restore the file before switching presets", err)
	}

	var edits []settings.Edit
	for _, key := range settings.ManagedKeys {
		want, wantOK := d.settings[key]
		have := current.Has(key)
		haveRaw := string(current.Raw(key))
		switch {
		case wantOK && !have:
			p.Settings = append(p.Settings, SettingOp{Key: key, Kind: OpAdd, New: display(key, want)})
			edits = append(edits, settings.Edit{Key: key, Raw: want})
		case !wantOK && have:
			p.Settings = append(p.Settings, SettingOp{Key: key, Kind: OpRemove, Old: display(key, haveRaw)})
			edits = append(edits, settings.Edit{Key: key, Delete: true})
		case wantOK && !settings.EqualJSON(want, haveRaw):
			p.Settings = append(p.Settings, SettingOp{Key: key, Kind: OpModify, Old: display(key, haveRaw), New: display(key, want)})
			edits = append(edits, settings.Edit{Key: key, Raw: want})
		}
	}

	untouched := d.haveExact && st.Settings != nil && existed && fsx.SHA256Hex(curData) == st.Settings.SHA256
	switch {
	case untouched && d.settingsExisted && !bytes.Equal(curData, d.exact):
		p.settingsAct, p.settingsData = settingsWrite, d.exact
	case untouched && !d.settingsExisted:
		p.settingsAct = settingsRemove
	case untouched:
	case len(edits) > 0:
		var base []byte
		if existed {
			base = curData
		}
		out, err := settings.Apply(base, edits)
		if err != nil {
			return fmt.Errorf("edit %s: %w", path, err)
		}
		p.settingsAct, p.settingsData = settingsWrite, out
	}
	return nil
}

func maps(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func jsonString(s string) string {
	data, _ := json.Marshal(s)
	return string(data)
}

// display shortens a raw value for the plan: scalars as compact JSON, the
// permissions object as "acceptEdits · 4 allow · 1 ask · 2 deny".
func display(key, raw string) string {
	if key == "permissions" {
		if summary := settings.ManagedFromRaw(map[string]string{key: raw}).PermissionsSummary(); summary != "" {
			return summary
		}
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, []byte(raw)); err != nil {
		return raw
	}
	return buf.String()
}

// permissionsJSON renders the permissions object indented to sit as a value
// of a top-level key in a two-space indented settings.json.
func permissionsJSON(perms *preset.Permissions) string {
	type member struct {
		key   string
		value any
	}
	var members []member
	if perms.DefaultMode != "" {
		members = append(members, member{"defaultMode", perms.DefaultMode})
	}
	for _, m := range []struct {
		key  string
		list []string
	}{{"allow", perms.Allow}, {"ask", perms.Ask}, {"deny", perms.Deny}, {"additionalDirectories", perms.AdditionalDirectories}} {
		if len(m.list) > 0 {
			members = append(members, member{m.key, m.list})
		}
	}
	extra := make([]string, 0, len(perms.Extra))
	for key := range perms.Extra {
		extra = append(extra, key)
	}
	sort.Strings(extra)
	for _, key := range extra {
		members = append(members, member{key, perms.Extra[key]})
	}
	if len(members) == 0 {
		return "{}"
	}

	var out strings.Builder
	out.WriteString("{")
	for i, m := range members {
		value, _ := json.MarshalIndent(m.value, "    ", "  ")
		if i > 0 {
			out.WriteString(",")
		}
		fmt.Fprintf(&out, "\n    %s: %s", jsonString(m.key), value)
	}
	out.WriteString("\n  }")
	return out.String()
}

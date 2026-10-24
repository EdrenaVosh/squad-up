// Package workspace is what the command line and the terminal interface share:
// listing presets, creating, copying, renaming and removing them, checking them
// and judging the models they use. It holds no screen or flag logic.
package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/EdrenaVosh/squadup/internal/agentfile"
	"github.com/EdrenaVosh/squadup/internal/apply"
	"github.com/EdrenaVosh/squadup/internal/claude"
	"github.com/EdrenaVosh/squadup/internal/fsx"
	"github.com/EdrenaVosh/squadup/internal/initial"
	"github.com/EdrenaVosh/squadup/internal/lint"
	"github.com/EdrenaVosh/squadup/internal/models"
	"github.com/EdrenaVosh/squadup/internal/paths"
	"github.com/EdrenaVosh/squadup/internal/preset"
	"github.com/EdrenaVosh/squadup/internal/settings"
	"github.com/EdrenaVosh/squadup/templates"
)

var (
	// ErrInvalidName means a preset name does not match the naming rule.
	ErrInvalidName = errors.New("preset names use lowercase letters, digits and dashes, start with a letter or digit, and have up to 64 characters")
	// ErrReservedName means the name belongs to the original setup.
	ErrReservedName = errors.New(`"initial" is reserved for your original setup`)
	// ErrProtected means Initial cannot be changed or removed.
	ErrProtected = errors.New("initial is your original setup and cannot be renamed or removed")
	// ErrNoClaude means the claude binary could not be found.
	ErrNoClaude = errors.New("claude was not found")
	// ErrNotLoggedIn means claude is installed but not signed in.
	ErrNotLoggedIn = errors.New("claude is not logged in")
	// ErrEquipped means the preset is equipped and removing it needs force.
	ErrEquipped = errors.New("the preset is equipped right now")
)

// Workspace is the squadup data of one Claude Code config directory.
type Workspace struct {
	Paths paths.Paths
	Now   func() time.Time // defaults to time.Now
}

func (w Workspace) now() time.Time {
	if w.Now != nil {
		return w.Now()
	}
	return time.Now()
}

// Store is the preset store.
func (w Workspace) Store() preset.Store { return preset.Store{Dir: w.Paths.PresetsDir()} }

// Engine applies presets to the Claude config.
func (w Workspace) Engine() apply.Engine { return apply.Engine{Paths: w.Paths, Now: w.now} }

// CheckName validates the name of a new preset.
func CheckName(name string) error {
	switch {
	case name == "initial":
		return ErrReservedName
	case !preset.ValidName(name):
		return fmt.Errorf("%w: %q", ErrInvalidName, name)
	}
	return nil
}

// locked runs fn while holding the squadup lock, so that nothing else changes
// the presets or the state meanwhile.
func (w Workspace) locked(fn func() error) error {
	unlock, err := fsx.Lock(w.Paths.LockFile())
	if err != nil {
		return err
	}
	defer unlock()
	return fn()
}

// Entry is one row of the preset list.
type Entry struct {
	Name        string     `json:"name"`
	Title       string     `json:"title,omitempty"`
	Description string     `json:"description,omitempty"`
	Agents      int        `json:"agents"`
	Active      bool       `json:"active"`
	Initial     bool       `json:"initial"`
	Accent      string     `json:"accent,omitempty"`
	CapturedAt  *time.Time `json:"capturedAt,omitempty"`
	Error       string     `json:"error,omitempty"`
}

// Entries returns the stored presets alphabetically, then Initial.
func (w Workspace) Entries() ([]Entry, error) {
	state, err := apply.LoadState(w.Paths)
	if err != nil {
		return nil, err
	}
	meta, err := initial.Load(w.Paths)
	if err != nil {
		return nil, err
	}
	active := state.ActivePreset()

	names, err := w.Store().Names()
	if err != nil {
		return nil, err
	}
	var entries []Entry
	for _, name := range names {
		if name == "initial" {
			continue
		}
		e := Entry{Name: name, Active: name == active}
		if p, err := w.Store().LoadPreset(name); err != nil {
			e.Error = firstLine(err.Error())
		} else {
			e.Title, e.Description, e.Agents, e.Accent = p.Title, p.Description, len(p.Agents), p.Accent
		}
		entries = append(entries, e)
	}
	captured := meta.CapturedAt
	return append(entries, Entry{
		Name:        "initial",
		Title:       "Initial",
		Description: "Your original Claude Code setup",
		Agents:      len(meta.Summary().AgentNames),
		Active:      active == "initial",
		Initial:     true,
		CapturedAt:  &captured,
	}), nil
}

// New stores a blank preset: no agents, mode replace, nothing set.
func (w Workspace) New(name string, tweaks ...func(*preset.Preset)) (*preset.Bundle, error) {
	if err := CheckName(name); err != nil {
		return nil, err
	}
	b := preset.Bundle{Preset: preset.Preset{
		Schema: preset.SchemaVersion,
		Name:   name,
		Title:  name,
		Mode:   preset.ModeReplace,
		Agents: []string{},
	}}
	for _, tweak := range tweaks {
		tweak(&b.Preset)
	}
	if err := w.locked(func() error { return w.Store().Save(b) }); err != nil {
		return nil, err
	}
	return &b, nil
}

// Clone copies a preset, or Initial, under a new name. Initial's agents that
// cannot be part of a preset are returned in skipped.
func (w Workspace) Clone(from, name string) (skipped []preset.Skipped, err error) {
	if err := CheckName(name); err != nil {
		return nil, err
	}
	err = w.locked(func() error {
		var b preset.Bundle
		if from == "initial" {
			b, skipped, err = w.cloneInitial(name)
			if err != nil {
				return err
			}
		} else {
			loaded, err := w.Store().Load(from)
			if err != nil {
				return err
			}
			b = *loaded
			if b.Preset.Title == "" || b.Preset.Title == from {
				b.Preset.Title = name
			}
		}
		b.Preset.Name = name
		return w.Store().Save(b)
	})
	return skipped, err
}

func (w Workspace) cloneInitial(name string) (preset.Bundle, []preset.Skipped, error) {
	meta, err := initial.Load(w.Paths)
	if err != nil {
		return preset.Bundle{}, nil, err
	}
	files, _, err := agentfile.ScanDir(filepath.Join(w.Paths.InitialDir(), "agents"))
	if err != nil {
		return preset.Bundle{}, nil, err
	}
	b, skipped, err := preset.FromFiles(name, files, settings.ManagedFromRaw(meta.Managed))
	if err != nil {
		return preset.Bundle{}, nil, err
	}
	b.Preset.Description = "Copy of your original setup"
	return b, skipped, nil
}

// Rename gives a preset a new name. When it is the equipped preset the state
// follows.
func (w Workspace) Rename(old, name string) error {
	if old == "initial" {
		return ErrProtected
	}
	if err := CheckName(name); err != nil {
		return err
	}
	return w.locked(func() error {
		b, err := w.Store().Load(old)
		if err != nil {
			return err
		}
		b.Preset.Name = name
		if b.Preset.Title == "" || b.Preset.Title == old {
			b.Preset.Title = name
		}
		if err := w.Store().Save(*b); err != nil {
			return err
		}

		state, err := apply.LoadState(w.Paths)
		if err == nil && state.Active == old {
			state.Active = name
			err = apply.SaveState(w.Paths, state)
		}
		if err != nil {
			_ = fsx.RemoveAll(w.Paths.PresetDir(name))
			return err
		}
		return fsx.RemoveAll(w.Paths.PresetDir(old))
	})
}

// Remove deletes a preset. Initial is never removed; the equipped preset only
// with force, and then its files stay in the Claude config (`squad restore`
// takes them away). It reports whether the removed preset was equipped.
func (w Workspace) Remove(name string, force bool) (wasActive bool, err error) {
	if name == "initial" {
		return false, ErrProtected
	}
	err = w.locked(func() error {
		if !w.Store().Exists(name) {
			return fmt.Errorf("%w: %s", preset.ErrNotFound, name)
		}
		state, err := apply.LoadState(w.Paths)
		if err != nil {
			return err
		}
		wasActive = state.Active == name
		if wasActive && !force {
			return fmt.Errorf("%w: %s", ErrEquipped, name)
		}
		return fsx.RemoveAll(w.Paths.PresetDir(name))
	})
	return wasActive, err
}

// Save overwrites a stored preset with the edited bundle.
func (w Workspace) Save(b preset.Bundle) error {
	return w.locked(func() error { return w.Store().Replace(b) })
}

// Lint checks a preset, or "initial".
func (w Workspace) Lint(name string) (lint.Report, error) {
	if name == "initial" {
		return w.lintInitial()
	}
	p, err := w.Store().LoadPreset(name)
	if err != nil {
		return lint.Report{Name: name, Findings: []lint.Finding{{
			Level: lint.LevelError, Rule: "invalid-preset", File: "preset.yaml", Message: firstLine(err.Error()),
			Fix: "fix preset.yaml in " + w.Paths.PresetDir(name),
		}}}, nil
	}
	in := lint.Input{Name: name, Preset: p}
	for _, agent := range p.Agents {
		if !preset.ValidName(agent) {
			continue // reported by the preset's own validation
		}
		data, err := os.ReadFile(filepath.Join(w.Paths.PresetDir(name), "agents", agent+".md"))
		if err != nil {
			continue // reported as a missing agent file
		}
		in.Agents = append(in.Agents, lint.Agent{File: agent + ".md", Data: data})
	}
	return lint.Run(in), nil
}

func (w Workspace) lintInitial() (lint.Report, error) {
	meta, err := initial.Load(w.Paths)
	if err != nil {
		return lint.Report{}, err
	}
	in := lint.Input{Name: "initial"}
	for _, entry := range meta.Agents {
		data, err := os.ReadFile(filepath.Join(w.Paths.InitialDir(), "agents", filepath.FromSlash(entry.Path)))
		if err != nil {
			return lint.Report{}, err
		}
		in.Agents = append(in.Agents, lint.Agent{File: entry.Path, Data: data})
	}
	return lint.Run(in), nil
}

// Policy loads the model allowlist from the user's and the managed settings.
func (w Workspace) Policy() (models.Policy, []string) {
	p, notes, _ := models.LoadPolicy(w.Paths.ClaudeDir, models.ManagedDirs())
	return p, notes
}

// CachedProbe returns the saved probe when it can still be trusted, judging by
// the claude version only (it is cheap to ask for; the account type is not).
func (w Workspace) CachedProbe(ctx context.Context) *models.Cache {
	cache, err := models.LoadCache(w.Paths)
	if err != nil || cache == nil {
		return nil
	}
	bin, err := claude.Find()
	if err != nil {
		return nil
	}
	version, err := claude.Client{Bin: bin}.Version(ctx)
	if err != nil || !cache.Valid(version, "", w.now()) {
		return nil
	}
	return cache
}

// ModelWarnings lists the models a preset uses that the policy or a saved
// probe says the account cannot use.
func (w Workspace) ModelWarnings(ctx context.Context, b *preset.Bundle) []string {
	policy, _ := w.Policy()
	results := models.Evaluate(models.Catalog(), policy, w.CachedProbe(ctx))

	unavailable := func(name string) (string, bool) {
		for _, r := range results {
			if r.Name == name {
				return r.Reason, r.Status == models.StatusUnavailable
			}
		}
		// A full model ID: only the policy can say anything about it.
		if v := policy.Check(name, name); v.Blocked {
			return "blocked by policy: " + v.Reason, true
		}
		return "", false
	}

	var out []string
	if m := b.Preset.Settings.Model; m != "" {
		if reason, bad := unavailable(m); bad {
			out = append(out, fmt.Sprintf("the preset's default model %q is unavailable: %s", m, reason))
		}
	}
	for _, file := range b.Agents {
		agent, err := agentfile.Parse(file.Data)
		if err != nil {
			continue
		}
		raw, ok := agent.Get("model")
		if !ok {
			continue
		}
		model := fmt.Sprint(raw)
		if reason, bad := unavailable(model); bad {
			out = append(out, fmt.Sprintf("agent %q uses the model %q, which is unavailable: %s; edit the agent or set `model: inherit`", file.Name, model, reason))
		}
	}
	return out
}

// VerifiedAndUnavailable lists the probed aliases by outcome, alphabetically.
func VerifiedAndUnavailable(results []models.Result) (verified, unavailable []string) {
	verified, unavailable = []string{}, []string{}
	for _, r := range results {
		if !slices.Contains(models.ProbeAliases, r.Name) {
			continue
		}
		switch r.Status {
		case models.StatusVerified:
			verified = append(verified, r.Name)
		case models.StatusUnavailable:
			unavailable = append(unavailable, r.Name)
		}
	}
	slices.Sort(verified)
	slices.Sort(unavailable)
	return verified, unavailable
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

// Capture stores the live Claude Code setup as a new preset. Agent files that
// cannot be part of a preset are returned in skipped.
func (w Workspace) Capture(name string) (skipped []preset.Skipped, err error) {
	if err := CheckName(name); err != nil {
		return nil, err
	}
	err = w.locked(func() error {
		var b preset.Bundle
		b, skipped, err = preset.FromConfig(w.Paths, name)
		if err != nil {
			return err
		}
		return w.Store().Save(b)
	})
	return skipped, err
}

// probeTimeout bounds each request of a probe.
const probeTimeout = 30 * time.Second

// ProbeModels sends one tiny request per model alias, stores the answers in
// the cache and returns them. It needs claude to be installed and signed in.
func (w Workspace) ProbeModels(ctx context.Context) (*models.Cache, error) {
	bin, err := claude.Find()
	if err != nil {
		return nil, ErrNoClaude
	}
	client := claude.Client{Bin: bin}
	version, err := client.Version(ctx)
	if err != nil {
		return nil, fmt.Errorf("ask claude for its version: %w", err)
	}
	auth, err := client.Auth(ctx)
	if err != nil {
		return nil, fmt.Errorf("ask claude who is signed in: %w", err)
	}
	if !auth.LoggedIn {
		return nil, ErrNotLoggedIn
	}
	cache := &models.Cache{
		ClaudeVersion: version,
		AuthMethod:    auth.AuthMethod,
		ProbedAt:      w.now().UTC(),
		Results:       models.Probe(ctx, client, models.ProbeAliases, probeTimeout),
	}
	if err := models.SaveCache(w.Paths, cache); err != nil {
		return nil, fmt.Errorf("save the probe: %w", err)
	}
	return cache, nil
}

// ImportOptions say what to do when an imported preset's name is taken.
type ImportOptions struct {
	As    string // store it under this name instead
	Force bool   // replace the preset that has the name
}

// ImportResult says where an imported preset ended up.
type ImportResult struct {
	Name     string
	Renamed  bool // stored under a name other than its own
	Replaced bool // an existing preset was overwritten
}

// Import stores a preset that came from outside. It never equips it. A name
// that is taken (or reserved) gets a numeric suffix, unless As or Force says
// otherwise.
func (w Workspace) Import(b preset.Bundle, opts ImportOptions) (ImportResult, error) {
	var res ImportResult
	err := w.locked(func() error {
		store := w.Store()
		name := b.Preset.Name
		if opts.As != "" {
			name = opts.As
		}
		checked := CheckName(name)
		if checked != nil && (opts.As != "" || !errors.Is(checked, ErrReservedName)) {
			return checked
		}
		taken := func() bool { return CheckName(name) != nil || store.Exists(name) }

		switch {
		case taken() && opts.Force && checked == nil:
			res.Replaced = true
		case taken() && opts.As != "":
			return fmt.Errorf("%w: %s", preset.ErrExists, name)
		case taken():
			base := name
			for n := 2; taken(); n++ {
				name = fmt.Sprintf("%s-%d", base, n)
			}
		}

		res.Name, res.Renamed = name, name != b.Preset.Name
		if b.Preset.Title == "" || b.Preset.Title == b.Preset.Name {
			b.Preset.Title = name
		}
		b.Preset.Name = name
		if res.Replaced {
			return store.Replace(b)
		}
		return store.Save(b)
	})
	return res, err
}

// NewFromTemplate stores a starter template as a new preset called name.
func (w Workspace) NewFromTemplate(id, name string) error {
	if err := CheckName(name); err != nil {
		return err
	}
	b, err := templates.Load(id)
	if err != nil {
		return err
	}
	b.Preset.Name = name
	return w.locked(func() error { return w.Store().Save(b) })
}

// ErrNoAgent means the agent is not in the preset (or in any preset).
var ErrNoAgent = errors.New("agent not found")

// AmbiguousAgentError means several presets have the agent and none of them
// is equipped.
type AmbiguousAgentError struct {
	Agent   string
	Presets []string
}

func (e *AmbiguousAgentError) Error() string {
	return fmt.Sprintf("agent %q is in several presets: %s", e.Agent, strings.Join(e.Presets, ", "))
}

// FindAgent returns the file of an agent and the preset it was found in. With
// a preset name it looks only there; without one it tries the equipped preset,
// then the other presets, where the agent must be in exactly one.
func (w Workspace) FindAgent(presetName, agent string) (string, []byte, error) {
	if !preset.ValidName(agent) {
		return "", nil, fmt.Errorf("%w: %q is not a valid agent name", ErrNoAgent, agent)
	}
	store := w.Store()
	read := func(name string) ([]byte, error) {
		p, err := store.LoadPreset(name)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(p.Agents, agent) {
			return nil, fmt.Errorf("%w: preset %s has no agent %s", ErrNoAgent, name, agent)
		}
		return os.ReadFile(filepath.Join(w.Paths.PresetDir(name), "agents", agent+".md"))
	}

	if presetName != "" {
		data, err := read(presetName)
		return presetName, data, err
	}
	if state, err := apply.LoadState(w.Paths); err == nil && state.Active != "" && state.Active != "initial" {
		if data, err := read(state.Active); err == nil {
			return state.Active, data, nil
		}
	}
	names, err := store.Names()
	if err != nil {
		return "", nil, err
	}
	var found []string
	for _, name := range names {
		if name == "initial" {
			continue
		}
		if p, err := store.LoadPreset(name); err == nil && slices.Contains(p.Agents, agent) {
			found = append(found, name)
		}
	}
	switch len(found) {
	case 0:
		return "", nil, fmt.Errorf("%w: no preset has an agent called %s", ErrNoAgent, agent)
	case 1:
		data, err := read(found[0])
		return found[0], data, err
	}
	return "", nil, &AmbiguousAgentError{Agent: agent, Presets: found}
}

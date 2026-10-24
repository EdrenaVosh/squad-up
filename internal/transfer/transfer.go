// Package transfer moves presets between machines: it writes the
// `.squad.json` exchange format, reads one that came from somewhere else, and
// checks it for anything that should not be applied blindly.
package transfer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/goccy/go-yaml"

	"github.com/EdrenaVosh/squadup/internal/agentfile"
	"github.com/EdrenaVosh/squadup/internal/preset"
)

// File formats and the schema this build reads and writes.
const (
	FormatPreset     = "squadup-preset"
	FormatCollection = "squadup-presets"
	SchemaVersion    = 1
)

// MaxSize is the largest document squad reads: 1 MB.
const MaxSize = 1 << 20

// ErrTooLarge means a document is over MaxSize.
var ErrTooLarge = errors.New("the file is too large (the limit is 1 MB)")

// Level says how much a finding matters.
type Level string

// Finding levels. A danger needs the author's explicit consent to import.
const (
	LevelDanger Level = "danger"
	LevelWarn   Level = "warn"
)

// Finding is one thing the importer wants the user to know before applying.
type Finding struct {
	Level   Level
	Message string
}

// Options tune Parse.
type Options struct {
	// AllowHooks keeps the hooks and mcpServers of agent files, which are
	// removed by default because they run commands.
	AllowHooks bool
}

// Imported is one preset read from a document, ready to be stored.
type Imported struct {
	Bundle   preset.Bundle
	Findings []Finding
	// HasHooks is true when an agent of the file carries hooks or mcpServers,
	// whether they were removed or kept.
	HasHooks bool
}

// Dangerous reports whether any finding is a danger.
func (i Imported) Dangerous() bool {
	return slices.ContainsFunc(i.Findings, func(f Finding) bool { return f.Level == LevelDanger })
}

type presetDoc struct {
	Name        string          `json:"name"`
	Title       string          `json:"title,omitempty"`
	Description string          `json:"description,omitempty"`
	Accent      string          `json:"accent,omitempty"`
	Mode        string          `json:"mode"`
	Main        *string         `json:"main,omitempty"`
	Flow        *preset.Flow    `json:"flow,omitempty"`
	Settings    preset.Settings `json:"settings,omitzero"`
}

type agentDoc struct {
	File    string `json:"file"`
	Content string `json:"content"`
}

type entry struct {
	Preset *presetDoc `json:"preset"`
	Agents []agentDoc `json:"agents"`
}

type document struct {
	Format     string     `json:"format"`
	Schema     int        `json:"schema"`
	Generator  string     `json:"generator,omitempty"`
	ExportedAt string     `json:"exportedAt,omitempty"`
	Preset     *presetDoc `json:"preset,omitempty"`
	Agents     []agentDoc `json:"agents,omitempty"`
	Presets    []entry    `json:"presets,omitempty"`
}

func toEntry(b preset.Bundle) (entry, error) {
	p := b.Preset
	e := entry{Preset: &presetDoc{
		Name: p.Name, Title: p.Title, Description: p.Description, Accent: p.Accent,
		Mode: p.Mode, Main: p.Main, Flow: p.Flow, Settings: p.Settings,
	}}
	byName := map[string]preset.AgentFile{}
	for _, a := range b.Agents {
		byName[a.Name] = a
	}
	for _, name := range p.Agents {
		a, ok := byName[name]
		if !ok {
			return entry{}, fmt.Errorf("agent %q is listed in preset %s but has no file", name, p.Name)
		}
		if !utf8.Valid(a.Data) {
			return entry{}, fmt.Errorf("agent file %s.md is not valid UTF-8 text and cannot be exported", name)
		}
		e.Agents = append(e.Agents, agentDoc{File: name + ".md", Content: string(a.Data)})
	}
	return e, nil
}

func encode(doc document) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return nil, fmt.Errorf("encode export: %w", err)
	}
	return buf.Bytes(), nil
}

// Export renders one preset as a `squadup-preset` document.
func Export(b preset.Bundle, generator string, now time.Time) ([]byte, error) {
	e, err := toEntry(b)
	if err != nil {
		return nil, err
	}
	return encode(document{
		Format: FormatPreset, Schema: SchemaVersion, Generator: generator,
		ExportedAt: now.UTC().Format(time.RFC3339), Preset: e.Preset, Agents: e.Agents,
	})
}

// ExportAll renders several presets as one `squadup-presets` document.
func ExportAll(bundles []preset.Bundle, generator string, now time.Time) ([]byte, error) {
	doc := document{
		Format: FormatCollection, Schema: SchemaVersion, Generator: generator,
		ExportedAt: now.UTC().Format(time.RFC3339),
	}
	for _, b := range bundles {
		e, err := toEntry(b)
		if err != nil {
			return nil, err
		}
		doc.Presets = append(doc.Presets, e)
	}
	return encode(doc)
}

// Parse reads a document and checks it. It refuses what cannot be trusted
// (broken files, unsafe names) and reports what needs a decision as findings.
// It never touches the disk.
func Parse(data []byte, opts Options) ([]Imported, error) {
	if len(data) > MaxSize {
		return nil, ErrTooLarge
	}
	var doc document
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("this is not a squadup file (it is not valid JSON: %w)", err)
	}
	var entries []entry
	switch doc.Format {
	case FormatPreset:
		if doc.Preset == nil {
			return nil, errors.New("the file has no preset in it")
		}
		entries = []entry{{Preset: doc.Preset, Agents: doc.Agents}}
	case FormatCollection:
		if len(doc.Presets) == 0 {
			return nil, errors.New("the file has no presets in it")
		}
		entries = doc.Presets
	default:
		return nil, fmt.Errorf("this is not a squadup preset file (format %q)", doc.Format)
	}
	switch {
	case doc.Schema > SchemaVersion:
		return nil, fmt.Errorf("the file uses schema %d, made by a newer squad than this one (schema %d)", doc.Schema, SchemaVersion)
	case doc.Schema < 1:
		return nil, fmt.Errorf("the file has an invalid schema %d", doc.Schema)
	}

	out := make([]Imported, 0, len(entries))
	for _, e := range entries {
		imp, err := read(e, opts)
		if err != nil {
			return nil, err
		}
		out = append(out, imp)
	}
	return out, nil
}

func read(e entry, opts Options) (Imported, error) {
	if e.Preset == nil {
		return Imported{}, errors.New("an entry of the file has no preset in it")
	}
	d := e.Preset
	p := preset.Preset{
		Schema: preset.SchemaVersion, Name: d.Name, Title: d.Title, Description: d.Description,
		Accent: d.Accent, Mode: d.Mode, Main: d.Main, Flow: d.Flow, Agents: []string{}, Settings: d.Settings,
	}

	var imp Imported
	seen := map[string]bool{}
	for _, file := range e.Agents {
		name, err := agentName(file.File)
		if err != nil {
			return Imported{}, err
		}
		if seen[name] {
			return Imported{}, fmt.Errorf("agent file %q appears twice", file.File)
		}
		seen[name] = true

		agent, err := agentfile.Parse([]byte(file.Content))
		if err != nil {
			return Imported{}, fmt.Errorf("agent file %q: %w", file.File, err)
		}
		if got := agent.Name(); got != name {
			return Imported{}, fmt.Errorf("agent file %q declares the name %q; the name inside must match the file", file.File, got)
		}
		data, findings, hooked, err := vet(name, agent, opts)
		if err != nil {
			return Imported{}, err
		}
		imp.Findings = append(imp.Findings, findings...)
		imp.HasHooks = imp.HasHooks || hooked
		p.Agents = append(p.Agents, name)
		imp.Bundle.Agents = append(imp.Bundle.Agents, preset.AgentFile{Name: name, Data: data})
	}

	if err := p.Validate(); err != nil {
		return Imported{}, fmt.Errorf("the preset %q is not valid:\n%w", d.Name, err)
	}
	if p.Main != nil && !seen[*p.Main] {
		return Imported{}, fmt.Errorf("the preset %q names %q as its main agent, which is not among its agents", p.Name, *p.Main)
	}
	imp.Bundle.Preset = p

	imp.Findings = append(imp.Findings, vetSettings(p.Settings)...)
	for _, s := range ScanSecrets(imp.Bundle) {
		imp.Findings = append(imp.Findings, Finding{LevelWarn, fmt.Sprintf(
			"agent %s, line %d: looks like a secret key (%s)", s.Agent, s.Line, s.Snippet)})
	}
	return imp, nil
}

// agentName turns the file name of an exchange entry into the agent name, and
// refuses anything that is not a plain `<name>.md`.
func agentName(file string) (string, error) {
	name, ok := strings.CutSuffix(file, ".md")
	if !ok || !preset.ValidName(name) {
		return "", fmt.Errorf("agent file %q is not allowed: it must be a plain <name>.md in lowercase letters, digits and dashes, not a path", file)
	}
	return name, nil
}

// AllowHooksHint ends the warning about removed hooks; screens that keep them
// another way replace it.
const AllowHooksHint = " (import with --allow-hooks to keep it)"

var dangerousModes = []string{"bypassPermissions", "dontAsk", "auto"}

var broadRules = []string{"Bash", "Bash(*)", "WebFetch"}

func vetSettings(s preset.Settings) []Finding {
	var out []Finding
	perms := s.Permissions
	if perms == nil {
		return nil
	}
	if slices.Contains(dangerousModes, perms.DefaultMode) {
		out = append(out, Finding{LevelDanger, fmt.Sprintf(
			"settings: defaultMode %s makes Claude skip or auto-answer permission prompts", perms.DefaultMode)})
	}
	for _, rule := range perms.Allow {
		if slices.Contains(broadRules, strings.TrimSpace(rule)) {
			out = append(out, Finding{LevelWarn, fmt.Sprintf(
				"settings: the allow rule %s lets Claude do this without asking", rule)})
		}
	}
	return out
}

// vet checks one agent and returns the bytes to store: the file as it came,
// or without hooks and mcpServers.
func vet(name string, agent *agentfile.Agent, opts Options) (data []byte, out []Finding, hooked bool, err error) {
	if v, ok := agent.Get("permissionMode"); ok {
		if mode := fmt.Sprint(v); slices.Contains(dangerousModes, mode) {
			out = append(out, Finding{LevelDanger, fmt.Sprintf(
				"agent %s: permissionMode %s makes this agent skip or auto-answer permission prompts", name, mode)})
		}
	}
	for _, key := range []string{"hooks", "mcpServers"} {
		v, ok := agent.Get(key)
		if !ok {
			continue
		}
		hooked = true
		runs := strings.Join(commandsIn(v), "; ")
		if opts.AllowHooks {
			out = append(out, Finding{LevelWarn, fmt.Sprintf("agent %s: keeps %s, which will run: %s", name, key, runs)})
			continue
		}
		agent.Delete(key)
		out = append(out, Finding{LevelWarn, fmt.Sprintf(
			"agent %s: removed %s, which would run: %s%s", name, key, runs, AllowHooksHint)})
	}
	data, err = agent.Marshal()
	if err != nil {
		return nil, nil, false, fmt.Errorf("agent file %s.md: %w", name, err)
	}
	return data, out, hooked, nil
}

// commandsIn lists the commands and URLs found in a hooks or mcpServers
// value, so the user can see what would run.
func commandsIn(v any) []string {
	var out []string
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case yaml.MapSlice:
			var command, args string
			for _, item := range x {
				switch key := fmt.Sprint(item.Key); key {
				case "command":
					command = fmt.Sprint(item.Value)
				case "args":
					if list, ok := item.Value.([]any); ok {
						parts := make([]string, len(list))
						for i, a := range list {
							parts[i] = fmt.Sprint(a)
						}
						args = strings.Join(parts, " ")
					}
				case "url":
					out = append(out, fmt.Sprint(item.Value))
				default:
					walk(item.Value)
				}
			}
			if command != "" {
				out = append(out, strings.TrimSpace(command+" "+args))
			}
		case map[string]any:
			keys := make([]string, 0, len(x))
			for k := range x {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			items := make(yaml.MapSlice, 0, len(keys))
			for _, k := range keys {
				items = append(items, yaml.MapItem{Key: k, Value: x[k]})
			}
			walk(items)
		case []any:
			for _, item := range x {
				walk(item)
			}
		}
	}
	walk(v)
	if len(out) == 0 {
		out = []string{"(nothing readable)"}
	}
	return out
}

// Secret is a string in an agent file that looks like a credential.
type Secret struct {
	Agent   string
	Line    int
	Snippet string // the line, with the credential masked
}

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{16,}`),
	regexp.MustCompile(`\bghp_[A-Za-z0-9]{20,}`),
	regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),
}

// ScanSecrets looks for API keys in the agent files of a bundle. What it
// returns is masked, so it is safe to print.
func ScanSecrets(b preset.Bundle) []Secret {
	var out []Secret
	for _, a := range b.Agents {
		for i, line := range strings.Split(string(a.Data), "\n") {
			masked, count := line, 0
			for _, re := range secretPatterns {
				count += len(re.FindAllString(masked, -1))
				masked = re.ReplaceAllStringFunc(masked, func(m string) string { return m[:4] + "…" })
			}
			for range count {
				out = append(out, Secret{Agent: a.Name, Line: i + 1, Snippet: clip(strings.TrimSpace(masked), 80)})
			}
		}
	}
	return out
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// AgentLine is one agent of a preview.
type AgentLine struct {
	Name, Role, Model, Tools string
}

// Description is what a preview shows about a preset.
type Description struct {
	Name, Title, Mode, Main string
	Settings                string // "model opus · effort high · …", or "none"
	Agents                  []AgentLine
}

// Describe summarises a bundle for the import preview.
func Describe(b preset.Bundle) Description {
	p := b.Preset
	d := Description{Name: p.Name, Title: p.Title, Mode: p.Mode}
	if p.Main != nil {
		d.Main = *p.Main
	}
	var parts []string
	if p.Settings.Model != "" {
		parts = append(parts, "model "+p.Settings.Model)
	}
	if p.Settings.EffortLevel != "" {
		parts = append(parts, "effort "+p.Settings.EffortLevel)
	}
	if s := p.Settings.Permissions.Summary(); s != "" {
		parts = append(parts, s)
	}
	d.Settings = "none"
	if len(parts) > 0 {
		d.Settings = strings.Join(parts, " · ")
	}
	for _, file := range b.Agents {
		line := AgentLine{Name: file.Name}
		if agent, err := agentfile.Parse(file.Data); err == nil {
			line.Role = agent.Role()
			if v, ok := agent.Get("model"); ok {
				line.Model = fmt.Sprint(v)
			}
			if v, ok := agent.Get("tools"); ok {
				line.Tools = listString(v)
			}
		}
		d.Agents = append(d.Agents, line)
	}
	return d
}

// listString renders a frontmatter value that is either a string or a list.
func listString(v any) string {
	list, ok := v.([]any)
	if !ok {
		return fmt.Sprint(v)
	}
	parts := make([]string, len(list))
	for i, item := range list {
		parts[i] = fmt.Sprint(item)
	}
	return strings.Join(parts, ", ")
}

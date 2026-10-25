// Package preset defines the preset model, its validation and its storage.
package preset

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"

	"github.com/goccy/go-yaml"
)

// SchemaVersion is the preset.yaml schema this build reads and writes.
const SchemaVersion = 1

// Preset modes: replace removes the agents of Initial, extend keeps them.
const (
	ModeReplace = "replace"
	ModeExtend  = "extend"
)

// Value sets accepted by Validate.
var (
	Accents      = []string{"red", "blue", "green", "yellow", "purple", "orange", "pink", "cyan"}
	EffortLevels = []string{"low", "medium", "high", "xhigh", "max"}
	// SettingsEffortLevels are the values of the `effortLevel` settings key,
	// which does not accept "max" (agents' own `effort` field does).
	SettingsEffortLevels = []string{"low", "medium", "high", "xhigh"}
	PermissionModes      = []string{"default", "acceptEdits", "auto", "dontAsk", "bypassPermissions", "plan"}
)

var nameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// ValidName reports whether s is a valid preset or agent name: a lowercase
// slug of at most 64 characters. It also keeps names safe to use as paths.
func ValidName(s string) bool { return nameRE.MatchString(s) }

// Preset is the content of preset.yaml.
type Preset struct {
	Schema      int      `yaml:"schema"`
	Name        string   `yaml:"name"`
	Title       string   `yaml:"title,omitempty"`
	Description string   `yaml:"description,omitempty"`
	Accent      string   `yaml:"accent,omitempty"`
	Mode        string   `yaml:"mode"`
	Main        *string  `yaml:"main"`
	Agents      []string `yaml:"agents,flow"`
	Flow        *Flow    `yaml:"flow,omitempty"`
	Settings    Settings `yaml:"settings,omitempty"`
}

// DefaultRounds is how many times the gate may send work back when a flow
// does not say.
const DefaultRounds = 3

// MaxRounds is the most rounds a flow may ask for.
const MaxRounds = 9

// Flow makes the agents a team: the main agent hands work down the slots in
// order. When Gate and Fix are both set, the gate agent checks the result and
// the fix agent gets its defects back until there are none or Rounds is spent.
type Flow struct {
	Gate   string `yaml:"gate,omitempty" json:"gate,omitempty"`
	Fix    string `yaml:"fix,omitempty" json:"fix,omitempty"`
	Rounds int    `yaml:"rounds,omitempty" json:"rounds,omitempty"`
}

// Loops reports whether the flow sends defects back.
func (f *Flow) Loops() bool { return f != nil && f.Gate != "" && f.Fix != "" }

// RenameInFlow follows an agent's new name in the flow.
func (p *Preset) RenameInFlow(old, name string) {
	if p.Flow == nil {
		return
	}
	if p.Flow.Gate == old {
		p.Flow.Gate = name
	}
	if p.Flow.Fix == old {
		p.Flow.Fix = name
	}
}

// PruneFlow drops the review loop when the agent it needs is gone. The order
// of the flow stays: it is the slots.
func (p *Preset) PruneFlow() {
	if p.Flow == nil || !p.Flow.Loops() {
		return
	}
	if !slices.Contains(p.Agents, p.Flow.Gate) || !slices.Contains(p.Agents, p.Flow.Fix) {
		p.Flow.Gate, p.Flow.Fix, p.Flow.Rounds = "", "", 0
	}
}

// MaxRounds is Rounds, or DefaultRounds when unset.
func (f *Flow) MaxRounds() int {
	if f == nil || f.Rounds <= 0 {
		return DefaultRounds
	}
	return f.Rounds
}

// Settings are the managed settings.json keys a preset sets. An empty field
// means "take the value from Initial".
type Settings struct {
	Model       string       `yaml:"model,omitempty" json:"model,omitempty"`
	EffortLevel string       `yaml:"effortLevel,omitempty" json:"effortLevel,omitempty"`
	Permissions *Permissions `yaml:"permissions,omitempty" json:"permissions,omitempty"`
}

// Permissions mirrors the `permissions` object of settings.json. Keys squad
// does not know are kept under `extra`. (goccy/go-yaml v1.19.2 panics on an
// inline map combined with Strict decoding, so they are not inlined.)
type Permissions struct {
	DefaultMode           string         `yaml:"defaultMode,omitempty" json:"defaultMode,omitempty"`
	Allow                 []string       `yaml:"allow,omitempty" json:"allow,omitempty"`
	Ask                   []string       `yaml:"ask,omitempty" json:"ask,omitempty"`
	Deny                  []string       `yaml:"deny,omitempty" json:"deny,omitempty"`
	AdditionalDirectories []string       `yaml:"additionalDirectories,omitempty" json:"additionalDirectories,omitempty"`
	Extra                 map[string]any `yaml:"extra,omitempty" json:"extra,omitempty"`
}

// Summary renders "acceptEdits · 4 allow · 1 ask · 2 deny", or "" for nil.
func (p *Permissions) Summary() string {
	if p == nil {
		return ""
	}
	mode := p.DefaultMode
	if mode == "" {
		mode = "default"
	}
	return fmt.Sprintf("%s · %d allow · %d ask · %d deny", mode, len(p.Allow), len(p.Ask), len(p.Deny))
}

// Marshal renders preset.yaml.
func Marshal(p Preset) ([]byte, error) {
	return yaml.Marshal(p)
}

// Unmarshal parses preset.yaml and rejects unknown fields, so typos in a
// hand-edited preset are reported instead of silently ignored.
func Unmarshal(data []byte) (*Preset, error) {
	var p Preset
	if err := yaml.UnmarshalWithOptions(data, &p, yaml.Strict()); err != nil {
		return nil, fmt.Errorf("invalid preset.yaml: %w", err)
	}
	return &p, nil
}

// Validate reports every problem found, each naming its field.
func (p Preset) Validate() error {
	var errs []error
	add := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }

	if p.Schema != SchemaVersion {
		add("schema: %d is not supported (this squad reads schema %d)", p.Schema, SchemaVersion)
	}
	if !ValidName(p.Name) {
		add("name: %q must match %s", p.Name, nameRE)
	}
	if p.Mode != ModeReplace && p.Mode != ModeExtend {
		add("mode: %q must be %q or %q", p.Mode, ModeReplace, ModeExtend)
	}
	if p.Accent != "" && !slices.Contains(Accents, p.Accent) {
		add("accent: %q must be one of %v", p.Accent, Accents)
	}
	if e := p.Settings.EffortLevel; e != "" && !slices.Contains(SettingsEffortLevels, e) {
		add("effortLevel: %q must be one of %v (\"max\" is only valid for an agent's own effort)", e, SettingsEffortLevels)
	}
	if perms := p.Settings.Permissions; perms != nil && perms.DefaultMode != "" && !slices.Contains(PermissionModes, perms.DefaultMode) {
		add("defaultMode: %q must be one of %v", perms.DefaultMode, PermissionModes)
	}
	if perms := p.Settings.Permissions; perms != nil {
		// extra is written into the real permissions object next to these keys:
		// one hidden there would escape every check made on the field itself.
		for _, key := range []string{"defaultMode", "allow", "ask", "deny", "additionalDirectories"} {
			if _, hidden := perms.Extra[key]; hidden {
				add("permissions.extra: %q is a key of its own and cannot be hidden in extra", key)
			}
		}
	}
	seen := map[string]bool{}
	for _, name := range p.Agents {
		switch {
		case !ValidName(name):
			add("agents: %q is not a valid agent name (%s)", name, nameRE)
		case seen[name]:
			add("agents: %q is listed twice", name)
		}
		seen[name] = true
	}
	if f := p.Flow; f != nil {
		if (f.Gate == "") != (f.Fix == "") {
			add("flow: gate and fix go together; set both or neither")
		}
		for field, name := range map[string]string{"gate": f.Gate, "fix": f.Fix} {
			if name != "" && !seen[name] {
				add("flow.%s: %q is not among the agents", field, name)
			}
		}
		if f.Gate != "" && f.Gate == f.Fix {
			add("flow: the gate and the fix agent must be different agents")
		}
		if f.Rounds < 0 || f.Rounds > MaxRounds {
			add("flow.rounds: %d must be between 1 and %d", f.Rounds, MaxRounds)
		}
	}
	return errors.Join(errs...)
}

// PermissionsFromJSON converts the `permissions` object of settings.json.
func PermissionsFromJSON(raw []byte) (*Permissions, error) {
	var generic map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil, fmt.Errorf("permissions is not a JSON object: %w", err)
	}
	perms := &Permissions{}
	for key, value := range generic {
		switch key {
		case "defaultMode":
			s, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf("permissions.defaultMode must be a string")
			}
			perms.DefaultMode = s
		case "allow", "ask", "deny", "additionalDirectories":
			list, err := stringList(key, value)
			if err != nil {
				return nil, err
			}
			switch key {
			case "allow":
				perms.Allow = list
			case "ask":
				perms.Ask = list
			case "deny":
				perms.Deny = list
			default:
				perms.AdditionalDirectories = list
			}
		default:
			if perms.Extra == nil {
				perms.Extra = map[string]any{}
			}
			perms.Extra[key] = value
		}
	}
	return perms, nil
}

func stringList(key string, value any) ([]string, error) {
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("permissions.%s must be a list of strings", key)
	}
	var list []string
	for _, item := range items {
		s, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("permissions.%s must be a list of strings", key)
		}
		list = append(list, s)
	}
	return list, nil
}

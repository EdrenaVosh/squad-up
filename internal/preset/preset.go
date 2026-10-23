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
	Settings    Settings `yaml:"settings,omitempty"`
}

// Settings are the managed settings.json keys a preset sets. An empty field
// means "take the value from Initial".
type Settings struct {
	Model       string       `yaml:"model,omitempty"`
	EffortLevel string       `yaml:"effortLevel,omitempty"`
	Permissions *Permissions `yaml:"permissions,omitempty"`
}

// Permissions mirrors the `permissions` object of settings.json. Keys squad
// does not know are kept under `extra`. (goccy/go-yaml v1.19.2 panics on an
// inline map combined with Strict decoding, so they are not inlined.)
type Permissions struct {
	DefaultMode           string         `yaml:"defaultMode,omitempty"`
	Allow                 []string       `yaml:"allow,omitempty"`
	Ask                   []string       `yaml:"ask,omitempty"`
	Deny                  []string       `yaml:"deny,omitempty"`
	AdditionalDirectories []string       `yaml:"additionalDirectories,omitempty"`
	Extra                 map[string]any `yaml:"extra,omitempty"`
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

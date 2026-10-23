// Package agentfile reads and writes Claude Code subagent files: a Markdown
// body with a YAML frontmatter. An untouched file always serialises back to
// the exact bytes it was read from.
package agentfile

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/goccy/go-yaml"
)

var (
	// ErrNoFrontmatter means the file does not start with a `---` line.
	ErrNoFrontmatter = errors.New("file has no YAML frontmatter")
	// ErrUnterminated means the opening `---` has no closing `---` line.
	ErrUnterminated = errors.New("frontmatter is not closed with ---")
	// ErrNotMapping means the frontmatter is valid YAML but not key: value pairs.
	ErrNotMapping = errors.New("frontmatter is not a key: value mapping")
)

// KnownKeys lists the frontmatter fields Claude Code understands, in the order
// Canonical writes them.
var KnownKeys = []string{
	"name", "description", "tools", "disallowedTools", "model", "permissionMode",
	"maxTurns", "skills", "mcpServers", "hooks", "memory", "background", "effort",
	"isolation", "color", "initialPrompt", "omitClaudeMd", "autoCompactWindow", "experimental",
}

// Agent is a parsed agent file.
type Agent struct {
	fields yaml.MapSlice
	body   []byte
	raw    []byte
	dirty  bool
}

// Parse splits data into frontmatter fields and body.
func Parse(data []byte) (*Agent, error) {
	frontmatter, body, err := split(data)
	if err != nil {
		return nil, err
	}

	var decoded any
	if err := yaml.UnmarshalWithOptions(frontmatter, &decoded, yaml.UseOrderedMap()); err != nil {
		return nil, fmt.Errorf("invalid frontmatter YAML: %w", err)
	}
	var fields yaml.MapSlice
	switch v := decoded.(type) {
	case nil:
	case yaml.MapSlice:
		fields = v
	default:
		return nil, ErrNotMapping
	}

	return &Agent{
		fields: fields,
		body:   append([]byte(nil), body...),
		raw:    append([]byte(nil), data...),
	}, nil
}

// split returns the bytes between the two `---` lines and everything after
// the closing line.
func split(data []byte) (frontmatter, body []byte, err error) {
	first, rest, _ := cutLine(data)
	if !isDelimiter(first) {
		return nil, nil, ErrNoFrontmatter
	}
	start := len(data) - len(rest)
	for pos := start; pos < len(data); {
		line, next, _ := cutLine(data[pos:])
		if isDelimiter(line) {
			return data[start:pos], next, nil
		}
		pos = len(data) - len(next)
	}
	return nil, nil, ErrUnterminated
}

// cutLine returns the first line without its newline, the remainder after it,
// and whether a newline was found.
func cutLine(data []byte) (line, rest []byte, found bool) {
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		return data[:i], data[i+1:], true
	}
	return data, nil, false
}

func isDelimiter(line []byte) bool {
	return string(bytes.TrimSuffix(line, []byte("\r"))) == "---"
}

// Name is the `name` field, or "" when absent.
func (a *Agent) Name() string { return a.str("name") }

// Description is the `description` field, or "" when absent.
func (a *Agent) Description() string { return a.str("description") }

// Prompt is the Markdown body, the agent's system prompt.
func (a *Agent) Prompt() string { return string(a.body) }

// Get returns the raw value of a frontmatter field.
func (a *Agent) Get(key string) (any, bool) {
	for _, item := range a.fields {
		if fmt.Sprint(item.Key) == key {
			return item.Value, true
		}
	}
	return nil, false
}

// Keys returns the frontmatter keys in file order.
func (a *Agent) Keys() []string {
	keys := make([]string, 0, len(a.fields))
	for _, item := range a.fields {
		keys = append(keys, fmt.Sprint(item.Key))
	}
	return keys
}

// Set stores a frontmatter value. After a Set, Marshal emits the canonical
// form instead of the original bytes.
func (a *Agent) Set(key string, value any) {
	a.dirty = true
	for i, item := range a.fields {
		if fmt.Sprint(item.Key) == key {
			a.fields[i].Value = value
			return
		}
	}
	a.fields = append(a.fields, yaml.MapItem{Key: key, Value: value})
}

// SetBody replaces the prompt. After it, Marshal emits the canonical form.
func (a *Agent) SetBody(body string) {
	a.dirty = true
	a.body = []byte(body)
}

func (a *Agent) str(key string) string {
	v, ok := a.Get(key)
	if !ok || v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

// Marshal returns the original bytes of an untouched file and the canonical
// form of a modified one.
func (a *Agent) Marshal() ([]byte, error) {
	if !a.dirty {
		return append([]byte(nil), a.raw...), nil
	}
	return a.Canonical()
}

// Canonical renders the agent with known keys in documented order, then
// unknown keys in file order, skipping empty values. The body is copied as is.
func (a *Agent) Canonical() ([]byte, error) {
	var ordered yaml.MapSlice
	used := map[int]bool{}
	for _, key := range KnownKeys {
		for i, item := range a.fields {
			if fmt.Sprint(item.Key) == key && !used[i] {
				used[i] = true
				if !isEmpty(item.Value) {
					ordered = append(ordered, item)
				}
			}
		}
	}
	for i, item := range a.fields {
		if !used[i] && !isEmpty(item.Value) {
			ordered = append(ordered, item)
		}
	}

	var out bytes.Buffer
	out.WriteString("---\n")
	if len(ordered) > 0 {
		encoded, err := yaml.Marshal(ordered)
		if err != nil {
			return nil, fmt.Errorf("encode frontmatter: %w", err)
		}
		out.Write(encoded)
	}
	out.WriteString("---\n")
	out.Write(a.body)
	return out.Bytes(), nil
}

func isEmpty(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case string:
		return x == ""
	case []any:
		return len(x) == 0
	case yaml.MapSlice:
		return len(x) == 0
	}
	return false
}

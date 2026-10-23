// Package models knows which model names Claude Code accepts and which of them
// an account can really use. Claude Code has no command that lists the
// available models, so the answer is assembled from a built-in catalogue, the
// allowlist policy in settings, and an optional probe.
package models

import "regexp"

// Entry is a model name that can be written somewhere in a preset.
type Entry struct {
	Name     string
	Agents   bool // valid in an agent's `model` field
	Settings bool // valid in the `model` settings key
	Note     string
}

// Catalog lists the model names Claude Code documents, agents' first.
func Catalog() []Entry {
	return []Entry{
		{Name: "inherit", Agents: true, Note: "uses the model of the session"},
		{Name: "fable", Agents: true, Settings: true},
		{Name: "opus", Agents: true, Settings: true},
		{Name: "sonnet", Agents: true, Settings: true},
		{Name: "haiku", Agents: true, Settings: true},
		{Name: "default", Settings: true, Note: "your account's default model"},
		{Name: "best", Settings: true, Note: "fable where available, otherwise opus"},
		{Name: "opus[1m]", Settings: true, Note: "opus with a 1M-token context window"},
		{Name: "sonnet[1m]", Settings: true, Note: "sonnet with a 1M-token context window"},
		{Name: "opusplan", Settings: true, Note: "opus in plan mode, sonnet for execution"},
	}
}

// ProbeAliases are the aliases a probe can test; the other names are derived.
var ProbeAliases = []string{"fable", "opus", "sonnet", "haiku"}

var fullID = regexp.MustCompile(`^claude-[a-z0-9][a-z0-9.\-]*(\[1m\])?$`)

// ValidAgentModel reports whether s is acceptable in an agent's `model` field:
// inherit, an alias, or a full model ID.
func ValidAgentModel(s string) bool {
	switch s {
	case "inherit", "fable", "opus", "sonnet", "haiku":
		return true
	}
	return fullID.MatchString(s)
}

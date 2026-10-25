// Package models knows which model names Claude Code accepts and which of them
// an account can really use. Claude Code has no command that lists the
// available models, so the answer is assembled from a built-in catalogue, the
// allowlist policy in settings, and an optional probe.
package models

import (
	"regexp"
	"strings"
)

// Entry is a model name that can be written somewhere in a preset.
type Entry struct {
	Name     string
	Agents   bool // valid in an agent's `model` field
	Settings bool // valid in the `model` settings key
	Note     string
	// Version is true for a model pinned to one release by its full ID, such as
	// claude-sonnet-5-5; an alias like sonnet follows the newest one.
	Version bool
	Family  string // the alias of its family: fable, opus, sonnet or haiku
	Title   string // how a person says it: "Sonnet 5.5"
}

// versions are the full model IDs the Claude Code documentation names, newest
// first within each family. Claude Code takes no version in an alias (sonnet-5
// is not a name), so a release is picked by its full ID. Which of them an
// account can use depends on its plan and provider, and only a probe knows.
var versions = []Entry{
	{Name: "claude-fable-5-1", Family: "fable", Title: "Fable 5.1"},
	{Name: "claude-fable-5", Family: "fable", Title: "Fable 5"},
	{Name: "claude-opus-5-5", Family: "opus", Title: "Opus 5.5"},
	{Name: "claude-opus-5", Family: "opus", Title: "Opus 5"},
	{Name: "claude-opus-4-6", Family: "opus", Title: "Opus 4.6"},
	{Name: "claude-sonnet-5-5", Family: "sonnet", Title: "Sonnet 5.5"},
	{Name: "claude-sonnet-5", Family: "sonnet", Title: "Sonnet 5"},
	{Name: "claude-sonnet-4-6", Family: "sonnet", Title: "Sonnet 4.6"},
	{Name: "claude-sonnet-4-5", Family: "sonnet", Title: "Sonnet 4.5"},
	{Name: "claude-haiku-5-5", Family: "haiku", Title: "Haiku 5.5"},
	{Name: "claude-haiku-4-5", Family: "haiku", Title: "Haiku 4.5"},
}

// Versions lists the model releases that can be picked by their full ID.
func Versions() []Entry {
	out := make([]Entry, len(versions))
	for i, v := range versions {
		v.Agents, v.Settings, v.Version = true, true, true
		out[i] = v
	}
	return out
}

// FullCatalog is the catalogue with the releases after the aliases.
func FullCatalog() []Entry { return append(Catalog(), Versions()...) }

// Title is the title of a known release ("Sonnet 5.5"), or the name itself.
func Title(name string) string {
	for _, v := range versions {
		if v.Name == name {
			return v.Title
		}
	}
	return name
}

// Pretty is how a person says a model: "sonnet 5.5" for a known full ID, the
// name itself for anything else.
func Pretty(name string) string {
	for _, v := range versions {
		if v.Name == name {
			return strings.ToLower(v.Title)
		}
	}
	return name
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

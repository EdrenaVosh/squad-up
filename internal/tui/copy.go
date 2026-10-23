// Package tui holds the interface texts. All user-facing strings live here so
// that wording stays consistent between the screens and the plain commands.
package tui

import "fmt"

// First-run and Initial texts.
const (
	WelcomeTitle   = "Welcome to squadup"
	FoundExisting  = "We found your existing Claude Code setup"
	SavedAsInitial = "and saved it as ◇ Initial — your safe point."
	CleanSetup     = "Clean Claude Code setup — Initial is the defaults"
	NothingChanged = "Nothing was changed. Run `squad restore` any time to get this back."
	SettingsBroken = "settings.json is not valid JSON — it was saved as it is"
	Unset          = "—"
)

// Labels of the setup summary.
const (
	LabelAgents      = "Agents"
	LabelModel       = "Model"
	LabelEffort      = "Effort"
	LabelPermissions = "Permissions"
	LabelMainAgent   = "Main agent"
)

// InitialLabel describes Initial in lists.
func InitialLabel(date string) string {
	return fmt.Sprintf("your original setup · captured %s", date)
}

// InitialHeader is the first line of `squad initial`.
func InitialHeader(when string) string {
	return fmt.Sprintf("◇ Initial — your original setup, captured %s", when)
}

// AgentCount renders "1 agent" / "3 agents".
func AgentCount(n int) string {
	if n == 1 {
		return "1 agent"
	}
	return fmt.Sprintf("%d agents", n)
}

// AgentsHaveIssues renders the lint pointer shown under a setup summary.
func AgentsHaveIssues(n int) string {
	if n == 1 {
		return "1 agent has issues — run squad lint initial"
	}
	return fmt.Sprintf("%d agents have issues — run squad lint initial", n)
}

// List texts.
const (
	NoPresetsYet = "No presets yet. Run `squad capture <name>` to save your current setup as one."
	ListDivider  = "──────────────────────────────────────────"
)

// Status texts.
const (
	ClaudeNotFound    = "not found — run squad doctor"
	ClaudeUnavailable = "unavailable — run squad doctor"
	ManualEdits       = "Manual edits detected"
	NoDrift           = "none"
)

// ModelOverride warns that ANTHROPIC_MODEL beats the preset's model.
func ModelOverride(value string) string {
	return fmt.Sprintf("ANTHROPIC_MODEL is set (%s): it overrides the model of every preset", value)
}

// Recapture texts.
const (
	RecapturePrompt = "Replace Initial with your current setup? The old snapshot is kept in backups. [y/N] "
	Cancelled       = "Cancelled."
)

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
	NotProbed         = "not probed — run squad models --probe"
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

// Equip, restore and undo texts.
const (
	ApplyPrompt = "Apply these changes? [y/N] "
	DriftPrompt = "You edited files of the equipped preset by hand. What now? [s]ave the edits into it, [d]iscard them, [c]ancel: "
	DryRun      = "Dry run: nothing was changed."
	NoLogYet    = "No operations yet."

	ManualEditsHeader = "Manual edits detected in the equipped preset:"
	ConflictsHeader   = "Cannot continue, these files are in the way:"
	NoFileChanges     = "No file changes: only the equipped preset changes."
)

// Equipped is shown after `squad use`.
func Equipped(preset, backup string) string {
	return fmt.Sprintf("Equipped %q. Backup: %s.\nGo back with `squad undo`, or return to your original setup with `squad restore`.", preset, backup)
}

// Restored is shown after `squad restore`.
func Restored(backup string) string {
	return fmt.Sprintf("Restored your original setup (Initial). Backup: %s.\nGo back with `squad undo`.", backup)
}

// Undone is shown after `squad undo`.
func Undone(target, backup string) string {
	return fmt.Sprintf("Undone: back to %q. Backup of the state before the undo: %s (`squad undo` again redoes it).", target, backup)
}

// AlreadyEquipped explains a no-op.
func AlreadyEquipped(target string) string {
	if target == "initial" {
		return "Already on Initial — nothing to change."
	}
	return fmt.Sprintf("%q is already equipped — nothing to change.", target)
}

// Import, export and template texts.
const (
	ImportPromptTitle   = "Import preset"
	ImportPromptHelp    = "A .squad.json file, an https link or gist:<id>."
	ImportTitle         = "Import preview"
	ImportOnlyStored    = "It is only stored; nothing is equipped."
	ImportAnywayTitle   = "Permissions without prompts"
	ImportAnywayWarning = "The preset switches permission prompts off or hands them to a classifier."
	ExportPromptTitle   = "Export preset"
	ExportPromptHelp    = "File to write. Nothing is uploaded."
	ExportSecretsTitle  = "These look like secret keys"
	ExportInitial       = "Initial cannot be exported: clone it first (c), then export the copy"
	TemplatesTitle      = "Starter templates"
	TemplateNameTitle   = "Name the new preset"
	TemplateNameHelp    = "Lowercase letters, digits and dashes."
	StarterTemplate     = "Use a template"
)

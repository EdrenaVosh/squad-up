package tui_test

import (
	"strings"
	"testing"

	"github.com/EdrenaVosh/squadup/internal/preset"
	"github.com/EdrenaVosh/squadup/internal/testutil"
)

func TestTheIndependentSwitch(t *testing.T) {
	h := newHarness(t)
	h.resize(140, 40)
	h.openAgent("solo-indie", 1) // reviewer

	h.field(fIndependent)
	h.contains("Independent", "◇ normal", "Normal", "works from its task and instructions alone")

	h.press("enter")
	h.contains("◈ independent", "What it changes", "short charter", "main agent is told", "All of it is instructions")
	h.lacks("omitClaudeMd", "Locked by Claude Code")
	if got := string(h.stored("solo-indie").Agents[1].Data); !strings.Contains(got, "squadIndependent: true") {
		t.Errorf("independence was not stored:\n%s", got)
	}

	h.press("space")
	h.contains("◇ normal")
	if got := string(h.stored("solo-indie").Agents[1].Data); strings.Contains(got, "squadIndependent") {
		t.Errorf("the default must not be written:\n%s", got)
	}
}

func TestTilesAndTheTableSayWhoIsIndependent(t *testing.T) {
	h := newHarness(t)
	h.resize(160, 44)
	if err := h.ws.SetIndependent("bug-hunt", "tester", true); err != nil {
		t.Fatal(err)
	}

	h.goTo("solo-indie")
	h.goTo("bug-hunt")
	h.contains("stance  normal", "stance  ◈ independent")

	h.goTo("refactor-day")
	h.lacks("stance")
}

func TestAgentsAndTheSquadLookDifferent(t *testing.T) {
	h := newHarness(t)
	h.resize(160, 44)
	h.goTo("bug-hunt")

	h.contains("◆ AGENTS", "▪ SQUAD")
	h.contains("╭─ 2 ", "┌─ SETTINGS") // agents are rounded tiles, the squad's panels are square
	h.lacks("╭─ SETTINGS")
}

func TestTheFlowShowsAsALeadAnOrderAndALoop(t *testing.T) {
	h := newHarness(t)
	h.resize(160, 44)
	if err := h.ws.SetFlow("bug-hunt", &preset.Flow{Gate: "tester", Fix: "debugger", Rounds: 2}); err != nil {
		t.Fatal(err)
	}

	h.goTo("solo-indie")
	h.goTo("bug-hunt")

	h.contains("◆ debugger", "tester", "↺ tester finds defects · up to 2", "▲") // the diagram of the flow
}

func TestTeamFlowIsChosenFromTheList(t *testing.T) {
	h := newHarness(t)
	h.open("solo-indie")
	h.press("down", "down", "down", "down", "down", "down", "down", "down") // Team flow
	h.contains("Team flow", "off")

	h.press("enter", "down", "down", "enter") // Review loop
	h.contains("Who checks the work?")
	h.press("down", "down", "enter") // tester
	h.contains("Who fixes the defects?")
	h.lacks("tester")
	h.press("enter") // planner
	h.contains("How many rounds at most?")
	h.press("up", "enter") // 2

	f := h.stored("solo-indie").Preset.Flow
	if f == nil || f.Gate != "tester" || f.Fix != "planner" || f.Rounds != 2 {
		t.Fatalf("flow = %+v", f)
	}
	h.contains("review loop", "tester", "checks,", "planner", "fixes · up to 2")

	h.press("enter", "up", "up", "enter") // Off
	if f := h.stored("solo-indie").Preset.Flow; f != nil {
		t.Errorf("flow = %+v, want none", f)
	}
}

func TestOrderOnlyFlowWarnsWhenNoMainAgentCarriesIt(t *testing.T) {
	h := newHarness(t)
	h.open("solo-indie")
	h.press("down", "down", "down", "down", "down", "down", "down", "down")

	h.press("enter", "down", "enter")

	if f := h.stored("solo-indie").Preset.Flow; f == nil || f.Loops() {
		t.Fatalf("flow = %+v", f)
	}
	h.contains("in order", "No main agent")
}

func TestTheAgentsTableHasAHeaderAndNoRoleColumn(t *testing.T) {
	h := newHarness(t)
	h.open("solo-indie")
	h.contains("SLOT", "AGENT", "MODEL", "EFFORT", "ACCESS", "STANCE")
	h.fits(80, 24)

	h.resize(140, 40)
	h.lacks("ROLE")
	h.contains("ABOUT  planner", "Breaks a task into steps. Use before starting any large change.")
	h.press("down", "down", "down", "down", "down", "down", "down", "down") // Team flow
	h.contains("ABOUT  Team flow", "How the agents work as a team")
}

func TestTheNotesOfAChoiceWrapInsteadOfBeingCutOff(t *testing.T) {
	h := newHarness(t)
	h.open("solo-indie")
	h.press("down", "down", "down", "down", "down", "down", "down", "down", "enter") // Team flow

	h.contains("Off", "In order", "Review loop", "separate helpers: Claude calls them as it sees fit", "the last check sends defects back to the one who fixes", "them")
	h.lacks("…")
}

func TestTheEquippedPresetShowsItsMascotsAtWork(t *testing.T) {
	h := equipped(t, "solo-indie")
	h.resize(160, 44)

	h.contains("▐▛███▜▌", "▝▜█████▛▘") // a big tile of a live preset has the mascot in its corner
	h.goTo("bug-hunt")
	h.lacks("▐▛███▜▌") // a preset that is not equipped has none
}

func TestTheMascotsComeToTheEquipCelebration(t *testing.T) {
	h := newHarness(t)
	h.open("bug-hunt")
	h.press("e", "enter")

	h.contains("Squad ready", "▐▛███▜▌")
}

func TestTheChecksOfAnAgentWrapInsteadOfBeingCutOff(t *testing.T) {
	h := newHarness(t)
	h.resize(80, 24)
	h.openAgent("solo-indie", 1)

	h.field(fPermission)
	h.press("enter", "down", "down", "down", "down", "enter") // bypassPermissions

	h.contains("CHECKS", "permissionMode bypassPermissions", "without asking", "→ use acceptEdits or default")
	h.lacks("without ask…")
}

func TestThePanelsOfTheSquadWrapTheirText(t *testing.T) {
	h := newHarness(t)
	h.resize(160, 44)

	h.contains("TOKENS · LIVE", "no session in this folder yet", "start `squad run` here and every", "agent's tokens show up as it works")
	h.lacks("ever…", "and…")
}

func TestAnAgentThatStillHasTheOldShareFieldIsNotWarnedAboutAndLosesItOnSave(t *testing.T) {
	h := newHarness(t)
	testutil.WriteFiles(t, h.paths.PresetDir("solo-indie"), map[string]string{
		"agents/tester.md": agentFile("tester", "Writes and runs tests. Use when a change needs coverage.", "tools: Read, Bash\nsquadShare: false\n", "Test it."),
	})

	h.open("solo-indie")
	h.lacks("unknown field", "squadShare")

	h.press("down", "down", "enter") // the tester
	h.field(fIndependent)
	h.press("enter") // any change saves the agent
	if got := string(h.stored("solo-indie").Agents[2].Data); strings.Contains(got, "squadShare") {
		t.Errorf("the old field stayed:\n%s", got)
	}
}

// The keys of docs/demo.tape, which cannot be recorded in a test: a new preset
// from the first template, equipped, and shown with its agents at work.
func TestTheKeysOfTheDemoDoWhatTheScriptExpects(t *testing.T) {
	h := newHarness(t)
	h.resize(156, 48)

	h.press("n")
	h.typeText("Cube Bug Hunter!")
	h.press("enter") // the name
	h.contains("Use a template")
	h.press("enter") // the first option
	h.contains("Bug Hunter", "Feature Creator")
	h.press("enter") // Bug Hunter, the first
	h.contains("Cube Bug Hunter!", "AGENTS", "debugger", "fixer", "reviewer", "tester")

	h.press("down", "down", "down", "down", "esc")
	h.contains("PRESETS", "Cube Bug Hunter!")
	h.goTo("cube-bug-hunter") // in the sandbox of the demo it is the only preset, so it is already selected
	h.press("right", "right", "right")
	h.contains("╔═ ▶ 4 ") // the fourth agent is selected

	h.press("e")
	h.contains("Equip cube-bug-hunter")
	h.press("enter")
	h.contains("Squad ready", "▐▛███▜▌")
	h.press("esc")
	h.contains("EQUIPPED")
}

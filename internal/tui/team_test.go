package tui_test

import (
	"strings"
	"testing"

	"github.com/EdrenaVosh/squadup/internal/preset"
)

func TestTheSharedContextSwitch(t *testing.T) {
	h := newHarness(t)
	h.resize(140, 36)
	h.openAgent("solo-indie", 0)

	h.field(fShare)
	h.contains("Shared context", "⇄ shared", "reads .squad/notes.md first", "sees only the task it is given", "What the notes look like")
	h.contains("read but not write") // the planner's tools cannot write the file

	h.press("enter")
	h.contains("⊘ isolated", "neither reads nor writes the notes")
	if got := string(h.stored("solo-indie").Agents[0].Data); !strings.Contains(got, "squadShare: false") {
		t.Errorf("the isolation was not stored:\n%s", got)
	}

	h.press("space")
	h.contains("⇄ shared")
	if got := string(h.stored("solo-indie").Agents[0].Data); strings.Contains(got, "squadShare") {
		t.Errorf("the default must not be written:\n%s", got)
	}
}

func TestAnAgentAloneHasNobodyToShareWith(t *testing.T) {
	h := newHarness(t)
	h.resize(140, 36)
	h.openAgent("refactor-day", 0)

	h.field(fShare)

	h.contains("alone", "no one to share with")
}

func TestTilesSayWhoSharesTheNotes(t *testing.T) {
	h := newHarness(t)
	h.resize(160, 44)

	h.goTo("bug-hunt")
	h.contains("context ⇄ shared")

	h.goTo("refactor-day")
	h.lacks("context ")
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

func TestTheAgentsTableHasAHeaderAndRoomForTheRole(t *testing.T) {
	h := newHarness(t)
	h.open("solo-indie")
	h.contains("SLOT", "AGENT", "MODEL", "EFFORT", "ACCESS", "CONTEXT")
	h.lacks("ROLE") // no room at 80 columns

	h.resize(140, 40)
	h.contains("ROLE", "Breaks a task into steps.", "ABOUT  planner", "Breaks a task into steps. Use before starting any large change.")
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

func TestALongErrorInADialogIsWrappedNotCutOff(t *testing.T) {
	h := newHarness(t)

	h.press("n")
	h.typeText("Cube creator")
	h.press("enter")

	h.contains("preset names use lowercase letters", "and have up to 64 characters")
	h.lacks("…")
}

func TestThePanelsOfTheSquadWrapTheirText(t *testing.T) {
	h := newHarness(t)
	h.resize(160, 44)

	h.contains("TOKENS · LIVE", "no session in this folder yet", "every agent says", "when to call it")
	h.lacks("ever…", "and…")
}

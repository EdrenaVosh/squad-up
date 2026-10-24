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

	h.contains("◆ debugger", "tester", "↺", "tester finds defects → debugger fixes · up to 2")
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

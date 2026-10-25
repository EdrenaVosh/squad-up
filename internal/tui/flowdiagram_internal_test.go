package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/EdrenaVosh/squadup/internal/preset"
	"github.com/EdrenaVosh/squadup/internal/workspace"
)

func flowCard(flow *preset.Flow, main string) workspace.Card {
	agents := []workspace.AgentInfo{
		{Name: "planner", Color: "cyan", Shared: true},
		{Name: "dev", Color: "green", Shared: true},
		{Name: "reviewer", Color: "blue", Shared: false},
	}
	return workspace.Card{Agents: agents, Flow: flow, Main: main, Sharing: true}
}

func plainRows(rows []string) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = ansi.Strip(r)
	}
	return out
}

func TestTheFlowDiagramShowsTheOrderAndTheLoop(t *testing.T) {
	c := flowCard(&preset.Flow{Gate: "reviewer", Fix: "dev", Rounds: 2}, "planner")

	rows := plainRows(flowDiagram(c, 100, 0, false))

	if len(rows) != 3 {
		t.Fatalf("rows = %q", rows)
	}
	if !strings.Contains(rows[0], "◆ planner ──▶ ■ dev ──▶ ■ reviewer⊘") {
		t.Errorf("order row = %q", rows[0])
	}
	at := func(s, sub string) int { return len([]rune(s[:strings.Index(s, sub)])) } // in cells, not bytes
	fix, gate := at(rows[1], "▲"), at(rows[1], "│")
	if fix < 0 || gate < 0 || fix >= gate {
		t.Errorf("the arc must go up into the fixer and down from the gate: %q", rows[1])
	}
	if at(rows[2], "╰") != fix || at(rows[2], "╯") != gate || !strings.Contains(rows[2], "↺ reviewer finds defects · up to 2") { // too long to fit between, so it follows the arc
		t.Errorf("arc row = %q", rows[2])
	}
	if at(rows[0], "dev")+1 < fix-2 || fix > at(rows[0], "reviewer") {
		t.Errorf("the arrow head is not under the fixer: %q / %q", rows[0], rows[1])
	}
}

func TestTheLoopLabelShrinksToFitInsideTheArc(t *testing.T) {
	c := flowCard(&preset.Flow{Gate: "reviewer", Fix: "planner"}, "")

	rows := plainRows(flowDiagram(c, 44, 0, false))

	if len(rows) != 3 || !strings.Contains(rows[2], "↺ ×3") || strings.Contains(rows[2], "finds defects") {
		t.Errorf("arc row = %q", rows)
	}
}

func TestTheLeadCanBeWhoFixes(t *testing.T) {
	c := flowCard(&preset.Flow{Gate: "reviewer", Fix: "planner"}, "planner")

	rows := plainRows(flowDiagram(c, 100, 0, false))

	if len(rows) != 3 || !strings.Contains(rows[1], "▲") {
		t.Errorf("rows = %q", rows)
	}
}

func TestTheFlowDiagramWithoutALoopIsOneRowAndNeedsAFlow(t *testing.T) {
	if rows := flowDiagram(flowCard(&preset.Flow{}, ""), 100, 0, false); len(rows) != 1 || !strings.Contains(ansi.Strip(rows[0]), "you ──▶ ■ planner") {
		t.Errorf("rows = %q", rows)
	}
	if flowDiagram(flowCard(nil, ""), 100, 0, false) != nil {
		t.Error("a card without a flow got a diagram")
	}
	c := flowCard(&preset.Flow{}, "")
	c.Initial = true
	if flowDiagram(c, 100, 0, false) != nil {
		t.Error("Initial got a diagram")
	}
}

func TestTheFlowDiagramGivesWayWhenItDoesNotFit(t *testing.T) {
	if rows := flowDiagram(flowCard(&preset.Flow{Gate: "reviewer", Fix: "dev"}, "planner"), 30, 0, false); rows != nil {
		t.Errorf("rows = %q", rows)
	}
}

func TestALitArrowMovesAlongTheFlow(t *testing.T) {
	c := flowCard(&preset.Flow{Gate: "reviewer", Fix: "dev"}, "planner")
	seen := map[string]bool{}
	for f := 1; f < 60; f++ {
		seen[strings.Join(flowDiagram(c, 100, f, true), "\n")] = true
	}
	if len(seen) < 3 {
		t.Errorf("only %d different pictures", len(seen))
	}
	if strings.Join(flowDiagram(c, 100, 0, false), "") != strings.Join(flowDiagram(c, 100, 0, true), "") {
		t.Error("frame 0 must be the still picture")
	}
}

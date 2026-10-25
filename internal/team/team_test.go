package team_test

import (
	"strings"
	"testing"

	"github.com/EdrenaVosh/squadup/internal/agentfile"
	"github.com/EdrenaVosh/squadup/internal/preset"
	"github.com/EdrenaVosh/squadup/internal/team"
)

func file(name, extra string) preset.AgentFile {
	return preset.AgentFile{Name: name, Data: []byte("---\nname: " + name + "\ndescription: Does " + name + " work\n" + extra + "---\nYou are " + name + ".\n")}
}

func bundle(flow *preset.Flow, main string, files ...preset.AgentFile) preset.Bundle {
	p := preset.Preset{Schema: 1, Name: "t", Mode: "replace", Flow: flow}
	for _, f := range files {
		p.Agents = append(p.Agents, f.Name)
	}
	if main != "" {
		p.Main = &main
	}
	return preset.Bundle{Preset: p, Agents: files}
}

func compile(t *testing.T, b preset.Bundle) map[string]string {
	t.Helper()
	out, err := team.Compile(b)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, f := range out {
		got[f.Name] = string(f.Data)
	}
	return got
}

func TestAnAgentIsNotIndependentUnlessItSays(t *testing.T) {
	b := bundle(nil, "", file("dev", ""), file("reviewer", ""))

	got := compile(t, b)

	for name, data := range got {
		if data != string(b.Agents[map[string]int{"dev": 0, "reviewer": 1}[name]].Data) {
			t.Errorf("%s changed although nothing was asked:\n%s", name, data)
		}
	}
}

func TestAnIndependentAgentGetsACharterAndANoteForItsCaller(t *testing.T) {
	got := compile(t, bundle(nil, "", file("dev", ""), file("reviewer", "squadIndependent: true\n")))

	r := got["reviewer"]
	if strings.Contains(r, team.IndependentKey) {
		t.Errorf("the squad-only field reached Claude Code:\n%s", r)
	}
	for _, want := range []string{"<!-- squad:independent -->", "You are an independent agent", "only a task and instructions", "Independent: give it only the task and instructions", "You are reviewer."} {
		if !strings.Contains(r, want) {
			t.Errorf("reviewer lacks %q:\n%s", want, r)
		}
	}
	if strings.Contains(r, "omitClaudeMd") {
		t.Errorf("an independent agent must still see CLAUDE.md:\n%s", r)
	}
	if strings.Contains(got["dev"], "squad:independent") {
		t.Errorf("a normal agent was touched:\n%s", got["dev"])
	}
}

func TestTheDescriptionTellsTheCallerHowToCallIt(t *testing.T) {
	a := mustParse(t, compile(t, bundle(nil, "", file("reviewer", "squadIndependent: true\n")))["reviewer"])

	if d := a.Description(); !strings.HasPrefix(d, "Does reviewer work") || !strings.Contains(d, "never your own conclusions or what other agents reasoned") {
		t.Errorf("description = %q", d)
	}
}

func TestCompilingIsRepeatableAndNeverDoublesABlock(t *testing.T) {
	b := bundle(&preset.Flow{Gate: "reviewer", Fix: "dev"}, "lead", file("lead", ""), file("dev", ""), file("reviewer", "squadIndependent: true\n"))
	first, second := compile(t, b), compile(t, b)
	for name := range first {
		if first[name] != second[name] {
			t.Errorf("%s differs between two compiles:\n%s\n---\n%s", name, first[name], second[name])
		}
	}

	// a stored file that already holds a block (pasted by hand, or copied from a
	// compiled file) gets one block, not two
	pasted := file("reviewer", "squadIndependent: true\n")
	pasted.Data = append(pasted.Data, []byte("\n<!-- squad:independent -->\nold text\n<!-- /squad:independent -->\n")...)
	got := compile(t, bundle(nil, "", file("dev", ""), pasted))["reviewer"]
	if strings.Count(got, "<!-- squad:independent -->") != 1 || strings.Contains(got, "old text") {
		t.Errorf("reviewer:\n%s", got)
	}
}

func TestTheLeadGetsTheFlowTheLoopAndWhoIsIndependent(t *testing.T) {
	flow := &preset.Flow{Gate: "reviewer", Fix: "dev", Rounds: 2}
	b := bundle(flow, "lead", file("lead", ""), file("dev", ""), file("reviewer", "squadIndependent: true\n"))
	got := compile(t, b)

	lead := got["lead"]
	for _, want := range []string{"<!-- squad:lead -->", "1 dev → 2 reviewer", "Independent: `reviewer`", "Give them only the original task and the instructions", "Never your own conclusions", "After 2 rounds"} {
		if !strings.Contains(lead, want) {
			t.Errorf("lead lacks %q:\n%s", want, lead)
		}
	}
	if strings.Contains(got["dev"], "squad:lead") {
		t.Errorf("a worker got the lead's brief:\n%s", got["dev"])
	}
}

func TestTheBriefWithoutAFlowStillSaysWhoIsIndependent(t *testing.T) {
	brief := team.Brief(bundle(nil, "", file("dev", ""), file("reviewer", "squadIndependent: true\n")))

	if !strings.Contains(brief, "Independent: `reviewer`") || strings.Contains(brief, "Delegate in this order") || strings.Contains(brief, "Loop") {
		t.Errorf("brief:\n%s", brief)
	}
}

func TestNothingToTellMeansNoBrief(t *testing.T) {
	if got := team.Brief(bundle(nil, "", file("dev", ""), file("tester", ""))); got != "" {
		t.Errorf("a preset with no flow and nobody independent got a brief:\n%s", got)
	}
	if got := team.Brief(bundle(&preset.Flow{}, "", file("dev", ""))); got != "" {
		t.Errorf("one agent got a brief:\n%s", got)
	}
}

func TestBriefForARunWithoutAMainAgent(t *testing.T) {
	brief := team.Brief(bundle(&preset.Flow{Gate: "tester", Fix: "dev"}, "", file("dev", ""), file("tester", "")))

	if !strings.Contains(brief, "1 dev → 2 tester") || !strings.Contains(brief, "After 3 rounds") {
		t.Errorf("brief:\n%s", brief)
	}
}

func TestIndependentAndSetIndependent(t *testing.T) {
	a := mustParse(t, string(file("dev", "").Data))
	if team.Independent(a) {
		t.Fatal("agents are not independent by default")
	}
	team.SetIndependent(a, true)
	if !team.Independent(a) {
		t.Fatal("SetIndependent(true) did nothing")
	}
	data, _ := a.Marshal()
	if !strings.Contains(string(data), "squadIndependent: true") {
		t.Errorf("not written:\n%s", data)
	}
	team.SetIndependent(a, false)
	data, _ = a.Marshal()
	if strings.Contains(string(data), team.IndependentKey) {
		t.Errorf("the default is written out:\n%s", data)
	}
}

func TestWhatTheToolsAllow(t *testing.T) {
	cases := map[string]bool{
		"":                                       true,
		"tools: Read, Grep\n":                    false,
		"tools: Read, Edit, Agent\n":             true,
		"disallowedTools: Agent\n":               false,
		"tools: Agent\ndisallowedTools: Agent\n": false,
	}
	for extra, want := range cases {
		if got := team.CanDelegate(mustParse(t, string(file("x", extra).Data))); got != want {
			t.Errorf("%q: delegate %v, want %v", extra, got, want)
		}
	}
}

func TestTheCharterIsShort(t *testing.T) {
	r := compile(t, bundle(nil, "", file("reviewer", "squadIndependent: true\n")))["reviewer"]
	block := r[strings.Index(r, "<!-- squad:independent -->"):]
	if words := len(strings.Fields(block)); words > 90 {
		t.Errorf("the charter is %d words; every call of the agent pays for it", words)
	}
}

func mustParse(t *testing.T, data string) *agentfile.Agent {
	t.Helper()
	a, err := agentfile.Parse([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestTheOldShareFieldIsNeverSentToClaudeCode(t *testing.T) {
	got := compile(t, bundle(nil, "", file("dev", "squadShare: false\n"), file("tester", "")))

	if strings.Contains(got["dev"], "squadShare") {
		t.Errorf("dev:\n%s", got["dev"])
	}
	if team.Independent(mustParse(t, string(file("dev", "squadShare: false\n").Data))) {
		t.Error("the old field must not make an agent independent")
	}
}

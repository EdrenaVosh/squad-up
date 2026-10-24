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

func TestAgentsShareTheNotesByDefault(t *testing.T) {
	got := compile(t, bundle(nil, "", file("dev", ""), file("tester", "")))

	for name, data := range got {
		if !strings.Contains(data, team.NotesFile) || !strings.Contains(data, "<!-- squad:notes -->") {
			t.Errorf("%s does not get the notes block:\n%s", name, data)
		}
		if !strings.HasPrefix(data, "---\nname: "+name) || !strings.Contains(data, "You are "+name+".") {
			t.Errorf("%s lost its own text:\n%s", name, data)
		}
	}
}

func TestAnIsolatedAgentGetsNoNotesAndNoSquadKey(t *testing.T) {
	got := compile(t, bundle(nil, "", file("dev", ""), file("reviewer", "squadShare: false\n")))

	if strings.Contains(got["reviewer"], "squad:notes") || strings.Contains(got["reviewer"], team.ShareKey) {
		t.Errorf("reviewer:\n%s", got["reviewer"])
	}
	if !strings.Contains(got["dev"], "squad:notes") {
		t.Errorf("dev:\n%s", got["dev"])
	}
}

func TestALoneAgentHasNobodyToShareWith(t *testing.T) {
	b := bundle(nil, "", file("dev", "squadShare: false\n"))
	got := compile(t, b)
	if strings.Contains(got["dev"], "squad:notes") || strings.Contains(got["dev"], team.ShareKey) {
		t.Errorf("dev:\n%s", got["dev"])
	}

	b = bundle(nil, "", file("dev", ""))
	if got := compile(t, b); got["dev"] != string(b.Agents[0].Data) {
		t.Errorf("an untouched agent changed:\n%s", got["dev"])
	}
}

func TestCompilingTwiceChangesNothing(t *testing.T) {
	b := bundle(&preset.Flow{Gate: "tester", Fix: "dev"}, "lead", file("lead", ""), file("dev", ""), file("tester", ""))
	first, err := team.Compile(b)
	if err != nil {
		t.Fatal(err)
	}
	again, err := team.Compile(preset.Bundle{Preset: b.Preset, Agents: first})
	if err != nil {
		t.Fatal(err)
	}
	for i := range first {
		if string(first[i].Data) != string(again[i].Data) {
			t.Errorf("%s changed on the second pass:\n%s\n---\n%s", first[i].Name, first[i].Data, again[i].Data)
		}
	}
}

func TestTheLeadGetsTheFlowAndTheLoop(t *testing.T) {
	flow := &preset.Flow{Gate: "reviewer", Fix: "dev", Rounds: 2}
	b := bundle(flow, "lead", file("lead", ""), file("dev", ""), file("reviewer", "squadShare: false\n"))
	got := compile(t, b)

	lead := got["lead"]
	for _, want := range []string{"<!-- squad:lead -->", "1 dev → 2 reviewer", "Isolated", "reviewer", "`dev`", "After 2 rounds"} {
		if !strings.Contains(lead, want) {
			t.Errorf("lead lacks %q:\n%s", want, lead)
		}
	}
	if strings.Contains(got["dev"], "squad:lead") {
		t.Errorf("a worker got the lead's brief:\n%s", got["dev"])
	}
}

func TestTheBriefWithoutALoopOrNotes(t *testing.T) {
	b := bundle(&preset.Flow{}, "lead", file("lead", "squadShare: false\n"), file("dev", "squadShare: false\n"))
	lead := compile(t, b)["lead"]

	if strings.Contains(lead, "Loop") || strings.Contains(lead, team.NotesFile) {
		t.Errorf("unexpected loop or notes:\n%s", lead)
	}
	if !strings.Contains(lead, "1 dev") {
		t.Errorf("no order:\n%s", lead)
	}
}

func TestBriefForARunWithoutAMainAgent(t *testing.T) {
	b := bundle(&preset.Flow{Gate: "tester", Fix: "dev"}, "", file("dev", ""), file("tester", ""))

	brief := team.Brief(b)

	if !strings.Contains(brief, "1 dev → 2 tester") || !strings.Contains(brief, "After 3 rounds") {
		t.Errorf("brief:\n%s", brief)
	}
	if team.Brief(bundle(nil, "", file("dev", ""), file("tester", ""))) != "" {
		t.Error("a preset without a flow got a brief")
	}
	if team.Brief(bundle(&preset.Flow{}, "", file("dev", ""))) != "" {
		t.Error("one agent got a brief")
	}
}

func TestSharedAndSetShared(t *testing.T) {
	a, err := agentfile.Parse(file("dev", "").Data)
	if err != nil {
		t.Fatal(err)
	}
	if !team.Shared(a) {
		t.Fatal("agents share by default")
	}
	team.SetShared(a, false)
	if team.Shared(a) {
		t.Fatal("SetShared(false) did not isolate")
	}
	data, _ := a.Marshal()
	if !strings.Contains(string(data), "squadShare: false") {
		t.Errorf("isolation not written:\n%s", data)
	}
	team.SetShared(a, true)
	data, _ = a.Marshal()
	if strings.Contains(string(data), team.ShareKey) {
		t.Errorf("the default is written out:\n%s", data)
	}
}

func TestTheNotesBlockIsSmall(t *testing.T) {
	got := compile(t, bundle(nil, "", file("dev", ""), file("tester", "")))
	block := got["dev"][strings.Index(got["dev"], "<!-- squad:notes -->"):]

	if words := len(strings.Fields(block)); words > 140 {
		t.Errorf("the notes block is %d words; it is paid for by every agent on every run", words)
	}
}

func TestAnAgentThatCannotWriteHandsItsLinesToTheLead(t *testing.T) {
	flow := &preset.Flow{Gate: "reviewer", Fix: "dev"}
	b := bundle(flow, "lead", file("lead", ""), file("dev", ""), file("reviewer", "tools: Read, Grep, Glob\n"))
	got := compile(t, b)

	if !strings.Contains(got["reviewer"], "You cannot edit it") || strings.Contains(got["reviewer"], "update it before") {
		t.Errorf("reviewer:\n%s", got["reviewer"])
	}
	if !strings.Contains(got["lead"], "cannot write end their answer") {
		t.Errorf("the lead is not told to record their lines:\n%s", got["lead"])
	}
}

func TestAnAgentThatCannotReadGetsNothing(t *testing.T) {
	got := compile(t, bundle(nil, "", file("dev", ""), file("writer", "tools: Write\n")))

	if strings.Contains(got["writer"], "squad:notes") {
		t.Errorf("writer:\n%s", got["writer"])
	}
}

func TestDisallowedToolsTakeAccessAway(t *testing.T) {
	got := compile(t, bundle(nil, "", file("dev", ""), file("ro", "disallowedTools: Write, Edit, Bash\n")))

	if !strings.Contains(got["ro"], "You cannot edit it") {
		t.Errorf("ro:\n%s", got["ro"])
	}
	if !strings.Contains(got["dev"], "update it before") {
		t.Errorf("dev:\n%s", got["dev"])
	}
}

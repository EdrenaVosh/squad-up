package templates_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/EdrenaVosh/squad-up/internal/agentfile"
	"github.com/EdrenaVosh/squad-up/internal/lint"
	"github.com/EdrenaVosh/squad-up/internal/team"
	"github.com/EdrenaVosh/squad-up/templates"
)

// what each template is made of: its agents, in order, with their models.
var want = map[string][]struct{ name, model string }{
	"bug-hunter":      {{"debugger", "opus"}, {"fixer", "sonnet"}, {"reviewer", "sonnet"}, {"tester", "sonnet"}},
	"feature-creator": {{"planner", "opus"}, {"executor", "sonnet"}, {"reviewer", "sonnet"}},
}

func load(t *testing.T, id string) map[string]*agentfile.Agent {
	t.Helper()
	b, err := templates.Load(id)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]*agentfile.Agent{}
	for _, f := range b.Agents {
		a, err := agentfile.Parse(f.Data)
		if err != nil {
			t.Fatalf("%s/%s: %v", id, f.Name, err)
		}
		out[f.Name] = a
	}
	return out
}

func tools(a *agentfile.Agent) []string {
	v, _ := a.Get("tools")
	var out []string
	switch x := v.(type) {
	case string:
		for _, t := range strings.Split(x, ",") {
			out = append(out, strings.TrimSpace(t))
		}
	case []any:
		for _, t := range x {
			out = append(out, t.(string))
		}
	}
	return out
}

func TestThereAreExactlyTwoStarterTemplates(t *testing.T) {
	list, err := templates.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].ID != "bug-hunter" || list[1].ID != "feature-creator" {
		t.Fatalf("templates = %+v", list)
	}
	for _, tpl := range list {
		var names []string
		for _, w := range want[tpl.ID] {
			names = append(names, w.name)
		}
		if !slices.Equal(tpl.Agents, names) {
			t.Errorf("%s agents = %v, want %v", tpl.ID, tpl.Agents, names)
		}
		if tpl.Title == "" || tpl.Description == "" {
			t.Errorf("%s needs a title and a description: %+v", tpl.ID, tpl)
		}
	}
	if list[0].Title != "Bug Hunter" || list[1].Title != "Feature Creator" {
		t.Errorf("titles = %q, %q", list[0].Title, list[1].Title)
	}
}

func TestEveryAgentHasTheRightModel(t *testing.T) {
	for id, agents := range want {
		got := load(t, id)
		for _, w := range agents {
			if v, _ := got[w.name].Get("model"); v != w.model {
				t.Errorf("%s/%s model = %v, want %s", id, w.name, v, w.model)
			}
		}
	}
}

func TestEveryTemplateIsAValidPresetThatLintsClean(t *testing.T) {
	for id := range want {
		t.Run(id, func(t *testing.T) {
			b, err := templates.Load(id)
			if err != nil {
				t.Fatal(err)
			}
			if b.Preset.Name != id {
				t.Errorf("name = %q", b.Preset.Name)
			}
			if err := b.Preset.Validate(); err != nil {
				t.Fatal(err)
			}
			in := lint.Input{Name: id, Preset: &b.Preset}
			for _, a := range b.Agents {
				in.Agents = append(in.Agents, lint.Agent{File: a.Name + ".md", Data: a.Data})
			}
			for _, f := range lint.Run(in).Findings {
				if f.Level != lint.LevelInfo {
					t.Errorf("%s %s: %s", f.Level, f.File, f.Message)
				}
			}
		})
	}
}

func TestEveryPromptSaysWhatItReceivesHowItWorksAndWhatItReports(t *testing.T) {
	for id := range want {
		for name, a := range load(t, id) {
			prompt := a.Prompt()
			for _, section := range []string{"## What you receive", "## How you work", "## Rules", "## What you report"} {
				if name == "reviewer" && section == "## How you work" {
					section = "## How you review"
				}
				if !strings.Contains(prompt, section) {
					t.Errorf("%s/%s has no %q", id, name, section)
				}
			}
			if len(strings.Fields(prompt)) < 250 {
				t.Errorf("%s/%s: the prompt is too thin to be useful (%d words)", id, name, len(strings.Fields(prompt)))
			}
			if desc := a.Description(); !strings.Contains(desc, "Use ") || len(desc) < 80 {
				t.Errorf("%s/%s: the description must say what it does and when to use it: %q", id, name, desc)
			}
			if a.Role() == "" || len(a.Role()) > 90 {
				t.Errorf("%s/%s: the first sentence is the role shown on the tile and must be short: %q", id, name, a.Role())
			}
		}
	}
}

func TestOnlyTheReviewersAreIndependentAndTheyCannotEdit(t *testing.T) {
	for id := range want {
		for name, a := range load(t, id) {
			isReviewer := name == "reviewer"
			if team.Independent(a) != isReviewer {
				t.Errorf("%s/%s independent = %v", id, name, team.Independent(a))
			}
			canEdit := slices.Contains(tools(a), "Edit") || slices.Contains(tools(a), "Write")
			switch name {
			case "reviewer", "debugger", "planner":
				if canEdit {
					t.Errorf("%s/%s must not be able to change files: %v", id, name, tools(a))
				}
			case "fixer", "tester", "executor":
				if !canEdit {
					t.Errorf("%s/%s must be able to change files: %v", id, name, tools(a))
				}
			}
		}
	}
}

func TestTheFlowsAreReviewLoopsThatSendDefectsToTheWorker(t *testing.T) {
	for id, fix := range map[string]string{"bug-hunter": "fixer", "feature-creator": "executor"} {
		b, err := templates.Load(id)
		if err != nil {
			t.Fatal(err)
		}
		f := b.Preset.Flow
		if f == nil || f.Gate != "reviewer" || f.Fix != fix || !f.Loops() {
			t.Errorf("%s flow = %+v, want reviewer checking and %s fixing", id, f, fix)
		}
	}
}

func TestTheFixerNeverTouchesTests(t *testing.T) {
	prompt := load(t, "bug-hunter")["fixer"].Prompt()
	for _, want := range []string{"Never create, edit, delete or skip a test", "*_test.*", "tester"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the fixer's prompt lacks %q", want)
		}
	}
}

func TestLoadRejectsAnUnknownTemplate(t *testing.T) {
	if _, err := templates.Load("nope"); err == nil || !strings.Contains(err.Error(), "bug-hunter") {
		t.Errorf("err = %v, want it to list the templates", err)
	}
}

func TestTemplatesAreAlwaysFreshCopies(t *testing.T) {
	a, _ := templates.Load("bug-hunter")
	a.Agents[0].Data[0] = 'X'
	b, _ := templates.Load("bug-hunter")
	if b.Agents[0].Data[0] == 'X' {
		t.Error("Load shares memory between calls")
	}
}

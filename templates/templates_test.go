package templates_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/EdrenaVosh/squadup/internal/agentfile"
	"github.com/EdrenaVosh/squadup/internal/lint"
	"github.com/EdrenaVosh/squadup/internal/preset"
	"github.com/EdrenaVosh/squadup/templates"
)

var wantAgents = map[string][]string{
	"solo-indie":      {"planner", "reviewer", "tester"},
	"bug-hunt":        {"debugger", "tester"},
	"refactor-day":    {"architect", "refactorer", "reviewer"},
	"ship-it":         {"release-checker", "changelog-writer"},
	"read-only-audit": {"security-auditor", "reviewer"},
}

func TestTheFiveStarterTemplatesAreThere(t *testing.T) {
	list, err := templates.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != len(wantAgents) {
		t.Fatalf("got %d templates, want %d", len(list), len(wantAgents))
	}
	for _, tpl := range list {
		want, ok := wantAgents[tpl.ID]
		if !ok {
			t.Errorf("unexpected template %q", tpl.ID)
			continue
		}
		if !slices.Equal(tpl.Agents, want) {
			t.Errorf("%s agents = %v, want %v", tpl.ID, tpl.Agents, want)
		}
		if tpl.Title == "" || tpl.Description == "" {
			t.Errorf("%s needs a title and a description: %+v", tpl.ID, tpl)
		}
	}
}

func TestEveryTemplateIsAValidPresetThatLintsClean(t *testing.T) {
	for id := range wantAgents {
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
			report := lint.Run(in)
			for _, f := range report.Findings {
				if f.Level != lint.LevelInfo {
					t.Errorf("%s %s: %s", f.Level, f.File, f.Message)
				}
			}
		})
	}
}

func TestTemplatePromptsAreConcrete(t *testing.T) {
	for id := range wantAgents {
		b, err := templates.Load(id)
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range b.Agents {
			agent, err := agentfile.Parse(a.Data)
			if err != nil {
				t.Fatal(err)
			}
			prompt := agent.Prompt()
			if !strings.Contains(prompt, "## Response format") {
				t.Errorf("%s/%s has no response format", id, a.Name)
			}
			if len(strings.Fields(prompt)) < 60 {
				t.Errorf("%s/%s: the prompt is too thin to be useful", id, a.Name)
			}
			if _, ok := agent.Get("tools"); !ok {
				t.Errorf("%s/%s lists no tools", id, a.Name)
			}
		}
	}
}

func TestTheAuditTemplateCanOnlyRead(t *testing.T) {
	b, err := templates.Load("read-only-audit")
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range b.Agents {
		agent, err := agentfile.Parse(a.Data)
		if err != nil {
			t.Fatal(err)
		}
		tools, _ := agent.Get("tools")
		for _, tool := range strings.Split(strings.ReplaceAll(strings.Trim(strings.TrimSpace(toString(tools)), "[]"), " ", ""), ",") {
			if !slices.Contains([]string{"Read", "Grep", "Glob"}, tool) {
				t.Errorf("%s may use %q", a.Name, tool)
			}
		}
	}
}

func toString(v any) string {
	if list, ok := v.([]any); ok {
		parts := make([]string, len(list))
		for i, item := range list {
			parts[i], _ = item.(string)
		}
		return strings.Join(parts, ",")
	}
	s, _ := v.(string)
	return s
}

func TestLoadRejectsAnUnknownTemplate(t *testing.T) {
	if _, err := templates.Load("nope"); err == nil || !strings.Contains(err.Error(), "solo-indie") {
		t.Errorf("err = %v, want it to list the templates", err)
	}
}

func TestTemplatesAreAlwaysFreshCopies(t *testing.T) {
	a, _ := templates.Load("bug-hunt")
	a.Agents[0].Data[0] = 'X'
	b, _ := templates.Load("bug-hunt")
	if b.Agents[0].Data[0] == 'X' {
		t.Error("Load shares memory between calls")
	}
	_ = preset.Bundle{}
}

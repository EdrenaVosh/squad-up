package apply_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/EdrenaVosh/squad-up/internal/apply"
	"github.com/EdrenaVosh/squad-up/internal/initial"
	"github.com/EdrenaVosh/squad-up/internal/paths"
	"github.com/EdrenaVosh/squad-up/internal/preset"
	"github.com/EdrenaVosh/squad-up/internal/testutil"
	"github.com/tidwall/gjson"
)

const originalSettings = `{
  "theme": "dark",
  "model": "opus",
  "env": { "SECRET": "keep-out" },
  "permissions": {
    "defaultMode": "default",
    "allow": ["Bash(git status)"],
    "deny": ["Read(./.env)"]
  },
  "hooks": { "SessionStart": [ { "hooks": [{ "type": "command", "command": "true" }] } ] },
  "effortLevel": "medium"
}
`

func agentMD(name, body string) string {
	return "---\nname: " + name + "\ndescription: " + name + " agent. Use when " + name + " is needed.\n---\n" + body + "\n"
}

// fx is a foreign Claude Code setup with Initial captured and three presets
// stored: solo (replace), bug-hunt (replace, main agent) and extra (extend).
type fx struct {
	t      *testing.T
	p      paths.Paths
	e      apply.Engine
	clock  time.Time
	store  preset.Store
	origin map[string]string // the ClaudeDir tree right after Initial was captured
}

func newFx(t *testing.T, files map[string]string) *fx {
	t.Helper()
	f := &fx{
		t:     t,
		p:     paths.Paths{ClaudeDir: filepath.Join(t.TempDir(), "claude"), Home: filepath.Join(t.TempDir(), "squad-up")},
		clock: time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC),
	}
	if files == nil {
		files = map[string]string{
			"agents/reviewer.md":      "---\r\nname: reviewer\r\ndescription: Reviews. Use after changes.\r\n---\r\nBe strict.\r\n",
			"agents/team/tester.md":   agentMD("tester", "Initial tester."),
			"agents/docs-writer.md":   agentMD("docs-writer", "Initial docs."),
			"agents/notes.txt":        "not an agent",
			"settings.json":           originalSettings,
			"projects/p/session.json": "{}",
		}
	}
	testutil.WriteFiles(t, f.p.ClaudeDir, files)
	if _, err := initial.Capture(f.p, f.now()); err != nil {
		t.Fatal(err)
	}
	f.e = apply.Engine{Paths: f.p, Now: f.now}
	f.store = preset.Store{Dir: f.p.PresetsDir()}
	f.origin = f.tree()

	main := "debugger"
	f.save(preset.Bundle{
		Preset: preset.Preset{
			Schema: 1, Name: "solo", Title: "Solo", Mode: preset.ModeReplace,
			Agents: []string{"planner", "reviewer", "tester"},
			Settings: preset.Settings{
				Model: "sonnet", EffortLevel: "high",
				Permissions: &preset.Permissions{DefaultMode: "acceptEdits", Allow: []string{"Bash(npm run *)"}, Ask: []string{"Bash(git push *)"}},
			},
		},
		Agents: []preset.AgentFile{
			{Name: "planner", Data: []byte(agentMD("planner", "Plan it."))},
			{Name: "reviewer", Data: []byte(agentMD("reviewer", "Solo reviewer."))},
			{Name: "tester", Data: []byte(agentMD("tester", "Solo tester."))},
		},
	})
	f.save(preset.Bundle{
		Preset: preset.Preset{
			Schema: 1, Name: "bug-hunt", Title: "Bug Hunt", Mode: preset.ModeReplace, Main: &main,
			Agents:   []string{"debugger", "tester"},
			Settings: preset.Settings{Model: "haiku"},
		},
		Agents: []preset.AgentFile{
			{Name: "debugger", Data: []byte(agentMD("debugger", "Find it."))},
			{Name: "tester", Data: []byte(agentMD("tester", "Hunt tester."))},
		},
	})
	f.save(preset.Bundle{
		Preset: preset.Preset{
			Schema: 1, Name: "extra", Mode: preset.ModeExtend,
			Agents: []string{"helper"},
		},
		Agents: []preset.AgentFile{{Name: "helper", Data: []byte(agentMD("helper", "Help."))}},
	})
	return f
}

// now is the injected clock: every call moves one second forward so that
// backup ids never collide.
func (f *fx) now() time.Time {
	f.clock = f.clock.Add(time.Second)
	return f.clock
}

func (f *fx) save(b preset.Bundle) {
	f.t.Helper()
	if err := f.store.Save(b); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fx) tree() map[string]string { return testutil.Tree(f.t, f.p.ClaudeDir) }

// use plans and executes `squad use name`, failing the test on any error.
func (f *fx) use(name string, opts apply.Options) *apply.Result {
	f.t.Helper()
	plan, err := f.e.PlanUse(name, opts)
	if err != nil {
		f.t.Fatalf("PlanUse(%s) error = %v", name, err)
	}
	res, err := f.e.Execute(plan, opts)
	if err != nil {
		f.t.Fatalf("Execute(use %s) error = %v", name, err)
	}
	return res
}

func (f *fx) undo(id string, opts apply.Options) *apply.Result {
	f.t.Helper()
	plan, err := f.e.PlanUndo(id, opts)
	if err != nil {
		f.t.Fatalf("PlanUndo(%q) error = %v", id, err)
	}
	res, err := f.e.Execute(plan, opts)
	if err != nil {
		f.t.Fatalf("Execute(undo %q) error = %v", id, err)
	}
	return res
}

func (f *fx) settingsString(path string) string {
	f.t.Helper()
	return gjson.Get(f.tree()["settings.json"], path).String()
}

func (f *fx) state() apply.State {
	f.t.Helper()
	s, err := apply.LoadState(f.p)
	if err != nil {
		f.t.Fatal(err)
	}
	return s
}

func extendPresetNamed(name, agent string) preset.Bundle {
	return preset.Bundle{
		Preset: preset.Preset{Schema: 1, Name: name, Mode: preset.ModeExtend, Agents: []string{agent}},
		Agents: []preset.AgentFile{{Name: agent, Data: []byte(agentMD(agent, "duplicate of an Initial agent"))}},
	}
}

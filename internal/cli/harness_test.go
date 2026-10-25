package cli_test

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/EdrenaVosh/squad-up/internal/cli"
	"github.com/EdrenaVosh/squad-up/internal/preset"
	"github.com/EdrenaVosh/squad-up/internal/testutil"
)

var fixedNow = time.Date(2026, 10, 24, 9, 30, 0, 0, time.UTC)

const (
	reviewerMD = "---\nname: reviewer\ndescription: Reviews diffs. Use after any code change.\ncolor: green\n---\nBe strict.\n"
	testerMD   = "---\nname: tester\ndescription: Writes tests. Use when coverage is missing.\n---\nTest it.\n"
	docsMD     = "---\nname: docs-writer\ndescription: Updates docs. Use after changes.\n---\nKeep it short.\n"

	settingsJSON = `{
  "theme": "dark",
  "model": "opus",
  "env": { "SECRET": "keep-out" },
  "permissions": {
    "defaultMode": "acceptEdits",
    "allow": ["Bash(npm run *)", "Bash(git status)", "Bash(git diff *)", "Read(./docs/**)"],
    "ask": ["Bash(git push *)"],
    "deny": ["Read(./.env)", "Read(./.env.*)"]
  },
  "effortLevel": "high"
}
`
)

// env is an isolated pair of config directories plus a fixed clock. Every test
// runs squad against it, never against the real ~/.claude or ~/.squad-up.
type env struct {
	t         *testing.T
	claudeDir string
	home      string
}

type result struct {
	stdout, stderr string
	code           int
}

func newEnv(t *testing.T) *env {
	t.Helper()
	e := &env{
		t:         t,
		claudeDir: filepath.Join(t.TempDir(), "claude"),
		home:      filepath.Join(t.TempDir(), "squad-up"),
	}
	t.Setenv("CLAUDE_CONFIG_DIR", e.claudeDir)
	t.Setenv("SQUAD_UP_HOME", e.home)
	t.Setenv("ANTHROPIC_MODEL", "")
	testutil.HideClaude(t)
	return e
}

func (e *env) writeSetup() {
	e.t.Helper()
	testutil.WriteFiles(e.t, e.claudeDir, map[string]string{
		"agents/reviewer.md":    reviewerMD,
		"agents/team/tester.md": testerMD,
		"agents/docs-writer.md": docsMD,
		"settings.json":         settingsJSON,
	})
}

func (e *env) run(args ...string) result {
	e.t.Helper()
	return e.runWith("", false, args...)
}

func (e *env) runWith(stdin string, interactive bool, args ...string) result {
	e.t.Helper()
	var out, errOut bytes.Buffer
	code := cli.Run(args, cli.IO{
		In:          strings.NewReader(stdin),
		Out:         &out,
		Err:         &errOut,
		Interactive: interactive,
		Now:         func() time.Time { return fixedNow },
	})
	return result{stdout: out.String(), stderr: errOut.String(), code: code}
}

func agentMD(name, body string) string {
	return "---\nname: " + name + "\ndescription: " + name + " agent. Use when " + name + " is needed.\n---\n" + body + "\n"
}

func (e *env) tree() map[string]string { return testutil.Tree(e.t, e.claudeDir) }

// addPresets stores two replace-mode presets: solo and bug-hunt.
func (e *env) addPresets() {
	e.t.Helper()
	store := preset.Store{Dir: filepath.Join(e.home, "presets")}
	main := "debugger"
	for _, b := range []preset.Bundle{
		{
			Preset: preset.Preset{
				Schema: 1, Name: "solo", Title: "Solo", Mode: preset.ModeReplace,
				Agents:   []string{"planner", "reviewer", "tester"},
				Settings: preset.Settings{Model: "sonnet", EffortLevel: "high"},
			},
			Agents: []preset.AgentFile{
				{Name: "planner", Data: []byte(agentMD("planner", "Plan it."))},
				{Name: "reviewer", Data: []byte(agentMD("reviewer", "Solo reviewer."))},
				{Name: "tester", Data: []byte(agentMD("tester", "Solo tester."))},
			},
		},
		{
			Preset: preset.Preset{
				Schema: 1, Name: "bug-hunt", Title: "Bug Hunt", Mode: preset.ModeReplace, Main: &main,
				Agents:   []string{"debugger", "tester"},
				Settings: preset.Settings{Model: "haiku"},
			},
			Agents: []preset.AgentFile{
				{Name: "debugger", Data: []byte(agentMD("debugger", "Find it."))},
				{Name: "tester", Data: []byte(agentMD("tester", "Hunt tester."))},
			},
		},
	} {
		if err := store.Save(b); err != nil {
			e.t.Fatal(err)
		}
	}
}

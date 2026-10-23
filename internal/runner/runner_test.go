package runner_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/EdrenaVosh/squadup/internal/preset"
	"github.com/EdrenaVosh/squadup/internal/runner"
	"github.com/EdrenaVosh/squadup/internal/testutil"
)

const reviewerMD = `---
name: reviewer
description: Reviews diffs. Use after any code change, before commit.
tools: Read, Grep, Glob
model: sonnet
permissionMode: default
maxTurns: 12
color: green
experimental:
  cacheTtl: 1h
custom_field: ignored
hooks:
  PreToolUse:
    - matcher: Bash
      hooks:
        - type: command
          command: echo checking
---
You are a strict code reviewer.
Report findings by severity.
`

const plannerMD = "---\nname: planner\ndescription: Plans. Use before large changes.\ntools:\n  - Read\n  - Glob\n---\nPlan it.\n"

func bundle(main *string) preset.Bundle {
	return preset.Bundle{
		Preset: preset.Preset{Schema: 1, Name: "solo", Mode: preset.ModeReplace, Main: main, Agents: []string{"planner", "reviewer"}},
		Agents: []preset.AgentFile{
			{Name: "planner", Data: []byte(plannerMD)},
			{Name: "reviewer", Data: []byte(reviewerMD)},
		},
	}
}

func TestBuildTurnsAgentFilesIntoTheAgentsFlag(t *testing.T) {
	spec, err := runner.Build(bundle(nil), nil)
	if err != nil {
		t.Fatal(err)
	}

	var got map[string]map[string]any
	if err := json.Unmarshal([]byte(spec.AgentsJSON), &got); err != nil {
		t.Fatalf("--agents value is not JSON: %v\n%s", err, spec.AgentsJSON)
	}
	reviewer := got["reviewer"]
	if reviewer["description"] != "Reviews diffs. Use after any code change, before commit." {
		t.Errorf("description = %v", reviewer["description"])
	}
	if reviewer["prompt"] != "You are a strict code reviewer.\nReport findings by severity.\n" {
		t.Errorf("prompt = %q", reviewer["prompt"])
	}
	if !reflect.DeepEqual(reviewer["tools"], []any{"Read", "Grep", "Glob"}) {
		t.Errorf("tools = %v, want the comma list as an array", reviewer["tools"])
	}
	if reviewer["model"] != "sonnet" || reviewer["permissionMode"] != "default" || reviewer["maxTurns"] != float64(12) {
		t.Errorf("reviewer = %v", reviewer)
	}
	for _, absent := range []string{"name", "color", "experimental", "custom_field"} {
		if _, ok := reviewer[absent]; ok {
			t.Errorf("%q must not be passed in --agents", absent)
		}
	}
	hooks, _ := reviewer["hooks"].(map[string]any)
	if _, ok := hooks["PreToolUse"]; !ok {
		t.Errorf("hooks = %v, want the nested structure kept", reviewer["hooks"])
	}
	if !reflect.DeepEqual(got["planner"]["tools"], []any{"Read", "Glob"}) {
		t.Errorf("a YAML list of tools must stay a list: %v", got["planner"]["tools"])
	}
}

func TestBuildComposesTheSettingsFile(t *testing.T) {
	resolved := map[string]string{
		"model":       `"sonnet"`,
		"effortLevel": `"high"`,
		"permissions": "{\n    \"defaultMode\": \"acceptEdits\",\n    \"allow\": [\n      \"Bash(npm run *)\"\n    ]\n  }",
		"agent":       `"reviewer"`,
	}

	spec, err := runner.Build(bundle(nil), resolved)

	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(spec.SettingsJSON, &got); err != nil {
		t.Fatalf("settings are not JSON: %v\n%s", err, spec.SettingsJSON)
	}
	if got["model"] != "sonnet" || got["effortLevel"] != "high" {
		t.Errorf("settings = %v", got)
	}
	perms, _ := got["permissions"].(map[string]any)
	if perms["defaultMode"] != "acceptEdits" {
		t.Errorf("permissions = %v", got["permissions"])
	}
	if _, ok := got["agent"]; ok {
		t.Error("the main agent goes through --agent, not the settings file")
	}
	if spec.Main != "reviewer" {
		t.Errorf("Main = %q: with no main in the preset it falls back to the resolved agent key", spec.Main)
	}
}

func TestPresetMainWinsOverTheResolvedAgent(t *testing.T) {
	main := "planner"

	spec, err := runner.Build(bundle(&main), map[string]string{"agent": `"reviewer"`})

	if err != nil || spec.Main != "planner" {
		t.Fatalf("Main = %q, %v", spec.Main, err)
	}
}

func TestArgsPutSquadFlagsFirstAndUserArgsLast(t *testing.T) {
	main := "planner"
	spec, err := runner.Build(bundle(&main), nil)
	if err != nil {
		t.Fatal(err)
	}

	args := spec.Args("/tmp/settings.json", []string{"--model", "opus", "fix the bug"})

	want := []string{"--agents", spec.AgentsJSON, "--settings", "/tmp/settings.json", "--agent", "planner", "--model", "opus", "fix the bug"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("Args() = %q\nwant %q", args, want)
	}
}

func TestAPresetWithoutAgentsOmitsTheAgentsFlag(t *testing.T) {
	b := preset.Bundle{Preset: preset.Preset{Schema: 1, Name: "bare", Mode: preset.ModeReplace, Agents: []string{}}}

	spec, err := runner.Build(b, map[string]string{"model": `"haiku"`})
	if err != nil {
		t.Fatal(err)
	}

	for _, a := range spec.Args("/tmp/s.json", nil) {
		if a == "--agents" {
			t.Fatal("--agents must be left out when the preset has no agents")
		}
	}
}

func TestBuildRefusesAnArgumentThatIsTooLargeToPass(t *testing.T) {
	big := bundle(nil)
	big.Agents[0].Data = []byte("---\nname: planner\ndescription: Plans. Use before large changes.\n---\n" + strings.Repeat("x", 200_000))

	_, err := runner.Build(big, nil)

	if err == nil || !strings.Contains(err.Error(), "squad use") {
		t.Fatalf("Build() error = %v, want advice to use `squad use` instead", err)
	}
}

func TestBuildReportsAnUnreadableAgent(t *testing.T) {
	b := bundle(nil)
	b.Agents[1].Data = []byte("no frontmatter")

	_, err := runner.Build(b, nil)

	if err == nil || !strings.Contains(err.Error(), "reviewer") {
		t.Fatalf("Build() error = %v", err)
	}
}

// ---- running ----

func run(t *testing.T, extra []string) (code int, stdout string, err error) {
	t.Helper()
	bin := testutil.FakeClaude(t)
	spec, buildErr := runner.Build(bundle(nil), map[string]string{"model": `"sonnet"`})
	if buildErr != nil {
		t.Fatal(buildErr)
	}
	var out, errOut bytes.Buffer
	code, err = runner.Run(bin, spec, extra, runner.Streams{In: strings.NewReader(""), Out: &out, Err: &errOut})
	return code, out.String(), err
}

func TestRunPassesArgumentsAndOutputThrough(t *testing.T) {
	argsFile := filepath.Join(t.TempDir(), "args")
	t.Setenv("FAKE_CLAUDE_ARGS_FILE", argsFile)

	code, stdout, err := run(t, []string{"--model", "opus"})

	if err != nil || code != 0 {
		t.Fatalf("Run() = %d, %v", code, err)
	}
	if !strings.Contains(stdout, "fake claude session") {
		t.Errorf("stdout = %q", stdout)
	}
	data, _ := os.ReadFile(argsFile)
	args := strings.Split(strings.TrimSpace(string(data)), "\x1f")
	if args[0] != "--agents" || args[2] != "--settings" || args[len(args)-2] != "--model" || args[len(args)-1] != "opus" {
		t.Errorf("args = %q", args)
	}
}

func TestRunWritesTheSettingsFileForClaudeAndRemovesIt(t *testing.T) {
	argsFile := filepath.Join(t.TempDir(), "args")
	t.Setenv("FAKE_CLAUDE_ARGS_FILE", argsFile)

	if _, _, err := run(t, nil); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(argsFile)
	args := strings.Split(strings.TrimSpace(string(data)), "\x1f")
	settingsPath := args[3]
	if _, err := os.Stat(settingsPath); !os.IsNotExist(err) {
		t.Errorf("temporary settings file %s was left behind (%v)", settingsPath, err)
	}
	seen, err := os.ReadFile(argsFile + ".settings")
	if err != nil || !strings.Contains(string(seen), `"model"`) {
		t.Errorf("claude saw settings %q, %v", seen, err)
	}
}

func TestRunReturnsTheExitCodeOfClaude(t *testing.T) {
	t.Setenv("FAKE_CLAUDE_EXIT", "7")

	code, _, err := run(t, nil)

	if err != nil || code != 7 {
		t.Fatalf("Run() = %d, %v, want 7", code, err)
	}
}

func TestRunReportsAClaudeThatCannotStart(t *testing.T) {
	spec, err := runner.Build(bundle(nil), nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = runner.Run(filepath.Join(t.TempDir(), "missing-claude"), spec, nil, runner.Streams{Out: &bytes.Buffer{}, Err: &bytes.Buffer{}})

	if err == nil || !strings.Contains(err.Error(), "start claude") {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestPrepareBuildsTheCommandAndTheTemporarySettingsFile(t *testing.T) {
	spec, err := runner.Build(bundle(nil), map[string]string{"model": `"sonnet"`})
	if err != nil {
		t.Fatal(err)
	}

	cmd, cleanup, err := runner.Prepare("/usr/bin/claude", spec, []string{"--model", "opus"})

	if err != nil {
		t.Fatal(err)
	}
	if cmd.Path != "/usr/bin/claude" || cmd.Args[1] != "--agents" || cmd.Args[len(cmd.Args)-1] != "opus" {
		t.Errorf("Args = %q", cmd.Args)
	}
	settingsPath := cmd.Args[4]
	data, err := os.ReadFile(settingsPath)
	if err != nil || !strings.Contains(string(data), `"model": "sonnet"`) {
		t.Fatalf("settings file %s = %q, %v", settingsPath, data, err)
	}
	cleanup()
	if _, err := os.Stat(settingsPath); !os.IsNotExist(err) {
		t.Errorf("cleanup left %s behind (%v)", settingsPath, err)
	}
	cleanup() // safe to call again
}

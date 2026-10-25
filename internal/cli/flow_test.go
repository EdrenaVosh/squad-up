package cli_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestFlowShowsSetsAndRemovesTheLoop(t *testing.T) {
	e := newEnv(t)
	e.addPresets()

	if r := e.run("flow", "solo"); r.code != 0 || !strings.Contains(r.stdout, "no flow") {
		t.Fatalf("show: %d %q", r.code, r.stdout)
	}

	r := e.run("flow", "solo", "--gate", "reviewer", "--fix", "planner", "--rounds", "2")
	if r.code != 0 {
		t.Fatalf("set: %d %s", r.code, r.stderr)
	}
	for _, want := range []string{"planner → reviewer → tester", "reviewer checks, planner fixes", "2 rounds", "No main agent"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("stdout lacks %q:\n%s", want, r.stdout)
		}
	}
	yaml, _ := os.ReadFile(filepath.Join(e.home, "presets", "solo", "preset.yaml"))
	if !strings.Contains(string(yaml), "gate: reviewer") || !strings.Contains(string(yaml), "rounds: 2") {
		t.Errorf("preset.yaml:\n%s", yaml)
	}

	if r := e.run("flow", "solo", "--rounds", "4"); r.code != 0 || !strings.Contains(r.stdout, "reviewer checks") || !strings.Contains(r.stdout, "4 rounds") {
		t.Errorf("--rounds keeps the loop: %d %q", r.code, r.stdout)
	}
	if r := e.run("flow", "solo", "--off"); r.code != 0 || !strings.Contains(r.stdout, "no flow") {
		t.Errorf("--off: %d %q", r.code, r.stdout)
	}
	yaml, _ = os.ReadFile(filepath.Join(e.home, "presets", "solo", "preset.yaml"))
	if strings.Contains(string(yaml), "flow:") {
		t.Errorf("the flow is still stored:\n%s", yaml)
	}
}

func TestFlowRefusesWhatDoesNotAddUp(t *testing.T) {
	e := newEnv(t)
	e.addPresets()

	for name, args := range map[string][]string{
		"unknown agent": {"flow", "solo", "--gate", "ghost", "--fix", "planner"},
		"half a loop":   {"flow", "solo", "--gate", "reviewer"},
		"same agent":    {"flow", "solo", "--gate", "tester", "--fix", "tester"},
		"too many":      {"flow", "solo", "--gate", "tester", "--fix", "planner", "--rounds", "99"},
		"off and gate":  {"flow", "solo", "--off", "--gate", "tester"},
		"no preset":     {"flow", "nope", "--order"},
		"initial":       {"flow", "initial", "--order"},
	} {
		if r := e.run(args...); r.code == 0 {
			t.Errorf("%s: accepted: %q", name, r.stdout)
		}
	}
	yaml, _ := os.ReadFile(filepath.Join(e.home, "presets", "solo", "preset.yaml"))
	if strings.Contains(string(yaml), "flow:") {
		t.Errorf("a refused flow was stored:\n%s", yaml)
	}
}

func TestEditCanMakeAnAgentIndependent(t *testing.T) {
	e := newEnv(t)
	e.addPresets()

	if r := e.run("edit", "solo", "reviewer", "--independent"); r.code != 0 || !strings.Contains(r.stdout, "independent") {
		t.Fatalf("independent: %d %s %s", r.code, r.stdout, r.stderr)
	}
	md, _ := os.ReadFile(filepath.Join(e.home, "presets", "solo", "agents", "reviewer.md"))
	if !strings.Contains(string(md), "squadIndependent: true") {
		t.Errorf("reviewer.md:\n%s", md)
	}

	if r := e.run("edit", "solo", "reviewer", "--independent=false"); r.code != 0 || !strings.Contains(r.stdout, "ordinary") {
		t.Fatalf("undo: %d %s", r.code, r.stderr)
	}
	md, _ = os.ReadFile(filepath.Join(e.home, "presets", "solo", "agents", "reviewer.md"))
	if strings.Contains(string(md), "squadIndependent") {
		t.Errorf("the default must not be written:\n%s", md)
	}

	if r := e.run("edit", "solo", "--independent"); r.code != 2 {
		t.Errorf("no agent: %d", r.code)
	}
	if r := e.run("edit", "solo", "ghost", "--independent"); r.code == 0 {
		t.Error("an unknown agent was accepted")
	}
}

// transcript writes a session of the project in dir, in the shape Claude Code
// writes it: the main session, and a subagent run with its meta file.
func (e *env) transcript(dir string) {
	e.t.Helper()
	if real, err := filepath.EvalSymlinks(dir); err == nil {
		dir = real // Claude Code names the folder after the real path
	}
	project := filepath.Join(e.claudeDir, "projects", regexp.MustCompile(`[^a-zA-Z0-9]`).ReplaceAllString(dir, "-"))
	sub := filepath.Join(project, "s1", "subagents")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		e.t.Fatal(err)
	}
	msg := func(id string, in, write, read, out int) string {
		return fmt.Sprintf(`{"type":"assistant","timestamp":"2026-10-24T09:30:00Z","message":{"id":%q,"model":"claude-sonnet-5-5","usage":{"input_tokens":%d,"cache_creation_input_tokens":%d,"cache_read_input_tokens":%d,"output_tokens":%d}}}`+"\n", id, in, write, read, out)
	}
	files := map[string]string{
		filepath.Join(project, "s1.jsonl"):       msg("m1", 3, 17000, 9000, 400) + msg("m1", 3, 17000, 9000, 400),
		filepath.Join(sub, "agent-a1.jsonl"):     msg("r1", 2, 20872, 0, 55),
		filepath.Join(sub, "agent-a1.meta.json"): `{"agentType":"reviewer","finished":true}`,
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			e.t.Fatal(err)
		}
	}
}

func TestUsagePrintsWhatEachAgentSpent(t *testing.T) {
	e := newEnv(t)
	dir := t.TempDir()
	t.Chdir(dir)
	if real, err := os.Getwd(); err == nil {
		dir = real // the working folder as the system spells it
	}
	e.transcript(dir)

	r := e.run("usage")

	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	for _, want := range []string{"AGENT", "SENT", "CACHED", "TOTAL", "main", "reviewer", "SQUAD", "17k", "20k"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("stdout lacks %q:\n%s", want, r.stdout)
		}
	}
}

func TestUsageJSONCountsAMessageOnce(t *testing.T) {
	e := newEnv(t)
	dir := t.TempDir()
	t.Chdir(dir)
	if real, err := os.Getwd(); err == nil {
		dir = real
	}
	e.transcript(dir)

	r := e.run("usage", "--json")

	var got struct {
		Found  bool `json:"found"`
		Agents []struct {
			Name  string `json:"name"`
			Runs  int    `json:"runs"`
			Total int    `json:"total"`
		} `json:"agents"`
		Total struct {
			Total, Messages int
			CacheRead       int `json:"cacheRead"`
		} `json:"total"`
	}
	if err := json.Unmarshal([]byte(r.stdout), &got); err != nil {
		t.Fatalf("%v\n%s", err, r.stdout)
	}
	if !got.Found || len(got.Agents) != 2 || got.Agents[0].Name != "main" || got.Agents[0].Total != 3+17000+400 {
		t.Errorf("agents = %+v", got.Agents)
	}
	if got.Agents[1].Name != "reviewer" || got.Agents[1].Runs != 1 || got.Total.Total != 3+17000+400+2+20872+55 || got.Total.CacheRead != 9000 {
		t.Errorf("reviewer %+v, total %+v", got.Agents[1], got.Total)
	}
}

func TestUsageWithoutASessionIsNotAnError(t *testing.T) {
	e := newEnv(t)
	t.Chdir(t.TempDir())

	if r := e.run("usage"); r.code != 0 || !strings.Contains(r.stdout, "No Claude Code session") {
		t.Errorf("text: %d %q", r.code, r.stdout)
	}
	if r := e.run("usage", "--json"); r.code != 0 || !strings.Contains(r.stdout, `"found": false`) {
		t.Errorf("json: %d %q", r.code, r.stdout)
	}
	if r := e.run("usage", "--watch"); r.code == 0 {
		t.Error("--watch worked without a terminal")
	}
}

func TestExportSaysTheWholePathOfTheFile(t *testing.T) {
	e := newEnv(t)
	e.addPresets()
	dir := t.TempDir()
	t.Chdir(dir)

	r := e.run("export", "solo")

	if r.code != 0 || !strings.Contains(r.stdout, filepath.Join(dir, "solo.squad.json")) && !strings.Contains(r.stdout, "solo.squad.json") {
		t.Fatalf("%d %q %q", r.code, r.stdout, r.stderr)
	}
	if !strings.Contains(r.stdout, string(filepath.Separator)) || !filepath.IsAbs(strings.TrimSuffix(strings.TrimSpace(r.stdout[strings.LastIndex(r.stdout, " to ")+4:]), ".")) {
		t.Errorf("the path is not whole: %q", r.stdout)
	}
}

func TestADryRunShowsTheSquadBrief(t *testing.T) {
	e := newEnv(t)
	e.addPresets()
	if r := e.run("flow", "solo", "--gate", "reviewer", "--fix", "planner"); r.code != 0 {
		t.Fatal(r.stderr)
	}
	if r := e.run("edit", "solo", "reviewer", "--independent"); r.code != 0 {
		t.Fatal(r.stderr)
	}

	r := e.run("run", "solo", "--dry-run")

	for _, want := range []string{"--append-system-prompt", "Squad brief", "Independent: `reviewer`", "1 planner → 2 reviewer", "You are an independent agent"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("stdout lacks %q:\n%s", want, r.stdout)
		}
	}
}

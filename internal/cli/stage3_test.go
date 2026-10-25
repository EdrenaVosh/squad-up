package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/EdrenaVosh/squad-up/internal/testutil"
)

// ---- models ----

func TestModelsWithoutAProbeListsTheCatalogAsUnknown(t *testing.T) {
	e := ready(t)
	testutil.FakeClaude(t)

	r := e.run("models")

	if r.code != 0 {
		t.Fatalf("exit code = %d\n%s", r.code, r.stderr)
	}
	for _, want := range []string{"MODEL", "STATUS", "fable", "opusplan", "unknown", "squad models --probe"} {
		if !strings.Contains(r.stdout+r.stderr, want) {
			t.Errorf("output lacks %q:\n%s%s", want, r.stdout, r.stderr)
		}
	}
	if _, err := os.Stat(filepath.Join(e.home, "cache", "models.json")); !os.IsNotExist(err) {
		t.Error("a plain `models` must not probe or cache anything")
	}
}

func TestModelsProbeFindsTheResolvedModelsAndCachesThem(t *testing.T) {
	e := ready(t)
	argsFile := filepath.Join(t.TempDir(), "args")
	testutil.FakeClaude(t)
	t.Setenv("FAKE_CLAUDE_ARGS_FILE", argsFile)

	r := e.run("models", "--probe")

	if r.code != 0 {
		t.Fatalf("exit code = %d\n%s", r.code, r.stderr)
	}
	for _, want := range []string{"verified", "claude-opus-5-5", "claude-haiku-5-5"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("stdout lacks %q:\n%s", want, r.stdout)
		}
	}
	data, _ := os.ReadFile(argsFile)
	if got := strings.Count(string(data), "Reply with OK"); got != 4 {
		t.Errorf("%d probe requests were sent, want one per alias (4)", got)
	}

	again := e.run("models")
	if !strings.Contains(again.stdout, "claude-sonnet-5-5") || !strings.Contains(again.stdout, "Probed") {
		t.Errorf("a later `models` should read the cache:\n%s", again.stdout)
	}
	after, _ := os.ReadFile(argsFile)
	if strings.Count(string(after), "Reply with OK") != 4 {
		t.Error("reading the cache must not probe again")
	}
}

func TestModelsProbeShowsWhyAModelIsUnavailable(t *testing.T) {
	e := ready(t)
	testutil.FakeClaude(t)
	t.Setenv("FAKE_CLAUDE_UNAVAILABLE", "fable")

	r := e.run("models", "--probe")

	line := lineWith(r.stdout, "fable ")
	if !strings.Contains(line, "unavailable") || !strings.Contains(line, "issue with the selected model") {
		t.Errorf("fable row = %q", line)
	}
}

func lineWith(text, sub string) string {
	for _, l := range strings.Split(text, "\n") {
		if strings.Contains(l, sub) {
			return l
		}
	}
	return ""
}

func TestModelsAppliesTheUsersAllowlist(t *testing.T) {
	e := ready(t)
	testutil.FakeClaude(t)
	testutil.WriteFiles(t, e.claudeDir, map[string]string{"settings.json": `{"availableModels": ["sonnet", "haiku"]}`})

	r := e.run("models")

	if row := lineWith(r.stdout, "opus "); !strings.Contains(row, "unavailable") || !strings.Contains(row, "availableModels") {
		t.Errorf("opus row = %q", row)
	}
	if row := lineWith(r.stdout, "sonnet "); !strings.Contains(row, "allowed by policy") {
		t.Errorf("sonnet row = %q", row)
	}
}

func TestModelsJSON(t *testing.T) {
	e := ready(t)
	testutil.FakeClaude(t)
	e.run("models", "--probe")

	r := e.run("models", "--json")

	var got []struct {
		Name       string   `json:"name"`
		UsedBy     []string `json:"usedBy"`
		Status     string   `json:"status"`
		ResolvedTo string   `json:"resolvedTo"`
	}
	if err := json.Unmarshal([]byte(r.stdout), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, r.stdout)
	}
	byName := map[string]int{}
	for i, m := range got {
		byName[m.Name] = i
	}
	opus := got[byName["opus"]]
	if opus.Status != "verified" || opus.ResolvedTo != "claude-opus-5-5" || !reflect.DeepEqual(opus.UsedBy, []string{"agents", "settings"}) {
		t.Errorf("opus = %+v", opus)
	}
	if inherit := got[byName["inherit"]]; !reflect.DeepEqual(inherit.UsedBy, []string{"agents"}) {
		t.Errorf("inherit = %+v", inherit)
	}
}

func TestModelsWithoutClaudeFallsBackToTheCatalog(t *testing.T) {
	e := ready(t)
	testutil.EmptyPath(t)

	plain := e.run("models")
	if plain.code != 0 || !strings.Contains(plain.stdout, "unknown") || !strings.Contains(plain.stderr, "claude was not found") {
		t.Fatalf("plain: exit code = %d\nstdout:\n%s\nstderr:\n%s", plain.code, plain.stdout, plain.stderr)
	}
	probe := e.run("models", "--probe")
	if probe.code != 1 || !strings.Contains(probe.stderr, "Fix:") {
		t.Fatalf("probe: exit code = %d\n%s", probe.code, probe.stderr)
	}
}

func TestModelsProbeNeedsALogin(t *testing.T) {
	e := ready(t)
	testutil.FakeClaude(t)
	t.Setenv("FAKE_CLAUDE_LOGGED_IN", "0")

	r := e.run("models", "--probe")

	if r.code != 1 || !strings.Contains(r.stderr, "claude auth login") {
		t.Fatalf("exit code = %d\n%s", r.code, r.stderr)
	}
}

func TestAProbeIsForgottenWhenClaudeIsUpdated(t *testing.T) {
	e := ready(t)
	testutil.FakeClaude(t)
	e.run("models", "--probe")
	t.Setenv("FAKE_CLAUDE_VERSION", "9.9.9 (Claude Code)")

	r := e.run("models")

	if strings.Contains(r.stdout, "Probed") || strings.Contains(r.stdout, "verified   claude-") || !strings.Contains(r.stderr, "out of date") {
		t.Errorf("stdout:\n%s\nstderr:\n%s", r.stdout, r.stderr)
	}
}

func TestModelsWarnsAboutAnthropicModel(t *testing.T) {
	e := ready(t)
	testutil.FakeClaude(t)
	t.Setenv("ANTHROPIC_MODEL", "opus")

	r := e.run("models")

	if !strings.Contains(r.stderr, "ANTHROPIC_MODEL is set (opus)") {
		t.Errorf("stderr:\n%s", r.stderr)
	}
}

func TestStatusSummarisesTheProbedModels(t *testing.T) {
	e := ready(t)
	testutil.FakeClaude(t)
	t.Setenv("FAKE_CLAUDE_UNAVAILABLE", "fable")

	before := e.run("status")
	if !strings.Contains(before.stdout, "not probed — run squad models --probe") {
		t.Errorf("status before a probe:\n%s", before.stdout)
	}
	e.run("models", "--probe")
	after := e.run("status")
	if !strings.Contains(after.stdout, "verified: haiku, opus, sonnet") || !strings.Contains(after.stdout, "unavailable: fable") {
		t.Errorf("status after a probe:\n%s", after.stdout)
	}

	var st struct {
		Models struct {
			Probed      bool     `json:"probed"`
			Verified    []string `json:"verified"`
			Unavailable []string `json:"unavailable"`
		} `json:"models"`
	}
	if err := json.Unmarshal([]byte(e.run("status", "--json").stdout), &st); err != nil {
		t.Fatal(err)
	}
	if !st.Models.Probed || !reflect.DeepEqual(st.Models.Verified, []string{"haiku", "opus", "sonnet"}) || !reflect.DeepEqual(st.Models.Unavailable, []string{"fable"}) {
		t.Errorf("models = %+v", st.Models)
	}
}

func TestUseWarnsAboutModelsTheAccountCannotUse(t *testing.T) {
	e := ready(t)
	testutil.FakeClaude(t)
	testutil.WriteFiles(t, e.claudeDir, map[string]string{"settings.json": `{"availableModels": ["sonnet"]}`})
	writeAgentWithModel(t, e, "model-user", "opus")

	settingsModel := e.run("use", "bug-hunt", "--dry-run") // bug-hunt sets model haiku
	if !strings.Contains(settingsModel.stdout, `default model "haiku" is unavailable`) {
		t.Errorf("stdout:\n%s", settingsModel.stdout)
	}
	agentModel := e.run("use", "model-user", "--dry-run")
	if !strings.Contains(agentModel.stdout, `agent "model-user" uses the model "opus"`) || !strings.Contains(agentModel.stdout, "model: inherit") {
		t.Errorf("stdout:\n%s", agentModel.stdout)
	}
	if clean := e.run("use", "solo", "--dry-run"); strings.Contains(clean.stdout, "unavailable") {
		t.Errorf("sonnet is allowed, nothing to warn about:\n%s", clean.stdout)
	}
}

// writeAgentWithModel stores a one-agent preset whose agent uses the model.
func writeAgentWithModel(t *testing.T, e *env, name, model string) {
	t.Helper()
	dir := filepath.Join(e.home, "presets", name)
	testutil.WriteFiles(t, dir, map[string]string{
		"preset.yaml":            "schema: 1\nname: " + name + "\nmode: extend\nmain: null\nagents: [" + name + "]\n",
		"agents/" + name + ".md": "---\nname: " + name + "\ndescription: Uses a model. Use when testing model checks.\ntools: Read\nmodel: " + model + "\n---\nBody\n",
	})
}

// ---- run ----

func runEnv(t *testing.T) (*env, string) {
	t.Helper()
	e := ready(t)
	testutil.FakeClaude(t)
	argsFile := filepath.Join(t.TempDir(), "args")
	t.Setenv("FAKE_CLAUDE_ARGS_FILE", argsFile)
	return e, argsFile
}

func recordedArgs(t *testing.T, argsFile string) []string {
	t.Helper()
	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("claude was not started: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	return strings.Split(lines[len(lines)-1], "\x1f")
}

func TestRunStartsClaudeWithThePresetAndWritesNothing(t *testing.T) {
	e, argsFile := runEnv(t)
	claudeBefore, homeBefore := e.tree(), testutil.Tree(t, e.home)

	r := e.run("run", "solo")

	if r.code != 0 {
		t.Fatalf("exit code = %d\n%s", r.code, r.stderr)
	}
	if !strings.Contains(r.stdout, "fake claude session") {
		t.Errorf("stdout = %q", r.stdout)
	}
	if !strings.Contains(r.stderr, `Running claude with "solo"`) || !strings.Contains(r.stderr, "+ your global agents") {
		t.Errorf("stderr lacks the run notice:\n%s", r.stderr)
	}
	args := recordedArgs(t, argsFile)
	if args[0] != "--agents" || args[2] != "--settings" {
		t.Fatalf("args = %q", args)
	}
	var agents map[string]map[string]any
	if err := json.Unmarshal([]byte(args[1]), &agents); err != nil {
		t.Fatalf("--agents is not JSON: %v", err)
	}
	for _, name := range []string{"planner", "reviewer", "tester"} {
		if agents[name]["description"] == nil || agents[name]["prompt"] == nil {
			t.Errorf("agent %s = %v", name, agents[name])
		}
	}
	seen, _ := os.ReadFile(argsFile + ".settings")
	if !strings.Contains(string(seen), `"model": "sonnet"`) {
		t.Errorf("claude saw the settings %q", seen)
	}
	if !reflect.DeepEqual(e.tree(), claudeBefore) {
		t.Error("run changed the Claude config")
	}
	if got := testutil.Tree(t, e.home); !reflect.DeepEqual(got, homeBefore) {
		t.Errorf("run changed ~/.squad-up: state, backups or journal must not appear")
	}
}

func TestRunPassesTheArgumentsAfterTheDashesToClaude(t *testing.T) {
	e, argsFile := runEnv(t)

	r := e.run("run", "bug-hunt", "--", "--model", "opus", "fix the failing test")

	if r.code != 0 {
		t.Fatalf("exit code = %d\n%s", r.code, r.stderr)
	}
	args := recordedArgs(t, argsFile)
	tail := args[len(args)-3:]
	if !reflect.DeepEqual(tail, []string{"--model", "opus", "fix the failing test"}) {
		t.Errorf("args = %q", args)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--agent debugger") {
		t.Errorf("the preset's main agent was not passed: %q", args)
	}
}

func TestRunReturnsTheExitCodeOfClaude(t *testing.T) {
	e, _ := runEnv(t)
	t.Setenv("FAKE_CLAUDE_EXIT", "5")

	r := e.run("run", "solo")

	if r.code != 5 {
		t.Fatalf("exit code = %d, want 5", r.code)
	}
	if strings.Contains(r.stderr, "Error:") {
		t.Errorf("a non-zero exit of claude is not an error of squad:\n%s", r.stderr)
	}
}

func TestRunNeedsAPresetName(t *testing.T) {
	e, _ := runEnv(t)

	r := e.run("run")

	if r.code != 1 || !strings.Contains(r.stderr, "squad run <preset>") {
		t.Fatalf("exit code = %d\n%s", r.code, r.stderr)
	}
}

func TestRunWithAnUnknownPreset(t *testing.T) {
	e, _ := runEnv(t)

	r := e.run("run", "nope")

	if r.code != 1 || !strings.Contains(r.stderr, "squad list") {
		t.Fatalf("exit code = %d\n%s", r.code, r.stderr)
	}
}

func TestRunRefusesInitial(t *testing.T) {
	e, _ := runEnv(t)

	r := e.run("run", "initial")

	if r.code != 1 || !strings.Contains(r.stderr, "run claude directly") {
		t.Fatalf("exit code = %d\n%s", r.code, r.stderr)
	}
}

func TestRunWithoutClaude(t *testing.T) {
	e := ready(t)
	testutil.EmptyPath(t)

	r := e.run("run", "solo")

	if r.code != 1 || !strings.Contains(r.stderr, "install Claude Code") {
		t.Fatalf("exit code = %d\n%s", r.code, r.stderr)
	}
}

func TestRunDryRunShowsWhatWouldStart(t *testing.T) {
	e, argsFile := runEnv(t)

	r := e.run("run", "solo", "--dry-run", "--", "--model", "opus")

	if r.code != 0 {
		t.Fatalf("exit code = %d\n%s", r.code, r.stderr)
	}
	for _, want := range []string{"claude --agents", "--settings", "--model opus", `"planner"`, `"model": "sonnet"`} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("stdout lacks %q:\n%s", want, r.stdout)
		}
	}
	if _, err := os.Stat(argsFile); !os.IsNotExist(err) {
		t.Error("--dry-run must not start claude")
	}
}

func TestRunWarnsAboutAnthropicModelAndUnavailableModels(t *testing.T) {
	e, _ := runEnv(t)
	t.Setenv("ANTHROPIC_MODEL", "opus")
	testutil.WriteFiles(t, e.claudeDir, map[string]string{"settings.json": `{"availableModels": ["sonnet"]}`})

	r := e.run("run", "bug-hunt")

	if !strings.Contains(r.stderr, "ANTHROPIC_MODEL is set (opus)") || !strings.Contains(r.stderr, `default model "haiku" is unavailable`) {
		t.Errorf("stderr:\n%s", r.stderr)
	}
}

// ---- lint ----

func writePreset(t *testing.T, e *env, name, presetYAML string, agents map[string]string) {
	t.Helper()
	files := map[string]string{"preset.yaml": presetYAML}
	for file, content := range agents {
		files["agents/"+file] = content
	}
	testutil.WriteFiles(t, filepath.Join(e.home, "presets", name), files)
}

const goodReviewer = "---\nname: reviewer\ndescription: Reviews diffs for bugs and risky changes. Use after any code change.\ntools: Read, Grep\n---\nYou review code.\n"

func TestLintOfACleanPreset(t *testing.T) {
	e := ready(t)
	writePreset(t, e, "clean", "schema: 1\nname: clean\nmode: replace\nmain: null\nagents: [reviewer]\n", map[string]string{"reviewer.md": goodReviewer})

	r := e.run("lint", "clean")

	if r.code != 0 {
		t.Fatalf("exit code = %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	for _, want := range []string{"clean", "no problems", "tokens per session"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("stdout lacks %q:\n%s", want, r.stdout)
		}
	}
}

func TestLintFailsWithExitCode2OnErrors(t *testing.T) {
	e := ready(t)
	bad := "---\nname: reviewer\ndescription: Reviews diffs for bugs and risky changes. Use after any code change.\ntools: Read\nmodel: gpt-5\ndisallowed_tools: Bash\n---\nBody\n"
	writePreset(t, e, "bad", "schema: 1\nname: bad\nmode: replace\nmain: null\nagents: [reviewer]\n", map[string]string{"reviewer.md": bad})

	r := e.run("lint", "bad")

	if r.code != 2 {
		t.Fatalf("exit code = %d, want 2\n%s%s", r.code, r.stdout, r.stderr)
	}
	for _, want := range []string{"error", "bad-model", "warn", "unknown-field", `did you mean "disallowedTools"`, "reviewer.md"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("stdout lacks %q:\n%s", want, r.stdout)
		}
	}
}

func TestLintStrictTreatsWarningsAsFailures(t *testing.T) {
	e := ready(t)
	warn := "---\nname: reviewer\ndescription: Short one. Use when needed.\ntools: Read\n---\nBody\n"
	writePreset(t, e, "warny", "schema: 1\nname: warny\nmode: replace\nmain: null\nagents: [reviewer]\n", map[string]string{"reviewer.md": warn})

	if r := e.run("lint", "warny"); r.code != 0 {
		t.Fatalf("warnings alone must pass: exit code = %d\n%s", r.code, r.stdout)
	}
	if r := e.run("lint", "warny", "--strict"); r.code != 2 {
		t.Fatalf("--strict: exit code = %d, want 2", r.code)
	}
}

func TestLintReportsAMissingAgentFileAndABrokenPresetFile(t *testing.T) {
	e := ready(t)
	writePreset(t, e, "gappy", "schema: 1\nname: gappy\nmode: replace\nmain: null\nagents: [reviewer, ghost]\n", map[string]string{"reviewer.md": goodReviewer})
	writePreset(t, e, "garbled", "schema: [oops\n", nil)

	gappy := e.run("lint", "gappy")
	if gappy.code != 2 || !strings.Contains(gappy.stdout, "missing-agent-file") || !strings.Contains(gappy.stdout, "ghost") {
		t.Errorf("gappy: exit code = %d\n%s", gappy.code, gappy.stdout)
	}
	garbled := e.run("lint", "garbled")
	if garbled.code != 2 || !strings.Contains(garbled.stdout, "invalid-preset") {
		t.Errorf("garbled: exit code = %d\n%s", garbled.code, garbled.stdout)
	}
}

func TestLintInitialChecksTheOriginalAgents(t *testing.T) {
	e := newEnv(t)
	testutil.WriteFiles(t, e.claudeDir, map[string]string{
		"agents/good.md":   goodReviewer,
		"agents/broken.md": "no frontmatter",
	})
	e.run("list")

	r := e.run("lint", "initial")

	if r.code != 2 || !strings.Contains(r.stdout, "broken.md") || !strings.Contains(r.stdout, "unreadable") {
		t.Fatalf("exit code = %d\n%s", r.code, r.stdout)
	}
}

func TestLintWithoutAnArgumentChecksEverything(t *testing.T) {
	e := ready(t)

	r := e.run("lint")

	for _, want := range []string{"bug-hunt", "solo", "initial"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("stdout lacks %q:\n%s", want, r.stdout)
		}
	}
}

func TestLintUnknownPreset(t *testing.T) {
	e := ready(t)

	r := e.run("lint", "nope")

	if r.code != 1 || !strings.Contains(r.stderr, "squad list") {
		t.Fatalf("exit code = %d\n%s", r.code, r.stderr)
	}
}

func TestLintJSON(t *testing.T) {
	e := ready(t)
	writePreset(t, e, "bad", "schema: 1\nname: bad\nmode: replace\nmain: null\nagents: [reviewer]\n",
		map[string]string{"reviewer.md": strings.Replace(goodReviewer, "tools: Read, Grep\n", "tools: Read\nmodel: gpt-5\n", 1)})

	r := e.run("lint", "bad", "--json")

	var got struct {
		Targets []struct {
			Name     string `json:"name"`
			Findings []struct {
				Level string `json:"level"`
				Rule  string `json:"rule"`
			} `json:"findings"`
			Cost struct {
				SessionTokens int `json:"sessionTokens"`
			} `json:"cost"`
		} `json:"targets"`
		Summary struct {
			Errors, Warnings, Infos int
		} `json:"summary"`
	}
	if err := json.Unmarshal([]byte(r.stdout), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, r.stdout)
	}
	if r.code != 2 || len(got.Targets) != 1 || got.Targets[0].Name != "bad" || got.Summary.Errors != 1 || got.Targets[0].Cost.SessionTokens == 0 {
		t.Errorf("exit code %d, report %+v", r.code, got)
	}
}

func TestModelsCanProbeEveryReleaseByItsFullID(t *testing.T) {
	e := ready(t)
	testutil.FakeClaude(t)

	r := e.run("models", "--releases")

	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	for _, want := range []string{"claude-opus-4-6", "claude-sonnet-4-5", "claude-haiku-5-5"} {
		line := ""
		for _, l := range strings.Split(r.stdout, "\n") {
			if strings.HasPrefix(l, want+" ") {
				line = l
			}
		}
		if !strings.Contains(line, "verified") {
			t.Errorf("%s: %q", want, line)
		}
	}
	if !strings.Contains(r.stderr, "older releases") {
		t.Errorf("stderr: %s", r.stderr)
	}

	again := e.run("models", "--probe") // an aliases-only probe keeps what is known of the releases
	if line := lineWith(again.stdout, "claude-opus-4-6 "); !strings.Contains(line, "verified") {
		t.Errorf("the answer for opus 4.6 was lost: %q", line)
	}
}

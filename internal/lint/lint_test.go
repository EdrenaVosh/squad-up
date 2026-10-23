package lint_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/EdrenaVosh/squadup/internal/lint"
	"github.com/EdrenaVosh/squadup/internal/preset"
)

// agentFile builds an agent file with the given extra frontmatter lines.
func agentFile(name, description, extra, body string) lint.Agent {
	text := "---\n"
	if name != "" {
		text += "name: " + name + "\n"
	}
	if description != "" {
		text += "description: " + description + "\n"
	}
	text += extra + "---\n" + body
	return lint.Agent{File: name + ".md", Data: []byte(text)}
}

const goodDesc = "Reviews diffs for bugs and risky changes. Use after any code change."

func clean(name string) lint.Agent {
	return agentFile(name, goodDesc+" ("+name+")", "tools: Read, Grep\n", "You review code.\n")
}

func run(agents ...lint.Agent) lint.Report {
	return lint.Run(lint.Input{Name: "t", Agents: agents})
}

func find(r lint.Report, rule string) []lint.Finding {
	var out []lint.Finding
	for _, f := range r.Findings {
		if f.Rule == rule {
			out = append(out, f)
		}
	}
	return out
}

func expect(t *testing.T, r lint.Report, rule string, level lint.Level, contains ...string) {
	t.Helper()
	got := find(r, rule)
	if len(got) == 0 {
		t.Fatalf("no %q finding in %+v", rule, r.Findings)
	}
	if got[0].Level != level {
		t.Errorf("%s level = %s, want %s", rule, got[0].Level, level)
	}
	for _, sub := range contains {
		if !strings.Contains(got[0].Message+got[0].Fix, sub) {
			t.Errorf("%s message %q lacks %q", rule, got[0].Message, sub)
		}
	}
}

func TestACleanAgentHasNoFindings(t *testing.T) {
	r := run(clean("reviewer"))

	if len(r.Findings) != 0 {
		t.Fatalf("findings = %+v", r.Findings)
	}
}

func TestUnknownFieldsAreWarnedAboutWithASuggestion(t *testing.T) {
	r := run(agentFile("reviewer", goodDesc, "disallowed_tools: Bash\nModel: opus\nzzzzzz: 1\ntools: Read\n", "Body\n"))

	var messages []string
	for _, f := range find(r, "unknown-field") {
		if f.Level != lint.LevelWarn {
			t.Errorf("unknown-field level = %s", f.Level)
		}
		messages = append(messages, f.Message)
	}
	joined := strings.Join(messages, "\n")
	for _, want := range []string{`"disallowed_tools"`, `"disallowedTools"`, `"Model"`, `"model"`, `"zzzzzz"`} {
		if !strings.Contains(joined, want) {
			t.Errorf("messages lack %s:\n%s", want, joined)
		}
	}
	if strings.Count(joined, "did you mean") != 2 {
		t.Errorf("only the two near-misses get a suggestion:\n%s", joined)
	}
}

func TestDocumentedFieldsAreNotUnknown(t *testing.T) {
	extra := "tools: Read\ndisallowedTools: Bash\nmodel: sonnet\npermissionMode: plan\nmaxTurns: 5\nskills: [a]\nmemory: user\nbackground: true\neffort: high\nisolation: worktree\ncolor: cyan\ninitialPrompt: hi\nomitClaudeMd: true\nautoCompactWindow: 200000\nexperimental:\n  cacheTtl: 5m\n"

	r := run(agentFile("reviewer", goodDesc, extra, "Body\n"))

	if len(r.Findings) != 0 {
		t.Fatalf("findings = %+v", r.Findings)
	}
}

func TestNameProblems(t *testing.T) {
	expect(t, run(agentFile("Bad_Name", goodDesc, "tools: Read\n", "b\n")), "invalid-name", lint.LevelError, "Bad_Name")
	expect(t, run(lint.Agent{File: "x.md", Data: []byte("---\ndescription: " + goodDesc + "\ntools: Read\n---\nb\n")}), "missing-name", lint.LevelError)
	expect(t, run(clean("a"), clean("a")), "duplicate-name", lint.LevelError, `"a"`)
}

func TestDescriptionProblems(t *testing.T) {
	expect(t, run(agentFile("a", "Short. Use when asked.", "tools: Read\n", "b\n")), "short-description", lint.LevelWarn)
	expect(t, run(agentFile("a", "Handles all kinds of review work for the whole team.", "tools: Read\n", "b\n")), "no-trigger", lint.LevelWarn, "when")
	expect(t, run(lint.Agent{File: "a.md", Data: []byte("---\nname: a\ntools: Read\n---\nb\n")}), "missing-description", lint.LevelError)
	if got := run(agentFile("a", "Handles all kinds of review work. Call it BEFORE you commit.", "tools: Read\n", "b\n")); len(find(got, "no-trigger")) != 0 {
		t.Errorf("a trigger word in any case satisfies the rule: %+v", got.Findings)
	}
}

func TestSimilarDescriptionsAreFlagged(t *testing.T) {
	a := agentFile("code-reviewer", "Reviews diffs for bugs and risky changes. Use after any code change.", "tools: Read\n", "b\n")
	b := agentFile("diff-reviewer", "Reviews diffs for bugs and risky changes. Use after any code change!", "tools: Read\n", "b\n")
	c := agentFile("tester", "Writes and runs tests for a change. Use when coverage is missing.", "tools: Read\n", "b\n")

	r := run(a, b, c)

	got := find(r, "similar-description")
	if len(got) != 1 || !strings.Contains(got[0].Message, "code-reviewer") || !strings.Contains(got[0].Message, "diff-reviewer") {
		t.Fatalf("similar-description = %+v", got)
	}
	if got[0].Level != lint.LevelWarn {
		t.Errorf("level = %s", got[0].Level)
	}
}

func TestAnEmptyPromptIsWarnedAbout(t *testing.T) {
	expect(t, run(agentFile("a", goodDesc, "tools: Read\n", "  \n\n")), "empty-prompt", lint.LevelWarn)
}

func TestAnAgentWithoutToolsIsAnInfo(t *testing.T) {
	expect(t, run(agentFile("a", goodDesc, "", "b\n")), "inherits-all-tools", lint.LevelInfo)
}

func TestInvalidFieldValues(t *testing.T) {
	cases := map[string]struct {
		extra string
		rule  string
		level lint.Level
	}{
		"model":             {"model: gpt-5\n", "bad-model", lint.LevelError},
		"effort":            {"effort: ultra\n", "bad-effort", lint.LevelError},
		"permission mode":   {"permissionMode: yolo\n", "bad-permission-mode", lint.LevelError},
		"memory":            {"memory: global\n", "bad-memory", lint.LevelError},
		"max turns text":    {"maxTurns: ten\n", "bad-max-turns", lint.LevelError},
		"max turns zero":    {"maxTurns: 0\n", "bad-max-turns", lint.LevelError},
		"isolation":         {"isolation: docker\n", "bad-isolation", lint.LevelError},
		"color":             {"color: magenta\n", "bad-color", lint.LevelWarn},
		"compact window":    {"autoCompactWindow: 5\n", "bad-auto-compact-window", lint.LevelError},
		"tools as a number": {"tools: 5\n", "bad-tools", lint.LevelError},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			base := "tools: Read\n"
			if strings.HasPrefix(c.extra, "tools:") {
				base = ""
			}
			expect(t, run(agentFile("a", goodDesc, base+c.extra, "b\n")), c.rule, c.level)
		})
	}
}

func TestWidePermissionModes(t *testing.T) {
	expect(t, run(agentFile("a", goodDesc, "tools: Read\npermissionMode: bypassPermissions\n", "b\n")), "wide-permissions", lint.LevelWarn)
	expect(t, run(agentFile("a", goodDesc, "tools: Read\npermissionMode: auto\n", "b\n")), "wide-permissions", lint.LevelInfo)
	if r := run(agentFile("a", goodDesc, "tools: Read\npermissionMode: acceptEdits\n", "b\n")); len(r.Findings) != 0 {
		t.Errorf("acceptEdits is fine: %+v", r.Findings)
	}
}

func TestAnUnreadableAgentIsAnErrorAndSkipsTheOtherRules(t *testing.T) {
	r := run(lint.Agent{File: "broken.md", Data: []byte("no frontmatter at all")}, clean("ok"))

	expect(t, r, "unreadable", lint.LevelError, "broken.md")
	if len(r.Findings) != 1 {
		t.Errorf("findings = %+v, want only the unreadable file", r.Findings)
	}
}

// ---- preset level ----

func presetOf(agents ...string) *preset.Preset {
	return &preset.Preset{Schema: 1, Name: "solo", Mode: preset.ModeReplace, Agents: agents}
}

func TestPresetValidationErrorsAreReported(t *testing.T) {
	p := presetOf("a")
	p.Name = "Bad Name"
	p.Settings.EffortLevel = "max"

	r := lint.Run(lint.Input{Name: "x", Preset: p, Agents: []lint.Agent{clean("a")}})

	got := find(r, "invalid-preset")
	if len(got) != 2 {
		t.Fatalf("invalid-preset = %+v, want one per problem", got)
	}
	for _, f := range got {
		if f.Level != lint.LevelError {
			t.Errorf("level = %s", f.Level)
		}
	}
}

func TestMainAgentThatIsNotInThePresetIsWarnedAbout(t *testing.T) {
	missing, present := "ghost", "a"
	p := presetOf("a")

	p.Main = &missing
	expect(t, lint.Run(lint.Input{Name: "x", Preset: p, Agents: []lint.Agent{clean("a")}}), "missing-main", lint.LevelWarn, "ghost")

	p.Main = &present
	if r := lint.Run(lint.Input{Name: "x", Preset: p, Agents: []lint.Agent{clean("a")}}); len(find(r, "missing-main")) != 0 {
		t.Errorf("a main agent from the preset is fine: %+v", r.Findings)
	}
}

func TestAListedAgentWithoutAFileIsAnError(t *testing.T) {
	r := lint.Run(lint.Input{Name: "x", Preset: presetOf("a", "ghost"), Agents: []lint.Agent{clean("a")}})

	expect(t, r, "missing-agent-file", lint.LevelError, "ghost")
}

func TestTheFileMustDefineTheAgentItIsListedAs(t *testing.T) {
	file := clean("somebody-else")
	file.File = "a.md"

	r := lint.Run(lint.Input{Name: "x", Preset: presetOf("a"), Agents: []lint.Agent{file}})

	expect(t, r, "name-mismatch", lint.LevelError, "somebody-else")
	initial := lint.Run(lint.Input{Name: "initial", Agents: []lint.Agent{file}})
	if len(find(initial, "name-mismatch")) != 0 {
		t.Error("Initial keeps files wherever the user put them; only the name counts")
	}
}

func TestSettingsRisks(t *testing.T) {
	p := presetOf("a")
	p.Settings.Permissions = &preset.Permissions{
		DefaultMode: "bypassPermissions",
		Allow:       []string{"Bash", "Bash(*)", "WebFetch", "Bash(npm run *)", "Read(./docs/**)"},
	}

	r := lint.Run(lint.Input{Name: "x", Preset: p, Agents: []lint.Agent{clean("a")}})

	expect(t, r, "wide-permissions", lint.LevelWarn, "bypassPermissions")
	broad := find(r, "broad-allow")
	if len(broad) != 3 {
		t.Fatalf("broad-allow = %+v, want Bash, Bash(*) and WebFetch", broad)
	}
	for _, f := range broad {
		if f.Level != lint.LevelWarn {
			t.Errorf("level = %s", f.Level)
		}
	}
}

func TestAPresetWithoutAgentsIsAnInfo(t *testing.T) {
	r := lint.Run(lint.Input{Name: "x", Preset: presetOf()})

	expect(t, r, "no-agents", lint.LevelInfo)
}

// ---- cost and report ----

func TestCostEstimatesTokensFromCharacters(t *testing.T) {
	a := lint.Agent{File: "ab.md", Data: []byte("---\nname: ab\ndescription: abcdef\ntools: Read\n---\n0123456789")}
	b := lint.Agent{File: "cd.md", Data: []byte("---\nname: cd\ndescription: abcdefghij\ntools: Read\n---\n")}

	r := run(a, b)

	if r.Cost.SessionTokens != 2+3 { // (2+6)/4 = 2, (2+10)/4 = 3
		t.Errorf("SessionTokens = %d, want 5", r.Cost.SessionTokens)
	}
	if len(r.Cost.Agents) != 2 || r.Cost.Agents[0].Name != "ab" || r.Cost.Agents[0].DescriptionTokens != 2 || r.Cost.Agents[0].PromptTokens != 3 {
		t.Errorf("Agents = %+v", r.Cost.Agents)
	}
}

func TestReportCountsByLevel(t *testing.T) {
	r := run(agentFile("Bad_Name", "short", "", "  "))

	errs, warns, infos := r.Count(lint.LevelError), r.Count(lint.LevelWarn), r.Count(lint.LevelInfo)
	if errs != 1 || warns < 2 || infos != 1 {
		t.Errorf("errors=%d warnings=%d infos=%d, findings: %s", errs, warns, infos, fmt.Sprint(r.Findings))
	}
}

func TestEveryFindingHasARuleAndAMessage(t *testing.T) {
	r := run(agentFile("Bad_Name", "short", "model: x\neffort: y\n", ""))

	for _, f := range r.Findings {
		if f.Rule == "" || f.Message == "" || f.Level == "" {
			t.Errorf("incomplete finding %+v", f)
		}
	}
}

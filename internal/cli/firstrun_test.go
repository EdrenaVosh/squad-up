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

func TestFirstRunOnExistingSetupCapturesInitialAndPrintsSummary(t *testing.T) {
	e := newEnv(t)
	e.writeSetup()
	before := testutil.Tree(t, e.claudeDir)

	r := e.run("list")

	if r.code != 0 {
		t.Fatalf("exit code = %d\nstderr:\n%s", r.code, r.stderr)
	}
	for _, want := range []string{
		"Welcome to squad-up",
		"We found your existing Claude Code setup",
		"saved it as ◇ Initial",
		"  Agents       3   docs-writer · reviewer · tester",
		"  Model        opus",
		"  Effort       high",
		"  Permissions  acceptEdits · 4 allow · 1 ask · 2 deny",
		"  Main agent   —",
		"Nothing was changed. Run `squad restore` any time to get this back.",
	} {
		if !strings.Contains(r.stderr, want) {
			t.Errorf("summary lacks %q:\n%s", want, r.stderr)
		}
	}
	if strings.Contains(r.stdout, "Welcome") {
		t.Errorf("the summary must not pollute stdout:\n%s", r.stdout)
	}
	if after := testutil.Tree(t, e.claudeDir); !reflect.DeepEqual(before, after) {
		t.Error("first run modified the Claude config directory")
	}
	if _, err := os.Stat(filepath.Join(e.home, "presets", "initial", "agents", "team", "tester.md")); err != nil {
		t.Errorf("Initial was not captured: %v", err)
	}
}

func TestFirstRunSummaryIsPrintedOnlyOnce(t *testing.T) {
	e := newEnv(t)
	e.writeSetup()
	e.run("list")

	r := e.run("list")

	if r.stderr != "" {
		t.Fatalf("second run printed to stderr:\n%s", r.stderr)
	}
}

func TestFirstRunOnCleanSetup(t *testing.T) {
	e := newEnv(t)

	r := e.run("list")

	if !strings.Contains(r.stderr, "Clean Claude Code setup — Initial is the defaults") {
		t.Errorf("stderr lacks the clean-setup text:\n%s", r.stderr)
	}
	if strings.Contains(r.stderr, "Agents ") {
		t.Errorf("a clean setup must not list agents:\n%s", r.stderr)
	}
	if !strings.Contains(r.stderr, "Nothing was changed.") {
		t.Errorf("stderr lacks the reassurance:\n%s", r.stderr)
	}
}

func TestFirstRunSummaryCountsAgentsWithIssues(t *testing.T) {
	e := newEnv(t)
	testutil.WriteFiles(t, e.claudeDir, map[string]string{
		"agents/good.md":   reviewerMD,
		"agents/broken.md": "---\nname: [oops\n---\n",
		"agents/other.md":  "no frontmatter",
	})

	r := e.run("list")

	if !strings.Contains(r.stderr, "2 agents have issues — run squad lint initial") {
		t.Errorf("stderr lacks the issue count:\n%s", r.stderr)
	}
}

func TestFirstRunSummarySingularIssue(t *testing.T) {
	e := newEnv(t)
	testutil.WriteFiles(t, e.claudeDir, map[string]string{"agents/broken.md": "no frontmatter"})

	r := e.run("list")

	if !strings.Contains(r.stderr, "1 agent has issues — run squad lint initial") {
		t.Errorf("stderr lacks the singular issue text:\n%s", r.stderr)
	}
}

func TestFirstRunSummaryMentionsInvalidSettings(t *testing.T) {
	e := newEnv(t)
	testutil.WriteFiles(t, e.claudeDir, map[string]string{"settings.json": `{"model": `})

	r := e.run("list")

	if r.code != 0 {
		t.Fatalf("exit code = %d: a broken settings.json must not block squad\n%s", r.code, r.stderr)
	}
	if !strings.Contains(r.stderr, "settings.json is not valid JSON") {
		t.Errorf("stderr lacks the settings warning:\n%s", r.stderr)
	}
}

func TestHelpAndVersionDoNotCaptureInitial(t *testing.T) {
	e := newEnv(t)
	e.writeSetup()

	for _, args := range [][]string{{"--help"}, {"--version"}, {"help"}, {"list", "--help"}, {"completion", "zsh"}} {
		r := e.run(args...)
		if r.code != 0 {
			t.Errorf("%v: exit code = %d\n%s", args, r.code, r.stderr)
		}
	}

	if _, err := os.Stat(e.home); !os.IsNotExist(err) {
		t.Fatalf("help/version/completion created %s (err %v)", e.home, err)
	}
}

func TestJSONOutputStaysCleanOnFirstRun(t *testing.T) {
	e := newEnv(t)
	e.writeSetup()

	r := e.run("list", "--json")

	if !json.Valid([]byte(r.stdout)) {
		t.Fatalf("stdout is not valid JSON:\n%s", r.stdout)
	}
	if !strings.Contains(r.stderr, "Welcome to squad-up") {
		t.Errorf("the summary should still reach stderr:\n%s", r.stderr)
	}
}

func TestFlagsOverrideEnvironmentLocations(t *testing.T) {
	e := newEnv(t)
	otherClaude := filepath.Join(t.TempDir(), "other-claude")
	otherHome := filepath.Join(t.TempDir(), "other-home")
	testutil.WriteFiles(t, otherClaude, map[string]string{"agents/reviewer.md": reviewerMD})

	r := e.run("list", "--claude-dir", otherClaude, "--home", otherHome)

	if r.code != 0 {
		t.Fatalf("exit code = %d\n%s", r.code, r.stderr)
	}
	if _, err := os.Stat(filepath.Join(otherHome, "presets", "initial", "agents", "reviewer.md")); err != nil {
		t.Errorf("flags were ignored: %v", err)
	}
	if _, err := os.Stat(e.home); !os.IsNotExist(err) {
		t.Errorf("the environment location %s was used despite --home", e.home)
	}
}

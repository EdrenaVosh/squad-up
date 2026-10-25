package cli_test

import (
	"strings"
	"testing"
)

func TestTemplatesListsTheStarters(t *testing.T) {
	e := newEnv(t)

	r := e.run("templates")

	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	for _, want := range []string{"bug-hunters", "feature-creator", "Bug Hunters", "Feature Creator", "debugger, fixer, reviewer, tester", "planner, executor, reviewer", "squad new --template"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("stdout should mention %q:\n%s", want, r.stdout)
		}
	}
}

func TestNewFromATemplate(t *testing.T) {
	e := newEnv(t)

	if r := e.run("new", "--template", "bug-hunters"); r.code != 0 || !e.hasPreset("bug-hunters") {
		t.Fatalf("default name: %d %q", r.code, r.stderr)
	}
	r := e.run("new", "mine", "--template", "feature-creator", "--title", "Mine")
	if r.code != 0 || !e.hasPreset("mine") {
		t.Fatalf("named: %d %q", r.code, r.stderr)
	}
	if !strings.Contains(r.stdout, "feature-creator") || !strings.Contains(r.stdout, "3 agents") {
		t.Errorf("stdout = %q", r.stdout)
	}
	if lint := e.run("lint", "mine", "--strict"); lint.code != 0 {
		t.Errorf("a fresh template should lint clean: %s", lint.stdout)
	}
}

func TestNewFromATemplateReportsMistakes(t *testing.T) {
	e := newEnv(t)
	if r := e.run("new", "x", "--template", "nope"); r.code != 1 || !strings.Contains(r.stderr, "bug-hunters") {
		t.Errorf("unknown template: %d %q", r.code, r.stderr)
	}
	if r := e.run("new", "x", "--template", "bug-hunters", "--from", "current"); r.code != 2 {
		t.Errorf("--template with --from: %d %q", r.code, r.stderr)
	}
	e.run("new", "taken", "--template", "bug-hunters")
	if r := e.run("new", "taken", "--template", "bug-hunters"); r.code != 1 || !strings.Contains(r.stderr, "choose another name") {
		t.Errorf("taken name: %d %q", r.code, r.stderr)
	}
}

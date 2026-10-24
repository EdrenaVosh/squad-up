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
	for _, want := range []string{"solo-indie", "bug-hunt", "refactor-day", "ship-it", "read-only-audit", "planner, reviewer, tester", "squad new --template"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("stdout should mention %q:\n%s", want, r.stdout)
		}
	}
}

func TestNewFromATemplate(t *testing.T) {
	e := newEnv(t)

	if r := e.run("new", "--template", "bug-hunt"); r.code != 0 || !e.hasPreset("bug-hunt") {
		t.Fatalf("default name: %d %q", r.code, r.stderr)
	}
	r := e.run("new", "mine", "--template", "solo-indie", "--title", "Mine")
	if r.code != 0 || !e.hasPreset("mine") {
		t.Fatalf("named: %d %q", r.code, r.stderr)
	}
	if !strings.Contains(r.stdout, "solo-indie") || !strings.Contains(r.stdout, "3 agents") {
		t.Errorf("stdout = %q", r.stdout)
	}
	if lint := e.run("lint", "mine", "--strict"); lint.code != 0 {
		t.Errorf("a fresh template should lint clean: %s", lint.stdout)
	}
}

func TestNewFromATemplateReportsMistakes(t *testing.T) {
	e := newEnv(t)
	if r := e.run("new", "x", "--template", "nope"); r.code != 1 || !strings.Contains(r.stderr, "solo-indie") {
		t.Errorf("unknown template: %d %q", r.code, r.stderr)
	}
	if r := e.run("new", "x", "--template", "bug-hunt", "--from", "current"); r.code != 2 {
		t.Errorf("--template with --from: %d %q", r.code, r.stderr)
	}
	e.run("new", "taken", "--template", "bug-hunt")
	if r := e.run("new", "taken", "--template", "bug-hunt"); r.code != 1 || !strings.Contains(r.stderr, "choose another name") {
		t.Errorf("taken name: %d %q", r.code, r.stderr)
	}
}

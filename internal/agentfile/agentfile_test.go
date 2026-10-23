package agentfile_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/EdrenaVosh/squadup/internal/agentfile"
)

const canonicalSample = `---
name: reviewer
description: Reviews diffs for bugs and risky changes. Use after any code change, before commit.
tools: Read, Grep, Glob, Bash
model: sonnet
permissionMode: default
effort: high
color: green
---
You are a strict code reviewer. Report findings ranked by severity...
`

func TestParseThenMarshalReturnsIdenticalBytes(t *testing.T) {
	cases := map[string]string{
		"canonical":                canonicalSample,
		"crlf line endings":        strings.ReplaceAll(canonicalSample, "\n", "\r\n"),
		"reordered keys, comments": "---\n# who am I\ncolor: green\nname: \"reviewer\"\ndescription: 'Use when reviewing'\n---\nBody\n",
		"no trailing newline":      "---\nname: a\ndescription: d\n---\nBody without newline",
		"empty body":               "---\nname: a\ndescription: d\n---\n",
		"empty body no newline":    "---\nname: a\ndescription: d\n---",
		"dashes inside body":       "---\nname: a\ndescription: d\n---\nIntro\n---\nnot frontmatter\n---\n",
		"unknown fields":           "---\nname: a\ndescription: d\ndisallowed_tools: Bash\nModel: opus\n---\nBody\n",
		"block scalar description": "---\nname: a\ndescription: |\n  line one\n  line two\n---\nBody\n",
		"blank line after fm":      "---\nname: a\ndescription: d\n---\n\nBody\n",
		"flow list":                "---\nname: a\ndescription: d\ntools: [Read, Grep]\n---\nBody\n",
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			a, err := agentfile.Parse([]byte(input))
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			got, err := a.Marshal()

			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			if string(got) != input {
				t.Fatalf("round trip changed bytes\n got: %q\nwant: %q", got, input)
			}
		})
	}
}

func TestAccessors(t *testing.T) {
	a, err := agentfile.Parse([]byte(canonicalSample))
	if err != nil {
		t.Fatal(err)
	}

	if a.Name() != "reviewer" {
		t.Errorf("Name() = %q", a.Name())
	}
	if !strings.HasPrefix(a.Description(), "Reviews diffs") {
		t.Errorf("Description() = %q", a.Description())
	}
	if v, ok := a.Get("model"); !ok || v != "sonnet" {
		t.Errorf("Get(model) = %v, %v", v, ok)
	}
	if _, ok := a.Get("missing"); ok {
		t.Error("Get(missing) reported presence")
	}
	if !strings.HasPrefix(a.Prompt(), "You are a strict code reviewer") {
		t.Errorf("Prompt() = %q", a.Prompt())
	}
	wantKeys := []string{"name", "description", "tools", "model", "permissionMode", "effort", "color"}
	if got := strings.Join(a.Keys(), ","); got != strings.Join(wantKeys, ",") {
		t.Errorf("Keys() = %v", a.Keys())
	}
}

func TestCanonicalOrdersKnownKeysThenUnknownAndSkipsEmpty(t *testing.T) {
	input := "---\ncolor: green\ndisallowed_tools: Bash\nmodel: \"\"\ndescription: d\ntools:\nname: a\neffort: high\n---\nBody\r\nkept\n"
	a, err := agentfile.Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}

	got, err := a.Canonical()

	if err != nil {
		t.Fatal(err)
	}
	want := "---\nname: a\ndescription: d\neffort: high\ncolor: green\ndisallowed_tools: Bash\n---\nBody\r\nkept\n"
	if string(got) != want {
		t.Fatalf("Canonical()\n got: %q\nwant: %q", got, want)
	}
}

func TestCanonicalOfCanonicalInputIsStable(t *testing.T) {
	a, err := agentfile.Parse([]byte(canonicalSample))
	if err != nil {
		t.Fatal(err)
	}

	got, err := a.Canonical()

	if err != nil || string(got) != canonicalSample {
		t.Fatalf("Canonical() = %q, %v", got, err)
	}
}

func TestSetMakesMarshalEmitCanonicalForm(t *testing.T) {
	a, err := agentfile.Parse([]byte("---\ncolor: green\nname: a\ndescription: d\n---\nBody\n"))
	if err != nil {
		t.Fatal(err)
	}

	a.Set("model", "opus")
	got, err := a.Marshal()

	if err != nil {
		t.Fatal(err)
	}
	if want := "---\nname: a\ndescription: d\nmodel: opus\ncolor: green\n---\nBody\n"; string(got) != want {
		t.Fatalf("Marshal() = %q, want %q", got, want)
	}
}

func TestCanonicalQuotesValuesThatNeedIt(t *testing.T) {
	a, err := agentfile.Parse([]byte("---\nname: a\ndescription: d\n---\nBody\n"))
	if err != nil {
		t.Fatal(err)
	}
	a.Set("description", "Use when: the build is red # urgent")

	out, err := a.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	back, err := agentfile.Parse(out)

	if err != nil {
		t.Fatalf("re-parse error = %v\n%s", err, out)
	}
	if back.Description() != "Use when: the build is red # urgent" {
		t.Fatalf("Description() = %q", back.Description())
	}
}

func TestParseErrors(t *testing.T) {
	cases := map[string]struct {
		input string
		want  error
	}{
		"no frontmatter":     {"just text\n", agentfile.ErrNoFrontmatter},
		"empty file":         {"", agentfile.ErrNoFrontmatter},
		"unterminated":       {"---\nname: a\n", agentfile.ErrUnterminated},
		"frontmatter a list": {"---\n- a\n- b\n---\nBody\n", agentfile.ErrNotMapping},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := agentfile.Parse([]byte(c.input))

			if !errors.Is(err, c.want) {
				t.Fatalf("Parse() error = %v, want %v", err, c.want)
			}
		})
	}
}

func TestParseReportsBrokenYAML(t *testing.T) {
	_, err := agentfile.Parse([]byte("---\nname: [unclosed\n---\nBody\n"))

	if err == nil {
		t.Fatal("Parse() accepted broken YAML")
	}
	if errors.Is(err, agentfile.ErrNoFrontmatter) || errors.Is(err, agentfile.ErrUnterminated) {
		t.Fatalf("Parse() error = %v, want a YAML error", err)
	}
}

func TestSetBodyReplacesThePromptAndKeepsTheFrontmatter(t *testing.T) {
	a, err := agentfile.Parse([]byte("---\ncolor: green\nname: a\ndescription: d\nzzz: 1\n---\nOld body\n"))
	if err != nil {
		t.Fatal(err)
	}

	a.SetBody("New body\nwith two lines\n")
	got, err := a.Marshal()

	if err != nil {
		t.Fatal(err)
	}
	want := "---\nname: a\ndescription: d\ncolor: green\nzzz: 1\n---\nNew body\nwith two lines\n"
	if string(got) != want {
		t.Fatalf("Marshal() = %q, want %q", got, want)
	}
	if a.Prompt() != "New body\nwith two lines\n" {
		t.Errorf("Prompt() = %q", a.Prompt())
	}
}

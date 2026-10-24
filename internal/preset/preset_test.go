package preset_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/EdrenaVosh/squadup/internal/preset"
)

func sample() preset.Preset {
	return preset.Preset{
		Schema:      1,
		Name:        "solo-indie",
		Title:       "Solo Indie",
		Description: "Planner, reviewer and tester for one-person apps",
		Accent:      "cyan",
		Mode:        "replace",
		Agents:      []string{"planner", "reviewer", "tester"},
		Settings: preset.Settings{
			Model:       "sonnet",
			EffortLevel: "high",
			Permissions: &preset.Permissions{
				DefaultMode: "acceptEdits",
				Allow:       []string{"Bash(npm run *)"},
				Ask:         []string{"Bash(git push *)"},
				Deny:        []string{"Read(./.env)", "Read(./.env.*)"},
			},
		},
	}
}

func TestMarshalUnmarshalRoundTrip(t *testing.T) {
	want := sample()
	main := "planner"
	want.Main = &main

	data, err := preset.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := preset.Unmarshal(data)

	if err != nil {
		t.Fatalf("Unmarshal() error = %v\n%s", err, data)
	}
	if !reflect.DeepEqual(*got, want) {
		t.Fatalf("round trip changed the preset\n got: %+v\nwant: %+v\nyaml:\n%s", *got, want, data)
	}
}

func TestMarshalFollowsDocumentedLayout(t *testing.T) {
	data, err := preset.Marshal(sample())
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)

	for _, want := range []string{"schema: 1\n", "name: solo-indie\n", "main: null\n", "agents: [planner, reviewer, tester]\n", "  model: sonnet\n"} {
		if !strings.Contains(text, want) {
			t.Errorf("yaml lacks %q:\n%s", want, text)
		}
	}
	last := -1
	for _, key := range []string{"schema:", "name:", "title:", "description:", "accent:", "mode:", "main:", "agents:", "settings:"} {
		i := strings.Index(text, "\n"+key)
		if key == "schema:" {
			i = strings.Index(text, key)
		}
		if i <= last {
			t.Errorf("key %s is out of order in:\n%s", key, text)
		}
		last = i
	}
}

func TestMarshalOmitsUnsetSettings(t *testing.T) {
	p := preset.Preset{Schema: 1, Name: "bare", Mode: "replace", Agents: []string{}}

	data, err := preset.Marshal(p)

	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "settings") {
		t.Fatalf("empty settings were written:\n%s", data)
	}
}

func TestUnmarshalRejectsUnknownFields(t *testing.T) {
	_, err := preset.Unmarshal([]byte("schema: 1\nname: a\nmode: replace\nagents: []\nbogus: sonnet\n"))

	if err == nil {
		t.Fatal("Unmarshal() accepted an unknown field")
	}
}

func TestValidateAcceptsSample(t *testing.T) {
	if err := sample().Validate(); err != nil {
		t.Fatalf("Validate() = %v", err)
	}
}

func TestValidateReportsEachProblemWithItsField(t *testing.T) {
	cases := map[string]struct {
		mutate func(*preset.Preset)
		field  string
	}{
		"uppercase name":       {func(p *preset.Preset) { p.Name = "Solo" }, "name"},
		"leading dash name":    {func(p *preset.Preset) { p.Name = "-solo" }, "name"},
		"empty name":           {func(p *preset.Preset) { p.Name = "" }, "name"},
		"path in name":         {func(p *preset.Preset) { p.Name = "../x" }, "name"},
		"too long name":        {func(p *preset.Preset) { p.Name = strings.Repeat("a", 65) }, "name"},
		"unsupported schema":   {func(p *preset.Preset) { p.Schema = 2 }, "schema"},
		"unknown mode":         {func(p *preset.Preset) { p.Mode = "merge" }, "mode"},
		"unknown accent":       {func(p *preset.Preset) { p.Accent = "magenta" }, "accent"},
		"unknown effort":       {func(p *preset.Preset) { p.Settings.EffortLevel = "ultra" }, "effortLevel"},
		"max in settings key":  {func(p *preset.Preset) { p.Settings.EffortLevel = "max" }, "effortLevel"},
		"unknown default mode": {func(p *preset.Preset) { p.Settings.Permissions.DefaultMode = "yolo" }, "defaultMode"},
		"duplicate agent":      {func(p *preset.Preset) { p.Agents = []string{"a", "a"} }, "agents"},
		"invalid agent name":   {func(p *preset.Preset) { p.Agents = []string{"Bad Name"} }, "agents"},
		"traversal agent name": {func(p *preset.Preset) { p.Agents = []string{"../evil"} }, "agents"},
		"empty agent name":     {func(p *preset.Preset) { p.Agents = []string{""} }, "agents"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			p := sample()
			c.mutate(&p)

			err := p.Validate()

			if err == nil || !strings.Contains(err.Error(), c.field) {
				t.Fatalf("Validate() = %v, want an error mentioning %q", err, c.field)
			}
		})
	}
}

func TestValidName(t *testing.T) {
	for _, ok := range []string{"a", "solo-indie", "x1", "0day", strings.Repeat("a", 64)} {
		if !preset.ValidName(ok) {
			t.Errorf("ValidName(%q) = false", ok)
		}
	}
	for _, bad := range []string{"", "-a", "A", "a_b", "a b", "a/b", "..", "a.md", strings.Repeat("a", 65)} {
		if preset.ValidName(bad) {
			t.Errorf("ValidName(%q) = true", bad)
		}
	}
}

func TestPermissionsFromJSONKeepsKnownAndUnknownKeys(t *testing.T) {
	raw := []byte(`{
  "defaultMode": "acceptEdits",
  "allow": ["Bash(npm run *)"],
  "ask": [],
  "deny": ["Read(./.env)"],
  "additionalDirectories": ["../shared"],
  "disableBypassPermissionsMode": "disable"
}`)

	got, err := preset.PermissionsFromJSON(raw)

	if err != nil {
		t.Fatal(err)
	}
	if got.DefaultMode != "acceptEdits" || !reflect.DeepEqual(got.Allow, []string{"Bash(npm run *)"}) ||
		!reflect.DeepEqual(got.Deny, []string{"Read(./.env)"}) || !reflect.DeepEqual(got.AdditionalDirectories, []string{"../shared"}) {
		t.Fatalf("PermissionsFromJSON() = %+v", got)
	}
	if got.Extra["disableBypassPermissionsMode"] != "disable" {
		t.Fatalf("Extra = %v, want the unknown key kept", got.Extra)
	}
}

func TestPermissionsFromJSONRejectsNonObjects(t *testing.T) {
	if _, err := preset.PermissionsFromJSON([]byte(`"acceptEdits"`)); err == nil {
		t.Fatal("PermissionsFromJSON() accepted a string")
	}
}

func TestPermissionsSummary(t *testing.T) {
	p := &preset.Permissions{DefaultMode: "acceptEdits", Allow: []string{"a", "b"}, Ask: []string{"c"}, Deny: []string{"d", "e", "f"}}
	if got := p.Summary(); got != "acceptEdits · 2 allow · 1 ask · 3 deny" {
		t.Errorf("Summary() = %q", got)
	}
	if got := (&preset.Permissions{}).Summary(); got != "default · 0 allow · 0 ask · 0 deny" {
		t.Errorf("Summary() of an empty value = %q", got)
	}
	var none *preset.Permissions
	if got := none.Summary(); got != "" {
		t.Errorf("Summary() of nil = %q, want empty", got)
	}
}

func TestSettingsSurviveAJSONRoundTrip(t *testing.T) {
	in := preset.Settings{
		Model: "sonnet", EffortLevel: "high",
		Permissions: &preset.Permissions{DefaultMode: "acceptEdits", Allow: []string{"Bash(npm run *)"}, Ask: []string{"Bash(git push *)"}, Extra: map[string]any{"x": "y"}},
	}

	data, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out preset.Settings
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round trip changed the settings\n got: %+v\nwant: %+v", out, in)
	}
	for _, key := range []string{`"model"`, `"effortLevel"`, `"permissions"`, `"defaultMode"`, `"allow"`, `"ask"`} {
		if !strings.Contains(string(data), key) {
			t.Errorf("JSON lacks %s: %s", key, data)
		}
	}
	if strings.Contains(string(data), `"deny"`) {
		t.Errorf("empty lists must be left out: %s", data)
	}
}

package models_test

import (
	"context"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/EdrenaVosh/squadup/internal/claude"
	"github.com/EdrenaVosh/squadup/internal/models"
	"github.com/EdrenaVosh/squadup/internal/paths"
	"github.com/EdrenaVosh/squadup/internal/testutil"
)

// ---- catalog ----

func TestCatalogListsTheDocumentedModels(t *testing.T) {
	var agents, settings []string
	for _, e := range models.Catalog() {
		if e.Agents {
			agents = append(agents, e.Name)
		}
		if e.Settings {
			settings = append(settings, e.Name)
		}
	}

	if want := []string{"inherit", "fable", "opus", "sonnet", "haiku"}; !reflect.DeepEqual(agents, want) {
		t.Errorf("agent models = %v, want %v", agents, want)
	}
	if want := []string{"fable", "opus", "sonnet", "haiku", "default", "best", "opus[1m]", "sonnet[1m]", "opusplan"}; !reflect.DeepEqual(settings, want) {
		t.Errorf("settings models = %v, want %v", settings, want)
	}
}

func TestValidAgentModel(t *testing.T) {
	for _, ok := range []string{"inherit", "sonnet", "opus", "haiku", "fable", "claude-sonnet-4-5-20250929", "claude-opus-5-5"} {
		if !models.ValidAgentModel(ok) {
			t.Errorf("ValidAgentModel(%q) = false", ok)
		}
	}
	for _, bad := range []string{"", "gpt-5", "Sonnet", "best", "opusplan", "claude sonnet"} {
		if models.ValidAgentModel(bad) {
			t.Errorf("ValidAgentModel(%q) = true", bad)
		}
	}
}

// ---- policy ----

func writePolicy(t *testing.T, userSettings, managed string, managedD map[string]string) (claudeDir string, managedDirs []string) {
	t.Helper()
	claudeDir = filepath.Join(t.TempDir(), "claude")
	managedDir := filepath.Join(t.TempDir(), "managed")
	if userSettings != "" {
		testutil.WriteFiles(t, claudeDir, map[string]string{"settings.json": userSettings})
	}
	if managed != "" {
		testutil.WriteFiles(t, managedDir, map[string]string{"managed-settings.json": managed})
	}
	for name, content := range managedD {
		testutil.WriteFiles(t, managedDir, map[string]string{"managed-settings.d/" + name: content})
	}
	return claudeDir, []string{managedDir}
}

func TestLoadPolicyWithoutAnyFilesRestrictsNothing(t *testing.T) {
	claudeDir, managed := writePolicy(t, "", "", nil)

	p, notes, err := models.LoadPolicy(claudeDir, managed)

	if err != nil || p.HasAvailable || len(p.Denied) != 0 || len(notes) != 0 {
		t.Fatalf("LoadPolicy() = %+v, %v, %v", p, notes, err)
	}
}

func TestLoadPolicyReadsTheUserAllowlist(t *testing.T) {
	claudeDir, managed := writePolicy(t, `{"availableModels": ["sonnet", "haiku"], "model": "opus"}`, "", nil)

	p, _, err := models.LoadPolicy(claudeDir, managed)

	if err != nil {
		t.Fatal(err)
	}
	if !p.HasAvailable || !reflect.DeepEqual(p.Available, []string{"sonnet", "haiku"}) {
		t.Fatalf("policy = %+v", p)
	}
}

func TestManagedAllowlistReplacesTheUsersAndEnforcementComesFromManaged(t *testing.T) {
	claudeDir, managed := writePolicy(t,
		`{"availableModels": ["opus"], "enforceAvailableModels": true}`,
		`{"availableModels": ["sonnet"], "enforceAvailableModels": true, "availableModelsMatch": "exact", "deniedModels": ["claude-sonnet-4-5"]}`,
		nil)

	p, notes, err := models.LoadPolicy(claudeDir, managed)

	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.Available, []string{"sonnet"}) || !p.Enforce || !p.Exact || !reflect.DeepEqual(p.Denied, []string{"claude-sonnet-4-5"}) {
		t.Fatalf("policy = %+v", p)
	}
	_ = notes
}

func TestUserListsAreMergedWhenThereIsNoManagedList(t *testing.T) {
	claudeDir, managed := writePolicy(t, `{"availableModels": ["sonnet"]}`, `{"deniedModels": ["opus"]}`, nil)

	p, _, err := models.LoadPolicy(claudeDir, managed)

	if err != nil || !reflect.DeepEqual(p.Available, []string{"sonnet"}) || !reflect.DeepEqual(p.Denied, []string{"opus"}) {
		t.Fatalf("policy = %+v, %v", p, err)
	}
}

func TestDeniedModelsInUserSettingsAreIgnoredWithANote(t *testing.T) {
	claudeDir, managed := writePolicy(t, `{"deniedModels": ["opus"]}`, "", nil)

	p, notes, err := models.LoadPolicy(claudeDir, managed)

	if err != nil || len(p.Denied) != 0 {
		t.Fatalf("policy = %+v, %v", p, err)
	}
	if len(notes) != 1 || !strings.Contains(notes[0], "deniedModels") {
		t.Errorf("notes = %v, want one about deniedModels", notes)
	}
}

func TestManagedDropInFilesOverrideInAlphabeticalOrder(t *testing.T) {
	claudeDir, managed := writePolicy(t, "", `{"availableModels": ["haiku"]}`, map[string]string{
		"20-b.json": `{"availableModels": ["opus"]}`,
		"10-a.json": `{"availableModels": ["sonnet"], "deniedModels": ["claude-opus-5-5"]}`,
		".hidden":   `{"availableModels": ["fable"]}`,
	})

	p, _, err := models.LoadPolicy(claudeDir, managed)

	if err != nil || !reflect.DeepEqual(p.Available, []string{"opus"}) || !reflect.DeepEqual(p.Denied, []string{"claude-opus-5-5"}) {
		t.Fatalf("policy = %+v, %v", p, err)
	}
}

func TestUnreadablePolicyFilesBecomeNotesNotErrors(t *testing.T) {
	claudeDir, managed := writePolicy(t, `{"availableModels": `, "", nil)

	p, notes, err := models.LoadPolicy(claudeDir, managed)

	if err != nil || p.HasAvailable {
		t.Fatalf("policy = %+v, %v", p, err)
	}
	if len(notes) != 1 || !strings.Contains(notes[0], "settings.json") {
		t.Errorf("notes = %v", notes)
	}
}

func TestManagedDirsPerSystem(t *testing.T) {
	dirs := models.ManagedDirs()

	if len(dirs) != 1 || !filepath.IsAbs(dirs[0]) {
		t.Fatalf("ManagedDirs() = %v", dirs)
	}
}

func TestCheck(t *testing.T) {
	sonnetHaiku := models.Policy{HasAvailable: true, Available: []string{"sonnet", "haiku"}}
	pinned := models.Policy{HasAvailable: true, Available: []string{"sonnet", "claude-sonnet-4-5"}}
	prefix := models.Policy{HasAvailable: true, Available: []string{"claude-fable-5"}}
	exact := models.Policy{HasAvailable: true, Available: []string{"claude-fable-5"}, Exact: true}
	deniedID := models.Policy{Denied: []string{"claude-opus-5-5"}}
	deniedFamily := models.Policy{Denied: []string{"opus"}}
	empty := models.Policy{HasAvailable: true, Available: []string{}}

	cases := []struct {
		name         string
		policy       models.Policy
		alias, id    string
		blocked      bool
		listed       bool
		uncertain    bool
		reasonSubstr string
	}{
		{"no policy allows everything", models.Policy{}, "opus", "", false, false, false, ""},
		{"alias in the allowlist", sonnetHaiku, "sonnet", "", false, true, false, ""},
		{"alias with a known id", sonnetHaiku, "haiku", "claude-haiku-5-5", false, true, false, ""},
		{"alias not in the allowlist", sonnetHaiku, "opus", "", true, false, false, "availableModels"},
		{"the [1m] suffix is ignored", sonnetHaiku, "sonnet[1m]", "", false, true, false, ""},
		{"inherit is always fine", sonnetHaiku, "inherit", "", false, false, false, ""},
		{"default is always fine", sonnetHaiku, "default", "", false, false, false, ""},
		{"a pinned version allows only itself", pinned, "sonnet", "claude-sonnet-4-5-20250929", false, true, false, ""},
		{"a pinned version blocks the latest", pinned, "sonnet", "claude-sonnet-5-5", true, false, false, "availableModels"},
		{"a pinned family is uncertain for the bare alias", pinned, "sonnet", "", false, false, true, "only some versions"},
		{"a prefix matches later versions", prefix, "fable", "claude-fable-5-1", false, true, false, ""},
		{"a prefix stops at the version boundary", prefix, "fable", "claude-fable-50", true, false, false, "availableModels"},
		{"exact matching allows only the named version", exact, "fable", "claude-fable-5-1", true, false, false, "availableModels"},
		{"exact matching allows the named version", exact, "fable", "claude-fable-5", false, true, false, ""},
		{"a denied id blocks", deniedID, "opus", "claude-opus-5-5", true, false, false, "deniedModels"},
		{"a denied id is uncertain for the bare alias", deniedID, "opus", "", false, false, true, "deniedModels"},
		{"a denied family blocks the alias", deniedFamily, "opus", "", true, false, false, "deniedModels"},
		{"an empty allowlist blocks named models", empty, "sonnet", "", true, false, false, "availableModels"},
		{"a full id is checked as such", sonnetHaiku, "claude-sonnet-5-5", "claude-sonnet-5-5", false, true, false, ""},
		{"opusplan needs opus and sonnet", sonnetHaiku, "opusplan", "", true, false, false, "opus"},
		{"best needs fable or opus", sonnetHaiku, "best", "", true, false, false, "fable"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.policy.Check(c.alias, c.id)

			if got.Blocked != c.blocked || got.Listed != c.listed || got.Uncertain != c.uncertain {
				t.Fatalf("Check(%q, %q) = %+v, want blocked=%v listed=%v uncertain=%v", c.alias, c.id, got, c.blocked, c.listed, c.uncertain)
			}
			if c.reasonSubstr != "" && !strings.Contains(got.Reason, c.reasonSubstr) {
				t.Errorf("Reason = %q, want it to mention %q", got.Reason, c.reasonSubstr)
			}
		})
	}
}

// ---- cache ----

func TestCacheRoundTripAndValidity(t *testing.T) {
	p := paths.Paths{Home: t.TempDir()}
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	c := &models.Cache{
		ClaudeVersion: "2.1.309 (Claude Code)",
		AuthMethod:    "claude.ai",
		ProbedAt:      now,
		Results: map[string]models.ProbeResult{
			"opus":  {OK: true, ID: "claude-opus-5-5"},
			"haiku": {OK: false, Error: "no access"},
		},
	}
	if err := models.SaveCache(p, c); err != nil {
		t.Fatal(err)
	}

	got, err := models.LoadCache(p)

	if err != nil || !reflect.DeepEqual(got, c) {
		t.Fatalf("LoadCache() = %+v, %v", got, err)
	}
	valid := func(version, auth string, at time.Time) bool { return got.Valid(version, auth, at) }
	if !valid("2.1.309 (Claude Code)", "claude.ai", now.Add(23*time.Hour)) {
		t.Error("a cache younger than 24h with the same key must be valid")
	}
	if valid("2.1.309 (Claude Code)", "claude.ai", now.Add(25*time.Hour)) {
		t.Error("a cache older than 24h must expire")
	}
	if valid("2.2.0 (Claude Code)", "claude.ai", now) {
		t.Error("a new claude version must invalidate the cache")
	}
	if valid("2.1.309 (Claude Code)", "api_key", now) {
		t.Error("a different account type must invalidate the cache")
	}
	if !valid("2.1.309 (Claude Code)", "", now) {
		t.Error("an empty auth method skips the comparison")
	}
}

func TestLoadCacheOfMissingOrBrokenFileIsNil(t *testing.T) {
	p := paths.Paths{Home: t.TempDir()}
	if got, err := models.LoadCache(p); got != nil || err != nil {
		t.Fatalf("missing: %+v, %v", got, err)
	}
	testutil.WriteFiles(t, p.Home, map[string]string{"cache/models.json": "{broken"})
	if got, err := models.LoadCache(p); got != nil || err != nil {
		t.Fatalf("broken: %+v, %v", got, err)
	}
}

// ---- probing ----

func TestProbeRunsAllAliasesInParallel(t *testing.T) {
	bin := testutil.FakeClaude(t)
	t.Setenv("FAKE_CLAUDE_SLEEP_MS", "400")
	t.Setenv("FAKE_CLAUDE_UNAVAILABLE", "fable")

	start := time.Now()
	got := models.Probe(context.Background(), claude.Client{Bin: bin}, []string{"fable", "opus", "sonnet", "haiku"}, 30*time.Second)

	if elapsed := time.Since(start); elapsed > 1500*time.Millisecond {
		t.Errorf("probing took %v: the requests were not parallel", elapsed)
	}
	if got["fable"].OK || !strings.Contains(got["fable"].Error, "issue with the selected model") {
		t.Errorf("fable = %+v", got["fable"])
	}
	for alias, want := range map[string]string{"opus": "claude-opus-5-5", "sonnet": "claude-sonnet-5-5", "haiku": "claude-haiku-5-5"} {
		if !got[alias].OK || got[alias].ID != want {
			t.Errorf("%s = %+v, want %s", alias, got[alias], want)
		}
	}
}

func TestProbeTimeoutIsPerRequest(t *testing.T) {
	bin := testutil.FakeClaude(t)
	t.Setenv("FAKE_CLAUDE_SLEEP_MS", "3000")

	start := time.Now()
	got := models.Probe(context.Background(), claude.Client{Bin: bin}, []string{"haiku"}, 300*time.Millisecond)

	if got["haiku"].OK || !strings.Contains(got["haiku"].Error, "timed out") {
		t.Errorf("haiku = %+v", got["haiku"])
	}
	if time.Since(start) > 2*time.Second {
		t.Errorf("took %v", time.Since(start))
	}
}

// ---- evaluation ----

func find(results []models.Result, name string) models.Result {
	for _, r := range results {
		if r.Name == name {
			return r
		}
	}
	return models.Result{}
}

func TestEvaluateWithoutAnyEvidenceIsUnknown(t *testing.T) {
	results := models.Evaluate(models.Catalog(), models.Policy{}, nil)

	for _, name := range []string{"fable", "opus", "sonnet", "haiku", "best", "opusplan", "opus[1m]"} {
		if got := find(results, name); got.Status != models.StatusUnknown {
			t.Errorf("%s = %+v, want unknown", name, got)
		}
	}
	for _, name := range []string{"inherit", "default"} {
		if got := find(results, name); got.Status != models.StatusVerified {
			t.Errorf("%s = %+v, want always available", name, got)
		}
	}
}

func TestEvaluateUsesTheProbeResults(t *testing.T) {
	cache := &models.Cache{Results: map[string]models.ProbeResult{
		"opus":   {OK: true, ID: "claude-opus-5-5"},
		"haiku":  {OK: false, Error: "There's an issue with the selected model (haiku)."},
		"sonnet": {OK: true, ID: "claude-sonnet-5-5"},
	}}

	results := models.Evaluate(models.Catalog(), models.Policy{}, cache)

	if got := find(results, "opus"); got.Status != models.StatusVerified || got.ResolvedTo != "claude-opus-5-5" {
		t.Errorf("opus = %+v", got)
	}
	if got := find(results, "haiku"); got.Status != models.StatusUnavailable || !strings.Contains(got.Reason, "issue with the selected model") {
		t.Errorf("haiku = %+v", got)
	}
	if got := find(results, "fable"); got.Status != models.StatusUnknown {
		t.Errorf("fable was not probed and must stay unknown: %+v", got)
	}
	if got := find(results, "opus[1m]"); got.Status != models.StatusUnknown || !strings.Contains(got.Reason, "1M") {
		t.Errorf("opus[1m] = %+v: the 1M window is not probed", got)
	}
	if got := find(results, "opusplan"); got.Status != models.StatusVerified {
		t.Errorf("opusplan needs opus and sonnet, both verified: %+v", got)
	}
}

func TestEvaluateLetsPolicyOverrideAndExplain(t *testing.T) {
	policy := models.Policy{HasAvailable: true, Available: []string{"sonnet", "haiku"}}
	cache := &models.Cache{Results: map[string]models.ProbeResult{"opus": {OK: true, ID: "claude-opus-5-5"}}}

	results := models.Evaluate(models.Catalog(), policy, cache)

	if got := find(results, "opus"); got.Status != models.StatusUnavailable || !strings.Contains(got.Reason, "availableModels") {
		t.Errorf("opus = %+v: the policy beats a successful probe", got)
	}
	if got := find(results, "sonnet"); got.Status != models.StatusAllowed {
		t.Errorf("sonnet = %+v, want allowed by policy", got)
	}
	if got := find(results, "best"); got.Status != models.StatusUnavailable {
		t.Errorf("best = %+v: neither fable nor opus is allowed", got)
	}
}

// ---- versions ----

func TestEveryDocumentedReleaseCanBePickedByItsFullID(t *testing.T) {
	var ids []string
	for _, v := range models.Versions() {
		ids = append(ids, v.Name)
		if !v.Agents || !v.Settings || !v.Version || v.Family == "" || v.Title == "" {
			t.Errorf("%+v is incomplete", v)
		}
		if !models.ValidAgentModel(v.Name) {
			t.Errorf("%s is not accepted in an agent", v.Name)
		}
	}
	for _, want := range []string{"claude-sonnet-5-5", "claude-sonnet-5", "claude-opus-5-5", "claude-opus-4-6", "claude-haiku-5-5", "claude-fable-5-1"} {
		if !slices.Contains(ids, want) {
			t.Errorf("%s is missing from %v", want, ids)
		}
	}
	if models.Pretty("claude-sonnet-5-5") != "sonnet 5.5" || models.Pretty("claude-opus-4-6") != "opus 4.6" || models.Pretty("claude-x-9") != "claude-x-9" {
		t.Error("Pretty does not say releases the way a person does")
	}
	if len(models.FullCatalog()) != len(models.Catalog())+len(models.Versions()) {
		t.Error("the full catalogue is the aliases and the releases")
	}
}

func TestAReleaseIsVerifiedByAnAliasThatResolvedToIt(t *testing.T) {
	cache := &models.Cache{Results: map[string]models.ProbeResult{
		"sonnet": {OK: true, ID: "claude-sonnet-5-5"},
		"opus":   {OK: true, ID: "claude-opus-5-5"},
	}}

	results := models.Evaluate(models.FullCatalog(), models.Policy{}, cache)

	if got := find(results, "claude-sonnet-5-5"); got.Status != models.StatusVerified || got.ResolvedTo != "claude-sonnet-5-5" || !strings.Contains(got.Reason, "sonnet") {
		t.Errorf("sonnet 5.5 = %+v: sonnet resolved to it", got)
	}
	if got := find(results, "claude-sonnet-4-6"); got.Status != models.StatusUnknown {
		t.Errorf("sonnet 4.6 was not probed and no alias resolved to it: %+v", got)
	}
}

func TestAProbeOfAReleaseSaysWhetherItCanBeUsed(t *testing.T) {
	cache := &models.Cache{Results: map[string]models.ProbeResult{
		"claude-opus-4-6":  {OK: true, ID: "claude-opus-4-6"},
		"claude-sonnet-5":  {OK: false, Error: "not available on your plan"},
		"claude-haiku-4-5": {OK: true, ID: "claude-haiku-4-5"},
	}}

	results := models.Evaluate(models.FullCatalog(), models.Policy{}, cache)

	if got := find(results, "claude-opus-4-6"); got.Status != models.StatusVerified {
		t.Errorf("opus 4.6 = %+v", got)
	}
	if got := find(results, "claude-sonnet-5"); got.Status != models.StatusUnavailable || !strings.Contains(got.Reason, "your plan") {
		t.Errorf("sonnet 5 = %+v", got)
	}
}

func TestAReleaseBlockedByThePolicyIsUnavailable(t *testing.T) {
	policy := models.Policy{HasAvailable: true, Available: []string{"opus"}, Enforce: true}

	results := models.Evaluate(models.FullCatalog(), policy, nil)

	if got := find(results, "claude-sonnet-4-5"); got.Status == models.StatusVerified {
		t.Errorf("sonnet 4.5 = %+v", got)
	}
}

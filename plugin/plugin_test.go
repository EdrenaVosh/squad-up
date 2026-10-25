// Package plugin_test checks the Claude Code plugin that ships in this folder
// and the marketplace file at the root of the repository.
package plugin_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readJSON(t *testing.T, path string, into any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, into); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

type manifest struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
}

func TestThePluginManifestNamesThePlugin(t *testing.T) {
	var m manifest
	readJSON(t, filepath.Join(".claude-plugin", "plugin.json"), &m)

	if m.Name != "squad-up" || m.Version == "" || m.Description == "" {
		t.Errorf("manifest = %+v", m)
	}
}

func TestTheMarketplaceListsThePlugin(t *testing.T) {
	var m manifest
	readJSON(t, filepath.Join(".claude-plugin", "plugin.json"), &m)

	var market struct {
		Name  string `json:"name"`
		Owner struct {
			Name string `json:"name"`
		} `json:"owner"`
		Plugins []struct {
			Name        string `json:"name"`
			Source      string `json:"source"`
			Description string `json:"description"`
		} `json:"plugins"`
	}
	readJSON(t, filepath.Join("..", ".claude-plugin", "marketplace.json"), &market)

	if market.Name == "" || market.Owner.Name == "" || len(market.Plugins) != 1 {
		t.Fatalf("marketplace = %+v", market)
	}
	entry := market.Plugins[0]
	if entry.Name != m.Name {
		t.Errorf("the entry is called %q but the manifest %q: installs by the manifest name would fail", entry.Name, m.Name)
	}
	if !strings.HasPrefix(entry.Source, "./") || strings.Contains(entry.Source, "..") {
		t.Errorf("source %q must be a path inside the repository", entry.Source)
	}
	if info, err := os.Stat(filepath.Join("..", filepath.FromSlash(entry.Source), ".claude-plugin", "plugin.json")); err != nil || info.IsDir() {
		t.Errorf("source %q does not hold a plugin: %v", entry.Source, err)
	}
	if entry.Description == "" {
		t.Error("the entry needs a description: it is what people see when they browse")
	}
}

func TestThePluginCarriesNoAgentsHooksOrServers(t *testing.T) {
	for _, name := range []string{"agents", "hooks", ".mcp.json", ".lsp.json", "monitors"} {
		if _, err := os.Stat(name); err == nil {
			t.Errorf("%s would load into every session; the plugin is only a command", name)
		}
	}
}

func TestTheSkillIsOnlyRunByTheUser(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("skills", "squad", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	front, _, ok := strings.Cut(strings.TrimPrefix(text, "---\n"), "\n---\n")
	if !strings.HasPrefix(text, "---\n") || !ok {
		t.Fatal("SKILL.md has no frontmatter")
	}
	for _, want := range []string{"description:", "disable-model-invocation: true", "argument-hint:", "allowed-tools:"} {
		if !strings.Contains(front, want) {
			t.Errorf("frontmatter lacks %q:\n%s", want, front)
		}
	}
	if !strings.Contains(text, "$ARGUMENTS") {
		t.Error("the skill never reads the user's argument")
	}
	if !strings.Contains(text, "install.sh") {
		t.Error("without squad installed the skill must say how to get it")
	}
	if !strings.Contains(text, "Never add `--force`") {
		t.Error("the skill must forbid itself the flags that override a conflict")
	}
}

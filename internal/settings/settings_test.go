package settings_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/EdrenaVosh/squad-up/internal/settings"
)

const sample = `{
  "theme": "dark",
  "model": "opus",
  "env": { "SECRET": "do-not-read" },
  "permissions": {
    "defaultMode": "acceptEdits",
    "allow": ["Bash(npm run *)"],
    "deny": [ "Read(./.env)" ]
  },
  "effortLevel": "high",
  "agent": "reviewer"
}
`

func write(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReadManagedReturnsOnlyManagedKeysWithRawValues(t *testing.T) {
	m, existed, err := settings.ReadManaged(write(t, sample))

	if err != nil || !existed {
		t.Fatalf("ReadManaged() = %v, %v", existed, err)
	}
	if got := m.String("model"); got != "opus" {
		t.Errorf("model = %q", got)
	}
	if got := m.String("effortLevel"); got != "high" {
		t.Errorf("effortLevel = %q", got)
	}
	if got := m.String("agent"); got != "reviewer" {
		t.Errorf("agent = %q", got)
	}
	wantPerms := "{\n    \"defaultMode\": \"acceptEdits\",\n    \"allow\": [\"Bash(npm run *)\"],\n    \"deny\": [ \"Read(./.env)\" ]\n  }"
	if got := string(m.Raw("permissions")); got != wantPerms {
		t.Errorf("permissions raw = %q, want %q", got, wantPerms)
	}
	for _, foreign := range []string{"theme", "env"} {
		if m.Has(foreign) {
			t.Errorf("Has(%q) = true: foreign keys must not be reported", foreign)
		}
	}
	if keys := strings.Join(m.Keys(), ","); keys != "model,effortLevel,agent,permissions" {
		t.Errorf("Keys() = %s, want managed order model,effortLevel,agent,permissions", keys)
	}
}

func TestReadManagedOfMissingFile(t *testing.T) {
	m, existed, err := settings.ReadManaged(filepath.Join(t.TempDir(), "settings.json"))

	if err != nil || existed {
		t.Fatalf("ReadManaged() = %v, %v", existed, err)
	}
	if len(m.Keys()) != 0 {
		t.Fatalf("Keys() = %v", m.Keys())
	}
}

func TestReadManagedWithoutManagedKeys(t *testing.T) {
	m, existed, err := settings.ReadManaged(write(t, `{"theme":"dark"}`))

	if err != nil || !existed || len(m.Keys()) != 0 {
		t.Fatalf("ReadManaged() = %v, %v, keys %v", existed, err, m.Keys())
	}
}

func TestReadManagedRejectsInvalidJSONWithFileName(t *testing.T) {
	path := write(t, `{"model": `)

	_, existed, err := settings.ReadManaged(path)

	if err == nil || !existed {
		t.Fatalf("ReadManaged() = existed %v, err %v; want an error", existed, err)
	}
	if !strings.Contains(err.Error(), path) {
		t.Fatalf("error %q does not name the file", err)
	}
}

func TestPermissionsSummary(t *testing.T) {
	m, _, err := settings.ReadManaged(write(t, sample))
	if err != nil {
		t.Fatal(err)
	}

	got := m.PermissionsSummary()

	if want := "acceptEdits · 1 allow · 0 ask · 1 deny"; got != want {
		t.Fatalf("PermissionsSummary() = %q, want %q", got, want)
	}
}

func TestPermissionsSummaryWithoutPermissions(t *testing.T) {
	m, _, _ := settings.ReadManaged(write(t, `{"model":"opus"}`))

	if got := m.PermissionsSummary(); got != "" {
		t.Fatalf("PermissionsSummary() = %q, want empty", got)
	}
}

// Package settings reads the keys of Claude Code's settings.json that squad
// manages. Every other key is left alone and never inspected.
package settings

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tidwall/gjson"

	"github.com/EdrenaVosh/squadup/internal/fsx"
)

// ManagedKeys are the only settings.json keys squad ever writes.
var ManagedKeys = []string{"model", "effortLevel", "agent", "permissions"}

// Managed holds the raw JSON text of the managed keys that are present.
type Managed struct {
	raw map[string]string
}

// ReadManaged reads the managed keys from the settings file. The bool reports
// whether the file existed. Raw values keep their original formatting.
func ReadManaged(path string) (Managed, bool, error) {
	data, existed, err := fsx.ReadFileIfExists(path)
	if err != nil {
		return Managed{}, false, fmt.Errorf("read %s: %w", path, err)
	}
	if !existed {
		return Managed{}, false, nil
	}
	m, err := ParseManaged(data)
	if err != nil {
		return Managed{}, true, fmt.Errorf("%s is not valid JSON", path)
	}
	return m, true, nil
}

// ParseManaged extracts the managed keys from the text of a settings file.
func ParseManaged(data []byte) (Managed, error) {
	if !gjson.ValidBytes(data) {
		return Managed{}, errors.New("not valid JSON")
	}
	m := Managed{raw: map[string]string{}}
	for _, key := range ManagedKeys {
		if res := gjson.GetBytes(data, key); res.Exists() {
			m.raw[key] = res.Raw
		}
	}
	return m, nil
}

// ManagedFromRaw wraps already-extracted raw JSON values (key → JSON text).
func ManagedFromRaw(raw map[string]string) Managed {
	m := Managed{raw: map[string]string{}}
	for _, key := range ManagedKeys {
		if value, ok := raw[key]; ok {
			m.raw[key] = value
		}
	}
	return m
}

// Has reports whether the key is present.
func (m Managed) Has(key string) bool {
	_, ok := m.raw[key]
	return ok
}

// Raw returns the key's JSON text exactly as it appears in the file.
func (m Managed) Raw(key string) []byte {
	if raw, ok := m.raw[key]; ok {
		return []byte(raw)
	}
	return nil
}

// String returns the key's value when it is a JSON string, else "".
func (m Managed) String(key string) string {
	if raw, ok := m.raw[key]; ok {
		if res := gjson.Parse(raw); res.Type == gjson.String {
			return res.Str
		}
	}
	return ""
}

// Keys lists the present managed keys in ManagedKeys order.
func (m Managed) Keys() []string {
	var keys []string
	for _, key := range ManagedKeys {
		if m.Has(key) {
			keys = append(keys, key)
		}
	}
	return keys
}

// PermissionsSummary renders "acceptEdits · 4 allow · 1 ask · 2 deny", or ""
// when there is no permissions key.
func (m Managed) PermissionsSummary() string {
	raw, ok := m.raw["permissions"]
	if !ok {
		return ""
	}
	mode := gjson.Get(raw, "defaultMode").String()
	if mode == "" {
		mode = "default"
	}
	count := func(list string) int { return len(gjson.Get(raw, list).Array()) }
	return strings.Join([]string{
		mode,
		fmt.Sprintf("%d allow", count("allow")),
		fmt.Sprintf("%d ask", count("ask")),
		fmt.Sprintf("%d deny", count("deny")),
	}, " · ")
}

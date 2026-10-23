package models

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/tidwall/gjson"
)

// Policy is the model allowlist configured in settings files.
type Policy struct {
	HasAvailable bool     // availableModels is set
	Available    []string // availableModels
	Enforce      bool     // enforceAvailableModels (managed settings only)
	Denied       []string // deniedModels (managed settings only)
	Exact        bool     // availableModelsMatch: "exact" (managed settings only)
}

// ManagedDirs returns where file-based managed settings live on this system.
// Policy delivered by an MDM profile or from the claude.ai console cannot be
// read from here.
func ManagedDirs() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{"/Library/Application Support/ClaudeCode"}
	case "windows":
		return []string{`C:\Program Files\ClaudeCode`}
	}
	return []string{"/etc/claude-code"}
}

// LoadPolicy reads availableModels from the user settings and the managed
// settings, and the managed-only keys from the managed settings. Unreadable
// files are reported in notes, not as errors, so that a broken file does not
// block the rest.
func LoadPolicy(claudeDir string, managedDirs []string) (Policy, []string, error) {
	var notes []string
	var p Policy

	userFile := filepath.Join(claudeDir, "settings.json")
	user, err := readKeys(userFile)
	if err != nil {
		notes = append(notes, err.Error())
	}
	if _, ok := user["deniedModels"]; ok {
		notes = append(notes, fmt.Sprintf("deniedModels in %s is ignored by Claude Code: it only works in managed settings", userFile))
	}

	managed := map[string]gjson.Result{}
	for _, dir := range managedDirs {
		files := []string{filepath.Join(dir, "managed-settings.json")}
		if entries, err := os.ReadDir(filepath.Join(dir, "managed-settings.d")); err == nil {
			var names []string
			for _, e := range entries {
				if !e.IsDir() && !strings.HasPrefix(e.Name(), ".") && strings.HasSuffix(e.Name(), ".json") {
					names = append(names, e.Name())
				}
			}
			slices.Sort(names)
			for _, n := range names {
				files = append(files, filepath.Join(dir, "managed-settings.d", n))
			}
		}
		for _, file := range files {
			keys, err := readKeys(file)
			if err != nil {
				notes = append(notes, err.Error())
			}
			for k, v := range keys {
				managed[k] = v
			}
		}
	}

	switch {
	case managed["availableModels"].Exists():
		p.HasAvailable, p.Available = true, stringList(managed["availableModels"])
	case user["availableModels"].Exists():
		p.HasAvailable, p.Available = true, stringList(user["availableModels"])
	}
	p.Enforce = managed["enforceAvailableModels"].Bool()
	p.Exact = managed["availableModelsMatch"].String() == "exact"
	p.Denied = stringList(managed["deniedModels"])
	return p, notes, nil
}

// stringList converts a JSON array to a string slice; a missing or non-array
// value gives nil, an empty array gives an empty non-nil slice.
func stringList(v gjson.Result) []string {
	if !v.IsArray() {
		return nil
	}
	out := []string{}
	for _, item := range v.Array() {
		out = append(out, item.String())
	}
	return out
}

// readKeys returns the policy keys present in a settings file. A missing file
// is not an error.
func readKeys(path string) (map[string]gjson.Result, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("could not read %s: %w", path, err)
	}
	if !gjson.ValidBytes(data) {
		return nil, fmt.Errorf("could not read %s: not valid JSON", path)
	}
	out := map[string]gjson.Result{}
	for _, key := range []string{"availableModels", "enforceAvailableModels", "availableModelsMatch", "deniedModels"} {
		if v := gjson.GetBytes(data, key); v.Exists() {
			out[key] = v
		}
	}
	return out, nil
}

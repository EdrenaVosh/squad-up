// Package paths resolves where Claude Code's config and squadup's data live.
package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

// Paths is the pair of directories squad works with.
type Paths struct {
	ClaudeDir string // the Claude Code config directory
	Home      string // squadup's own data directory
}

// Resolve picks each directory from, in order: the flag value, the
// environment variable (CLAUDE_CONFIG_DIR, SQUADUP_HOME), the default under the
// user's home directory. Results are absolute.
func Resolve(claudeDirFlag, homeFlag string) (Paths, error) {
	claudeDir, err := resolveOne(claudeDirFlag, "CLAUDE_CONFIG_DIR", ".claude")
	if err != nil {
		return Paths{}, err
	}
	home, err := resolveOne(homeFlag, "SQUADUP_HOME", ".squadup")
	if err != nil {
		return Paths{}, err
	}
	return Paths{ClaudeDir: claudeDir, Home: home}, nil
}

func resolveOne(flag, env, defaultName string) (string, error) {
	dir := flag
	if dir == "" {
		dir = os.Getenv(env)
	}
	if dir == "" {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("find home directory: %w (set %s to choose a location)", err, env)
		}
		dir = filepath.Join(userHome, defaultName)
	}
	return filepath.Abs(dir)
}

// AgentsDir is where Claude Code reads user-level subagents.
func (p Paths) AgentsDir() string { return filepath.Join(p.ClaudeDir, "agents") }

// SettingsFile is Claude Code's user settings.json.
func (p Paths) SettingsFile() string { return filepath.Join(p.ClaudeDir, "settings.json") }

// PresetsDir holds one folder per preset.
func (p Paths) PresetsDir() string { return filepath.Join(p.Home, "presets") }

// PresetDir is the folder of the named preset.
func (p Paths) PresetDir(name string) string { return filepath.Join(p.PresetsDir(), name) }

// InitialDir is the read-only snapshot of the original setup.
func (p Paths) InitialDir() string { return p.PresetDir("initial") }

// StateFile records the active preset and the manifest of written files.
func (p Paths) StateFile() string { return filepath.Join(p.Home, "state.json") }

// ConfigFile holds squadup's own settings.
func (p Paths) ConfigFile() string { return filepath.Join(p.Home, "config.json") }

// BackupsDir holds copies taken before every write.
func (p Paths) BackupsDir() string { return filepath.Join(p.Home, "backups") }

// CacheDir holds cached model probes.
func (p Paths) CacheDir() string { return filepath.Join(p.Home, "cache") }

// LockFile guards against two concurrent writers.
func (p Paths) LockFile() string { return filepath.Join(p.Home, ".lock") }

// LogFile is the operation journal.
func (p Paths) LogFile() string { return filepath.Join(p.Home, "log.jsonl") }

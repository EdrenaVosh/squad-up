// Package runner starts Claude Code with a preset for one session. Nothing is
// written to the Claude config directory: the agents travel in --agents and
// the settings in a temporary file given to --settings.
package runner

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"github.com/goccy/go-yaml"

	"github.com/EdrenaVosh/squad-up/internal/agentfile"
	"github.com/EdrenaVosh/squad-up/internal/preset"
	"github.com/EdrenaVosh/squad-up/internal/team"
)

// acceptedKeys are the frontmatter fields `--agents` understands, besides the
// description and the prompt. color and experimental are not accepted there.
var acceptedKeys = []string{
	"tools", "disallowedTools", "model", "permissionMode", "mcpServers", "hooks",
	"maxTurns", "autoCompactWindow", "skills", "initialPrompt", "memory", "effort",
	"background", "omitClaudeMd", "isolation",
}

// settingsKeys are the managed keys a session gets through --settings; the
// main agent has its own flag.
var settingsKeys = []string{"model", "effortLevel", "permissions"}

// EnvPreset names the environment variable that tells claude (and so its
// status line command) which preset `squad run` started it with.
const EnvPreset = "SQUAD_UP_RUN_PRESET"

// Spec is everything claude is started with.
type Spec struct {
	Preset       string // name of the preset, passed on in EnvPreset
	AgentsJSON   string // value of --agents; empty when the preset has no agents
	SettingsJSON []byte // content of the file given to --settings
	Main         string // value of --agent; empty for none
	Append       string // value of --append-system-prompt: the flow brief when no main agent carries it
}

// maxArgBytes is the longest --agents value squad passes on the command line:
// Windows limits a whole command line to about 32 KB, Linux a single argument
// to 128 KB.
func maxArgBytes() int {
	if runtime.GOOS == "windows" {
		return 30_000
	}
	return 120_000
}

// Build turns a preset into claude's arguments. resolved holds the managed
// settings the preset stands for (see apply.Engine.Resolve).
func Build(b preset.Bundle, resolved map[string]string) (Spec, error) {
	spec := Spec{Preset: b.Preset.Name}

	files, err := team.Compile(b)
	if err != nil {
		return Spec{}, err
	}
	if b.Preset.Main == nil {
		spec.Append = team.Brief(b)
	}

	agents := map[string]map[string]any{}
	for _, file := range files {
		def, err := agentDefinition(file.Data)
		if err != nil {
			return Spec{}, fmt.Errorf("agent %q: %w", file.Name, err)
		}
		agents[file.Name] = def
	}
	if len(agents) > 0 {
		data, err := json.Marshal(agents)
		if err != nil {
			return Spec{}, fmt.Errorf("encode the agents: %w", err)
		}
		if len(data) > maxArgBytes() {
			return Spec{}, fmt.Errorf("the agents of preset %q take %d bytes, more than fit on a command line (%d); equip the preset with `squad use %s` instead",
				b.Preset.Name, len(data), maxArgBytes(), b.Preset.Name)
		}
		spec.AgentsJSON = string(data)
	}

	settings := map[string]json.RawMessage{}
	for _, key := range settingsKeys {
		if raw, ok := resolved[key]; ok {
			settings[key] = json.RawMessage(raw)
		}
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return Spec{}, fmt.Errorf("encode the settings: %w", err)
	}
	spec.SettingsJSON = append(data, '\n')

	switch {
	case b.Preset.Main != nil:
		spec.Main = *b.Preset.Main
	case resolved["agent"] != "":
		_ = json.Unmarshal([]byte(resolved["agent"]), &spec.Main)
	}
	return spec, nil
}

// agentDefinition converts an agent file into the object --agents expects.
func agentDefinition(data []byte) (map[string]any, error) {
	a, err := agentfile.Parse(data)
	if err != nil {
		return nil, err
	}
	def := map[string]any{
		"description": a.Description(),
		"prompt":      a.Prompt(),
	}
	for _, key := range acceptedKeys {
		value, ok := a.Get(key)
		if !ok || value == nil {
			continue
		}
		if s, isString := value.(string); isString && s == "" {
			continue
		}
		def[key] = jsonValue(key, value)
	}
	return def, nil
}

// jsonValue makes a decoded YAML value serialisable as JSON. The tool lists are
// written as an array, the form the documentation uses for --agents, even when
// the frontmatter has a comma-separated string.
func jsonValue(key string, value any) any {
	if s, ok := value.(string); ok && (key == "tools" || key == "disallowedTools") {
		var list []any
		for _, item := range strings.Split(s, ",") {
			if item = strings.TrimSpace(item); item != "" {
				list = append(list, item)
			}
		}
		return list
	}
	return plain(value)
}

func plain(v any) any {
	switch x := v.(type) {
	case yaml.MapSlice:
		out := make(map[string]any, len(x))
		for _, item := range x {
			out[fmt.Sprint(item.Key)] = plain(item.Value)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = plain(item)
		}
		return out
	}
	return v
}

// Args returns the arguments for claude: squad's flags first, then the
// user's own, so that the user can override anything.
func (s Spec) Args(settingsPath string, extra []string) []string {
	var args []string
	if s.AgentsJSON != "" {
		args = append(args, "--agents", s.AgentsJSON)
	}
	args = append(args, "--settings", settingsPath)
	if s.Main != "" {
		args = append(args, "--agent", s.Main)
	}
	if s.Append != "" {
		args = append(args, "--append-system-prompt", s.Append)
	}
	return append(args, extra...)
}

// Streams are the standard streams given to claude. Pass the real terminal
// files for an interactive session.
type Streams struct {
	In       io.Reader
	Out, Err io.Writer
}

// Prepare creates the temporary settings file and the claude command that uses
// it. The caller starts the command and calls cleanup afterwards, which removes
// the file and is safe to call more than once.
func Prepare(bin string, spec Spec, extra []string) (*exec.Cmd, func(), error) {
	file, err := os.CreateTemp("", "squad-settings-*.json")
	if err != nil {
		return nil, nil, fmt.Errorf("create the temporary settings file: %w", err)
	}
	cleanup := func() { _ = os.Remove(file.Name()) }
	if _, err := file.Write(spec.SettingsJSON); err != nil {
		_ = file.Close()
		cleanup()
		return nil, nil, fmt.Errorf("write the temporary settings file: %w", err)
	}
	if err := file.Close(); err != nil {
		cleanup()
		return nil, nil, err
	}
	cmd := exec.Command(bin, spec.Args(file.Name(), extra)...)
	if spec.Preset != "" {
		cmd.Env = append(os.Environ(), EnvPreset+"="+spec.Preset)
	}
	return cmd, cleanup, nil
}

// Run starts claude with the preset and waits for it. Interrupt and
// termination signals are passed on to claude. It returns claude's exit code,
// or 128 plus the signal number when claude was killed by a signal.
func Run(bin string, spec Spec, extra []string, s Streams) (int, error) {
	cmd, cleanup, err := Prepare(bin, spec, extra)
	if err != nil {
		return 0, err
	}
	defer cleanup()
	cmd.Stdin, cmd.Stdout, cmd.Stderr = s.In, s.Out, s.Err

	signals := make(chan os.Signal, 4)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)

	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("start claude: %w", err)
	}
	done := make(chan struct{})
	go func() {
		for {
			select {
			case sig := <-signals:
				_ = cmd.Process.Signal(sig)
			case <-done:
				return
			}
		}
	}()
	waitErr := cmd.Wait()
	close(done)

	var exitErr *exec.ExitError
	if errors.As(waitErr, &exitErr) {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			return 128 + int(status.Signal()), nil
		}
		return exitErr.ExitCode(), nil
	}
	if waitErr != nil {
		return 0, fmt.Errorf("wait for claude: %w", waitErr)
	}
	return 0, nil
}

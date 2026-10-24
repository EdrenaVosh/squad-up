package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/EdrenaVosh/squadup/internal/agentfile"
)

// maxPromptFile is the largest file squad takes as a prompt: 256 KB, far more
// than Claude Code reads usefully.
const maxPromptFile = 256 << 10

// ReadPromptFile reads a Markdown file to use as the prompt of an agent. The
// path may start with ~, be quoted, or have escaped spaces (as a terminal
// pastes a dropped file). A file that begins with YAML frontmatter, such as
// another agent, gives its body only.
func ReadPromptFile(path string) (string, error) {
	path = cleanPath(path)
	if path == "" {
		return "", errors.New("no file given")
	}
	info, err := os.Stat(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return "", fmt.Errorf("%s does not exist", path)
	case err != nil:
		return "", fmt.Errorf("read %s: %w", path, err)
	case info.IsDir():
		return "", fmt.Errorf("%s is a folder, not a file", path)
	case info.Size() > maxPromptFile:
		return "", fmt.Errorf("%s is %d KB; a prompt can be at most %d KB", path, info.Size()>>10, maxPromptFile>>10)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	if !utf8.Valid(data) {
		return "", fmt.Errorf("%s is not UTF-8 text", path)
	}
	text := string(data)
	if strings.HasPrefix(text, "---\n") || strings.HasPrefix(text, "---\r\n") {
		if a, err := agentfile.Parse(data); err == nil {
			text = a.Prompt()
		}
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return "", fmt.Errorf("%s has no text in it", path)
	}
	return text + "\n", nil
}

// cleanPath turns what a terminal pastes into a usable path.
func cleanPath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.Trim(p, `'"`)
	p = strings.ReplaceAll(p, `\ `, " ")
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			p = filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}
	return p
}

// SetPrompt replaces the prompt of one agent of a stored preset.
func (w Workspace) SetPrompt(presetName, agent, prompt string) error {
	if presetName == "initial" {
		return ErrProtected
	}
	b, err := w.Store().Load(presetName)
	if err != nil {
		return err
	}
	for i, file := range b.Agents {
		if file.Name != agent {
			continue
		}
		a, err := agentfile.Parse(file.Data)
		if err != nil {
			return fmt.Errorf("agent %s: %w", agent, err)
		}
		a.SetBody(prompt)
		data, err := a.Marshal()
		if err != nil {
			return err
		}
		b.Agents[i].Data = data
		return w.Save(*b)
	}
	return fmt.Errorf("%w: preset %s has no agent %s", ErrNoAgent, presetName, agent)
}

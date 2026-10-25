// Package project reads and writes the `.squad-up` file that pins a preset to
// a project folder.
package project

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/EdrenaVosh/squad-up/internal/fsx"
	"github.com/EdrenaVosh/squad-up/internal/preset"
)

// FileName is the name of the pin file.
const FileName = ".squad-up"

// Pin is a preset pinned to a project.
type Pin struct {
	Preset string
	File   string // the path of the .squad-up file
}

// Parse reads the preset name from the content of a pin file: the first line
// that is not blank or a # comment.
func Parse(data []byte) (string, error) {
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if line == "initial" {
			return "", errors.New(`"initial" is your own setup and cannot be pinned`)
		}
		if !preset.ValidName(line) {
			return "", fmt.Errorf("%q is not a preset name (lowercase letters, digits and dashes)", line)
		}
		return line, nil
	}
	return "", errors.New("the file names no preset")
}

// Find looks for a .squad-up file in dir and in each folder above it, and
// returns the nearest. It returns nil when there is none.
func Find(dir string) (*Pin, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	for {
		file := filepath.Join(dir, FileName)
		data, err := os.ReadFile(file)
		switch {
		case err == nil:
			name, err := Parse(data)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", file, err)
			}
			return &Pin{Preset: name, File: file}, nil
		case !errors.Is(err, fs.ErrNotExist) && !isDir(file):
			return nil, fmt.Errorf("read %s: %w", file, err)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, nil
		}
		dir = parent
	}
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// Write pins the preset to the folder.
func Write(dir, name string) error {
	if _, err := Parse([]byte(name)); err != nil {
		return err
	}
	return fsx.WriteFileAtomic(filepath.Join(dir, FileName), []byte(name+"\n"), 0o644)
}

// Remove deletes the pin file of the folder and reports whether there was one.
func Remove(dir string) (bool, error) {
	err := os.Remove(filepath.Join(dir, FileName))
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

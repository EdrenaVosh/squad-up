package agentfile

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// File is an agent file found on disk.
type File struct {
	Path string // relative to the scanned dir, slash-separated
	Data []byte
}

// ScanDir reads every *.md file below root, the way Claude Code scans its
// agents folder. Symlinks to files are followed; symlinks to directories and
// dangling links are skipped. The bool reports whether root is a directory.
// Files come back sorted by path and are never parsed.
func ScanDir(root string) ([]File, bool, error) {
	info, err := os.Stat(root)
	if errors.Is(err, fs.ErrNotExist) || (err == nil && !info.IsDir()) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read %s: %w", root, err)
	}

	var files []File
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			if target, err := os.Stat(path); err != nil || target.IsDir() {
				return nil
			}
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, File{Path: filepath.ToSlash(rel), Data: data})
		return nil
	})
	if err != nil {
		return nil, true, fmt.Errorf("read agents in %s: %w", root, err)
	}
	slices.SortFunc(files, func(a, b File) int { return strings.Compare(a.Path, b.Path) })
	return files, true, nil
}

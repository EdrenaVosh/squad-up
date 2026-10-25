// Package fsx holds the file primitives squad relies on: atomic writes,
// hashes, and a safety fuse that keeps test binaries away from real configs.
package fsx

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// guard panics when a test binary is about to modify the developer's real
// ~/.claude or ~/.squad-up. Tests must point CLAUDE_CONFIG_DIR and SQUAD_UP_HOME
// at temporary directories.
func guard(path string) {
	if !testing.Testing() {
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return
	}
	for _, name := range []string{".claude", ".squad-up"} {
		root := filepath.Join(home, name)
		if abs == root || strings.HasPrefix(abs, root+string(filepath.Separator)) {
			panic(fmt.Sprintf("fsx: test attempted to modify real config %s (path %s)", root, abs))
		}
	}
}

// WriteFileAtomic writes data to path through a temp file in the same
// directory and a rename, creating missing parent directories.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) (err error) {
	guard(path)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = os.Remove(tmp.Name())
		}
	}()
	if _, err = tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// MkdirAll is os.MkdirAll behind the safety fuse.
func MkdirAll(path string, perm os.FileMode) error {
	guard(path)
	return os.MkdirAll(path, perm)
}

// Remove is os.Remove behind the safety fuse.
func Remove(path string) error {
	guard(path)
	return os.Remove(path)
}

// RemoveAll is os.RemoveAll behind the safety fuse.
func RemoveAll(path string) error {
	guard(path)
	return os.RemoveAll(path)
}

// Rename is os.Rename behind the safety fuse.
func Rename(oldPath, newPath string) error {
	guard(oldPath)
	guard(newPath)
	return os.Rename(oldPath, newPath)
}

// SHA256Hex returns the hex sha256 of data.
func SHA256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// SHA256File returns the hex sha256 of the file at path.
func SHA256File(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return SHA256Hex(data), nil
}

// ReadFileIfExists reads path; a missing file yields (nil, false, nil).
func ReadFileIfExists(path string) ([]byte, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

// AppendFile appends data to path, creating the file and its parents.
func AppendFile(path string, data []byte, perm os.FileMode) error {
	guard(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

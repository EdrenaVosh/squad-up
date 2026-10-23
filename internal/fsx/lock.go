package fsx

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ErrLocked means another squad process holds the lock.
var ErrLocked = errors.New("another squad is running")

// Lock takes an exclusive lock by creating path. The returned function
// releases it; it removes the file only while it still holds this very lock,
// and calling it again does nothing. A crashed process leaves the file behind,
// which the error message explains how to clear.
func Lock(path string) (unlock func(), err error) {
	guard(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	token := fmt.Sprintf("pid %d since %s", os.Getpid(), time.Now().UTC().Format(time.RFC3339Nano))

	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if errors.Is(err, fs.ErrExist) {
		holder, _ := os.ReadFile(path)
		return nil, fmt.Errorf("%w: %s is held by %q; if no squad is running, delete that file", ErrLocked, path, strings.TrimSpace(string(holder)))
	}
	if err != nil {
		return nil, err
	}
	if _, err := f.WriteString(token); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return nil, err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return nil, err
	}

	var once sync.Once
	return func() {
		once.Do(func() {
			if current, err := os.ReadFile(path); err == nil && string(current) == token {
				_ = os.Remove(path)
			}
		})
	}, nil
}

package fsx_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/EdrenaVosh/squad-up/internal/fsx"
)

func TestLockExcludesASecondHolderUntilReleased(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".lock")

	unlock, err := fsx.Lock(path)
	if err != nil {
		t.Fatalf("Lock() error = %v", err)
	}

	_, err = fsx.Lock(path)
	if !errors.Is(err, fsx.ErrLocked) {
		t.Fatalf("second Lock() error = %v, want ErrLocked", err)
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error %q does not name the lock file", err)
	}

	unlock()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("lock file still exists after unlock: %v", err)
	}
	again, err := fsx.Lock(path)
	if err != nil {
		t.Fatalf("Lock() after unlock error = %v", err)
	}
	again()
}

func TestUnlockTwiceIsHarmless(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".lock")
	unlock, err := fsx.Lock(path)
	if err != nil {
		t.Fatal(err)
	}

	unlock()
	other, err := fsx.Lock(path)
	if err != nil {
		t.Fatal(err)
	}
	unlock() // must not release the lock now held by `other`

	if _, err := fsx.Lock(path); !errors.Is(err, fsx.ErrLocked) {
		t.Fatalf("a stale unlock released someone else's lock: %v", err)
	}
	other()
}

func TestLockCreatesMissingDirectories(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a", "b", ".lock")

	unlock, err := fsx.Lock(path)

	if err != nil {
		t.Fatalf("Lock() error = %v", err)
	}
	unlock()
}

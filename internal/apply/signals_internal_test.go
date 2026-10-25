//go:build !windows

package apply

import (
	"os"
	"syscall"
	"testing"
	"time"
)

// An interrupt during the writes must not end the process half way through:
// it waits until the operation is done.
func TestAnInterruptIsHeldUntilTheWritesAreDone(t *testing.T) {
	release := holdSignals()

	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond) // the test process is still here

	if got := release(); got != syscall.SIGTERM {
		t.Errorf("release() = %v, want the held SIGTERM", got)
	}
}

func TestNothingIsHeldWhenNoSignalCame(t *testing.T) {
	if got := holdSignals()(); got != nil {
		t.Errorf("release() = %v, want nil", got)
	}
}

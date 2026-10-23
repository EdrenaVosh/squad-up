//go:build !windows

package runner_test

import (
	"os"
	"syscall"
	"testing"
	"time"
)

func TestRunForwardsSignalsToClaude(t *testing.T) {
	t.Setenv("FAKE_CLAUDE_SLEEP_MS", "5000")
	go func() {
		time.Sleep(400 * time.Millisecond)
		_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
	}()

	start := time.Now()
	code, _, err := run(t, nil)

	if err != nil {
		t.Fatal(err)
	}
	if code != 143 {
		t.Errorf("exit code = %d, want 143 (128 + SIGTERM)", code)
	}
	if time.Since(start) > 3*time.Second {
		t.Errorf("claude was not stopped: took %v", time.Since(start))
	}
}

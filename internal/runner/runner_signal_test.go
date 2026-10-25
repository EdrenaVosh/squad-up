//go:build !windows

package runner_test

import (
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestRunForwardsSignalsToClaude(t *testing.T) {
	started := filepath.Join(t.TempDir(), "args")
	t.Setenv("FAKE_CLAUDE_SLEEP_MS", "5000")
	t.Setenv("FAKE_CLAUDE_ARGS_FILE", started)

	// The signal must reach claude, not this test: until Run listens for it, a
	// SIGTERM would end the test process, so this test listens too, and sends the
	// signal only once claude has really started (it writes its arguments then).
	own := make(chan os.Signal, 4)
	signal.Notify(own, syscall.SIGTERM)
	defer signal.Stop(own)
	go func() {
		for deadline := time.Now().Add(20 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
			if _, err := os.Stat(started); err == nil {
				break
			}
		}
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
	if time.Since(start) > 15*time.Second {
		t.Errorf("claude was not stopped: took %v", time.Since(start))
	}
}

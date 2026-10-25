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

// sendWhenStarted sends sig to this process once claude has written its
// arguments file (that is, has really started). The test listens for the signal
// too: until Run does, a signal would end the test process itself.
func sendWhenStarted(t *testing.T, started string, sig syscall.Signal) {
	t.Helper()
	own := make(chan os.Signal, 4)
	signal.Notify(own, sig)
	t.Cleanup(func() { signal.Stop(own) })
	go func() {
		for deadline := time.Now().Add(20 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
			if _, err := os.Stat(started); err == nil {
				break
			}
		}
		_ = syscall.Kill(os.Getpid(), sig)
	}()
}

func TestRunForwardsSignalsToClaude(t *testing.T) {
	for sig, want := range map[syscall.Signal]int{syscall.SIGTERM: 143, syscall.SIGHUP: 129} {
		t.Run(sig.String(), func(t *testing.T) {
			started := filepath.Join(t.TempDir(), "args")
			t.Setenv("FAKE_CLAUDE_SLEEP_MS", "5000")
			t.Setenv("FAKE_CLAUDE_ARGS_FILE", started)
			sendWhenStarted(t, started, sig)

			start := time.Now()
			code, _, err := run(t, nil)

			if err != nil {
				t.Fatal(err)
			}
			if code != want {
				t.Errorf("exit code = %d, want %d (128 + signal)", code, want)
			}
			if time.Since(start) > 15*time.Second {
				t.Errorf("claude was not stopped: took %v", time.Since(start))
			}
		})
	}
}

// Ctrl-C reaches claude from the terminal, which sends it to the whole
// foreground group; passing it on as well would interrupt claude twice.
func TestRunDoesNotPassCtrlCOnBecauseTheTerminalAlreadyDid(t *testing.T) {
	started := filepath.Join(t.TempDir(), "args")
	t.Setenv("FAKE_CLAUDE_SLEEP_MS", "1500")
	t.Setenv("FAKE_CLAUDE_ARGS_FILE", started)
	sendWhenStarted(t, started, syscall.SIGINT)

	code, _, err := run(t, nil)

	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Errorf("exit code = %d: claude was interrupted by a signal squad passed on", code)
	}
}

package tui_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/EdrenaVosh/squadup/internal/apply"
	"github.com/EdrenaVosh/squadup/internal/testutil"
	"github.com/EdrenaVosh/squadup/internal/tui"
)

// TestEquipThroughTheRunningProgram drives the real Bubble Tea program: the
// keys of the whole scenario "choose a preset, equip it, close", and checks
// what ended up on disk.
func TestEquipThroughTheRunningProgram(t *testing.T) {
	ws, _ := newWorkspace(t)
	testutil.HideClaude(t)
	tm := teatest.NewTestModel(t, tui.New(tui.Env{Paths: ws.Paths, Now: ws.Now, ClaudeVersion: "2.1.309"}), teatest.WithInitialTermSize(80, 24))

	waitFor := func(text string) {
		t.Helper()
		teatest.WaitFor(t, tm.Output(), func(b []byte) bool { return bytes.Contains(b, []byte(text)) },
			teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(20*time.Millisecond))
	}

	waitFor("PRESETS")
	tm.Send(keyMsg("down"))
	tm.Send(keyMsg("down"))
	waitFor("Solo Indie")
	tm.Send(keyMsg("e"))
	waitFor("[ Equip ]")
	tm.Send(keyMsg("enter"))
	waitFor("Done.")
	tm.Send(keyMsg("x"))
	waitFor("● EQUIPPED")
	tm.Send(keyMsg("q"))
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))

	state, err := apply.LoadState(ws.Paths)
	if err != nil || state.Active != "solo-indie" {
		t.Fatalf("state = %+v, %v", state, err)
	}
	if _, err := os.Stat(filepath.Join(ws.Paths.ClaudeDir, "agents", "planner.md")); err != nil {
		t.Errorf("planner.md was not written: %v", err)
	}
	settings, _ := os.ReadFile(ws.Paths.SettingsFile())
	if !bytes.Contains(settings, []byte(`"model": "sonnet"`)) || !bytes.Contains(settings, []byte(`"theme": "dark"`)) {
		t.Errorf("settings.json = %s", settings)
	}
}

package editor_test

import (
	"reflect"
	"runtime"
	"testing"

	"github.com/EdrenaVosh/squad-up/internal/editor"
)

func TestCommandPrefersVisualOverEditorAndKeepsArguments(t *testing.T) {
	t.Setenv("VISUAL", "code -w --new-window")
	t.Setenv("EDITOR", "nano")

	cmd := editor.Command("/tmp/agent.md")

	want := []string{"code", "-w", "--new-window", "/tmp/agent.md"}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Fatalf("Args = %q, want %q", cmd.Args, want)
	}
}

func TestCommandFallsBackToEditorThenToASystemDefault(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "nano")
	if got := editor.Command("f").Args; !reflect.DeepEqual(got, []string{"nano", "f"}) {
		t.Errorf("EDITOR: Args = %q", got)
	}

	t.Setenv("EDITOR", "")
	want := "vi"
	if runtime.GOOS == "windows" {
		want = "notepad"
	}
	if got := editor.Command("f").Args; !reflect.DeepEqual(got, []string{want, "f"}) {
		t.Errorf("default: Args = %q", got)
	}
}

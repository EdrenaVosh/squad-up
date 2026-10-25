package fsx_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/EdrenaVosh/squad-up/internal/fsx"
)

func TestAppendFileCreatesThenAppends(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "log.jsonl")

	if err := fsx.AppendFile(path, []byte("one\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := fsx.AppendFile(path, []byte("two\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, _ := os.ReadFile(path)
	if string(got) != "one\ntwo\n" {
		t.Fatalf("content = %q", got)
	}
}

func TestAppendFileIntoRealConfigPanicsInTests(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	defer func() {
		if recover() == nil {
			t.Fatal("AppendFile into ~/.squad-up did not panic")
		}
	}()
	_ = fsx.AppendFile(filepath.Join(home, ".squad-up", "log.jsonl"), []byte("x"), 0o600)
}

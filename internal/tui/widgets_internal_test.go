package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestALongErrorInADialogIsWrappedNotCutOff(t *testing.T) {
	p := newPrompt("New preset", "help", "", nil, nil)
	p.err = "already exists: /Users/someone/a/very/long/folder/that/goes/on/and/on/and/on/for/a/while/preset.squad.json and then some more words at the very end"

	view := ansi.Strip(p.View(viewCtx{w: 80, h: 24}))

	if !strings.Contains(view, "already exists") || !strings.Contains(view, "very end") || strings.Contains(view, "…") {
		t.Errorf("the error is cut off:\n%s", view)
	}
}

package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/EdrenaVosh/squadup/internal/claude"
	"github.com/EdrenaVosh/squadup/internal/runner"
	"github.com/EdrenaVosh/squadup/internal/workspace"
)

// tryDoneMsg arrives when the claude started by Try has exited.
type tryDoneMsg struct {
	name string
	err  error
}

// tryCmd starts claude with the preset for this session only, as `squad run`
// does, and hands the terminal over until claude exits.
func tryCmd(ws workspace.Workspace, name string) tea.Cmd {
	bundle, resolved, err := ws.Engine().Resolve(name)
	if err != nil {
		return flashErr(err.Error())
	}
	spec, err := runner.Build(*bundle, resolved)
	if err != nil {
		return flashErr(err.Error())
	}
	bin, err := claude.Find()
	if err != nil {
		return flashErr("claude was not found: install Claude Code and check it with `squad doctor`")
	}
	cmd, cleanup, err := runner.Prepare(bin, spec, nil)
	if err != nil {
		return flashErr(err.Error())
	}
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		cleanup()
		return tryDoneMsg{name: name, err: err}
	})
}

// tryDone is the message the app turns into a status line.
func tryDone(m tryDoneMsg) tea.Msg {
	if m.err != nil {
		return flashMsg{text: fmt.Sprintf("claude with %q ended: %v", m.name, m.err), err: true}
	}
	return flashMsg{text: fmt.Sprintf("Back from claude with %q — nothing was written to ~/.claude", m.name)}
}

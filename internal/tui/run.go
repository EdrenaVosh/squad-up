package tui

import (
	"io"

	tea "charm.land/bubbletea/v2"
)

// Run opens the interface on the terminal and returns when the user quits.
func Run(env Env, in io.Reader, out io.Writer) error {
	program := tea.NewProgram(New(env), tea.WithInput(in), tea.WithOutput(out))
	_, err := program.Run()
	return err
}

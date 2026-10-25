package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// TickMsg is one animation frame, for tests that step the clock by hand.
func TickMsg() tea.Msg { return tickMsg(time.Time{}) }

// UsageTickMsg is one read of the session's transcripts, for tests that drive
// the clock by hand.
func UsageTickMsg() tea.Msg { return usageTickMsg{} }

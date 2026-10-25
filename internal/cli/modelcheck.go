package cli

import (
	"fmt"
	"time"

	"github.com/EdrenaVosh/squad-up/internal/workspace"
)

// workspace is the shared layer for presets, linting and model checks.
func (a *app) workspace() workspace.Workspace {
	return workspace.Workspace{Paths: a.paths, Now: a.now}
}

func age(now, then time.Time) string {
	d := now.Sub(then).Round(time.Minute)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	}
	return fmt.Sprintf("%d h ago", int(d.Hours()))
}

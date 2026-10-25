// Package version carries build metadata injected through -ldflags.
package version

import "fmt"

// Set at build time:
//
//	-X github.com/EdrenaVosh/squad-up/internal/version.Version=...
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// String is the one-line version banner printed by `squad --version`.
func String() string {
	return fmt.Sprintf("squad %s (commit %s, built %s)", Version, Commit, Date)
}

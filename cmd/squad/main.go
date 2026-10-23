// Command squad switches Claude Code agent presets.
package main

import (
	"os"

	"github.com/EdrenaVosh/squadup/internal/cli"
)

func main() {
	os.Exit(cli.Execute(os.Args[1:], os.Stdout, os.Stderr))
}

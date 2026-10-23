// Command fakeclaude is a stand-in for the real claude binary used by tests.
//
// Behaviour is driven by environment variables:
//
//	FAKE_CLAUDE_VERSION    output of `-v` (default "2.1.0 (Claude Code)")
//	FAKE_CLAUDE_LOGGED_IN  "0" makes `auth status` report a logged-out account
//	FAKE_CLAUDE_ARGS_FILE  if set, the received arguments are appended to it
package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	args := os.Args[1:]

	if path := os.Getenv("FAKE_CLAUDE_ARGS_FILE"); path != "" {
		if f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600); err == nil {
			fmt.Fprintln(f, strings.Join(args, "\x1f"))
			f.Close()
		}
	}

	switch {
	case len(args) == 1 && (args[0] == "-v" || args[0] == "--version"):
		version := os.Getenv("FAKE_CLAUDE_VERSION")
		if version == "" {
			version = "2.1.0 (Claude Code)"
		}
		fmt.Println(version)
	case len(args) == 2 && args[0] == "auth" && args[1] == "status":
		configDir := os.Getenv("CLAUDE_CONFIG_DIR")
		if os.Getenv("FAKE_CLAUDE_LOGGED_IN") == "0" {
			fmt.Printf("{\n  \"loggedIn\": false,\n  \"authMethod\": \"none\",\n  \"apiProvider\": \"firstParty\",\n  \"configDirectory\": %q\n}\n", configDir)
			os.Exit(1)
		}
		fmt.Printf("{\n  \"loggedIn\": true,\n  \"authMethod\": \"claude.ai\",\n  \"apiProvider\": \"firstParty\",\n  \"configDirectory\": %q\n}\n", configDir)
	default:
		fmt.Fprintf(os.Stderr, "fakeclaude: unsupported arguments: %v\n", args)
		os.Exit(64)
	}
}

// Command fakeclaude is a stand-in for the real claude binary used by tests.
//
// Behaviour is driven by environment variables:
//
//	FAKE_CLAUDE_VERSION      output of `-v` (default "2.1.0 (Claude Code)")
//	FAKE_CLAUDE_LOGGED_IN    "0" makes `auth status` report a logged-out account
//	FAKE_CLAUDE_ARGS_FILE    if set, the received arguments are appended to it, one
//	                         invocation per line, separated by 0x1f; the content of
//	                         a --settings file is copied to <file>.settings
//	FAKE_CLAUDE_UNAVAILABLE  comma-separated model aliases that a probe rejects
//	FAKE_CLAUDE_ID_<ALIAS>   full model ID a probe reports for the alias
//	FAKE_CLAUDE_SLEEP_MS     delay before answering a probe or a session
//	FAKE_CLAUDE_EXIT         exit code of a session (default 0)
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	args := os.Args[1:]
	logArgs(args)

	switch {
	case len(args) == 1 && (args[0] == "-v" || args[0] == "--version"):
		version := os.Getenv("FAKE_CLAUDE_VERSION")
		if version == "" {
			version = "2.1.0 (Claude Code)"
		}
		fmt.Println(version)
	case len(args) == 2 && args[0] == "auth" && args[1] == "status":
		authStatus()
	case contains(args, "-p"):
		probe(args)
	default:
		session()
	}
}

func authStatus() {
	configDir := os.Getenv("CLAUDE_CONFIG_DIR")
	if os.Getenv("FAKE_CLAUDE_LOGGED_IN") == "0" {
		fmt.Printf("{\n  \"loggedIn\": false,\n  \"authMethod\": \"none\",\n  \"apiProvider\": \"firstParty\",\n  \"configDirectory\": %q\n}\n", configDir)
		os.Exit(1)
	}
	fmt.Printf("{\n  \"loggedIn\": true,\n  \"authMethod\": \"claude.ai\",\n  \"apiProvider\": \"firstParty\",\n  \"configDirectory\": %q\n}\n", configDir)
}

// probe answers `claude -p ... --model X --output-format json`.
func probe(args []string) {
	sleep()
	model := value(args, "--model")
	for _, bad := range strings.Split(os.Getenv("FAKE_CLAUDE_UNAVAILABLE"), ",") {
		if bad != "" && bad == model {
			fmt.Printf("{\"type\":\"result\",\"subtype\":\"success\",\"is_error\":true,\"result\":\"There's an issue with the selected model (%s). It may not exist or you may not have access to it.\",\"modelUsage\":{}}\n", model)
			os.Exit(1)
		}
	}
	id := os.Getenv("FAKE_CLAUDE_ID_" + strings.ToUpper(model))
	switch {
	case id != "":
	case strings.HasPrefix(model, "claude-"): // a release named by its full ID answers as itself
		id = model
	default:
		id = "claude-" + model + "-5-5"
	}
	usage := fmt.Sprintf("%q:{\"inputTokens\":9,\"outputTokens\":2}", id)
	if model != "haiku" {
		// A helper call to another model, as Claude Code makes for small chores.
		usage += ",\"claude-haiku-5-5\":{\"inputTokens\":3,\"outputTokens\":1}"
	}
	fmt.Printf("{\"type\":\"result\",\"subtype\":\"success\",\"is_error\":false,\"result\":\"OK\",\"modelUsage\":{%s}}\n", usage)
}

// session stands for an interactive claude started with squad run.
func session() {
	if path := value(os.Args[1:], "--settings"); path != "" {
		if target := os.Getenv("FAKE_CLAUDE_ARGS_FILE"); target != "" {
			if data, err := os.ReadFile(path); err == nil {
				_ = os.WriteFile(target+".settings", data, 0o600)
			}
		}
	}
	sleep()
	fmt.Println("fake claude session")
	if code, err := strconv.Atoi(os.Getenv("FAKE_CLAUDE_EXIT")); err == nil {
		os.Exit(code)
	}
}

func logArgs(args []string) {
	path := os.Getenv("FAKE_CLAUDE_ARGS_FILE")
	if path == "" {
		return
	}
	if f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600); err == nil {
		fmt.Fprintln(f, strings.Join(args, "\x1f"))
		f.Close()
	}
}

func sleep() {
	if ms, err := strconv.Atoi(os.Getenv("FAKE_CLAUDE_SLEEP_MS")); err == nil {
		time.Sleep(time.Duration(ms) * time.Millisecond)
	}
}

func contains(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

func value(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

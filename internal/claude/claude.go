// Package claude locates the Claude Code binary and queries it.
package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// ErrNotFound means no `claude` executable is on PATH.
var ErrNotFound = errors.New("claude binary not found on PATH")

// Find returns the path to the `claude` executable on PATH.
func Find() (string, error) {
	path, err := exec.LookPath("claude")
	if err != nil {
		return "", ErrNotFound
	}
	return path, nil
}

// Client runs commands of one claude binary.
type Client struct {
	Bin string
}

// AuthStatus is the JSON printed by `claude auth status`.
type AuthStatus struct {
	LoggedIn        bool   `json:"loggedIn"`
	AuthMethod      string `json:"authMethod"`
	APIProvider     string `json:"apiProvider"`
	ConfigDirectory string `json:"configDirectory"`
}

// Version returns the trimmed output of `claude -v`.
func (c Client) Version(ctx context.Context) (string, error) {
	out, err := c.run(ctx, "-v")
	if err != nil {
		return "", fmt.Errorf("run claude -v: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// Auth runs `claude auth status`. The command exits with code 1 when the user
// is logged out but still prints JSON, so output is parsed regardless of the
// exit code; an error is returned only when no status could be read.
func (c Client) Auth(ctx context.Context) (AuthStatus, error) {
	out, runErr := c.run(ctx, "auth", "status")
	var status AuthStatus
	if err := json.Unmarshal(out, &status); err != nil {
		if runErr != nil {
			return AuthStatus{}, fmt.Errorf("run claude auth status: %w", runErr)
		}
		return AuthStatus{}, fmt.Errorf("parse claude auth status output: %w", err)
	}
	return status, nil
}

func (c Client) run(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, c.Bin, args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	err := cmd.Run()
	return stdout.Bytes(), err
}

// probeResult is the part of `claude -p --output-format json` that a probe reads.
type probeResult struct {
	IsError    bool                       `json:"is_error"`
	Result     string                     `json:"result"`
	Errors     []string                   `json:"errors"`
	ModelUsage map[string]json.RawMessage `json:"modelUsage"`
}

// Probe sends one minimal request on the given model alias and returns the
// full model ID it ran on. The reason is the error when the model cannot be
// used. The request runs outside any project and is not saved as a session.
func (c Client) Probe(ctx context.Context, model string) (string, error) {
	cmd := exec.CommandContext(ctx, c.Bin, "-p", "Reply with OK", "--model", model, "--output-format", "json", "--no-session-persistence")
	cmd.Dir = os.TempDir()
	cmd.WaitDelay = time.Second
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	runErr := cmd.Run()

	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "", fmt.Errorf("the probe of %s timed out", model)
	}
	if ctx.Err() != nil {
		return "", fmt.Errorf("the probe of %s was cancelled", model)
	}

	var res probeResult
	parsed := json.Unmarshal(stdout.Bytes(), &res) == nil
	if runErr != nil || (parsed && res.IsError) {
		return "", errors.New(probeReason(res, parsed, stderr.String(), runErr))
	}
	if !parsed {
		return "", fmt.Errorf("could not read the answer of claude: %q", firstLine(stdout.String()))
	}
	if id := pickModelID(res.ModelUsage, model); id != "" {
		return id, nil
	}
	return "", errors.New("claude answered but did not say which model it used")
}

func probeReason(res probeResult, parsed bool, stderr string, runErr error) string {
	switch {
	case parsed && res.Result != "":
		return firstLine(res.Result)
	case parsed && len(res.Errors) > 0:
		return firstLine(res.Errors[0])
	case strings.TrimSpace(stderr) != "":
		return firstLine(stderr)
	case runErr != nil:
		return runErr.Error()
	}
	return "claude reported an error"
}

// pickModelID chooses the model the request was made on. modelUsage also
// lists helper models Claude Code used on the side, so the key that mentions
// the requested alias wins.
func pickModelID(usage map[string]json.RawMessage, alias string) string {
	alias = strings.ToLower(strings.TrimSuffix(alias, "[1m]"))
	ids := make([]string, 0, len(usage))
	for id := range usage {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if strings.Contains(strings.ToLower(id), alias) {
			return id
		}
	}
	if len(ids) > 0 {
		return ids[0]
	}
	return ""
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return line
}

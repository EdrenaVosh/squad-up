// Package claude locates the Claude Code binary and queries it.
package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
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

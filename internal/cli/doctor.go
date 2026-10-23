package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/EdrenaVosh/squadup/internal/claude"
)

const doctorTimeout = 15 * time.Second

func newDoctor() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check that Claude Code is installed and logged in",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), doctorTimeout)
			defer cancel()
			return runDoctor(ctx, cmd.OutOrStdout())
		},
	}
}

func runDoctor(ctx context.Context, out io.Writer) error {
	bin, err := claude.Find()
	if errors.Is(err, claude.ErrNotFound) {
		report(out, false, "claude", "not found on PATH")
		return &ExitError{
			Code: 1,
			Err:  errors.New("claude is not installed or not on PATH"),
			Fix:  "install Claude Code (https://code.claude.com/docs) and make sure `claude` runs in this shell",
		}
	}
	if err != nil {
		return err
	}
	report(out, true, "claude", bin)

	c := claude.Client{Bin: bin}

	version, err := c.Version(ctx)
	if err != nil {
		report(out, false, "version", err.Error())
		return &ExitError{Code: 1, Err: err, Fix: "run `claude -v` yourself to see why it fails"}
	}
	report(out, true, "version", version)

	auth, err := c.Auth(ctx)
	if err != nil {
		report(out, false, "login", err.Error())
		return &ExitError{Code: 1, Err: err, Fix: "run `claude auth status` yourself to see why it fails"}
	}
	if !auth.LoggedIn {
		report(out, false, "login", "not logged in")
		if auth.ConfigDirectory != "" {
			report(out, true, "config dir", auth.ConfigDirectory)
		}
		return &ExitError{
			Code: 1,
			Err:  errors.New("claude is not logged in"),
			Fix:  "run `claude auth login` (login is stored per config directory, so a custom CLAUDE_CONFIG_DIR needs its own login)",
		}
	}
	report(out, true, "login", fmt.Sprintf("logged in via %s (%s)", auth.AuthMethod, auth.APIProvider))
	if auth.ConfigDirectory != "" {
		report(out, true, "config dir", auth.ConfigDirectory)
	}
	return nil
}

func report(out io.Writer, ok bool, name, detail string) {
	mark := "✓"
	if !ok {
		mark = "✗"
	}
	printf(out, "%s %-11s %s\n", mark, name, detail)
}

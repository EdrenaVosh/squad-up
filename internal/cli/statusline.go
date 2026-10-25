package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tidwall/gjson"

	"github.com/EdrenaVosh/squad-up/internal/fsx"
	"github.com/EdrenaVosh/squad-up/internal/runner"
	"github.com/EdrenaVosh/squad-up/internal/statusline"
)

// statuslineTimeout bounds the user's own status line command.
const statuslineTimeout = 4 * time.Second

func (a *app) newStatusline() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "statusline",
		Short: "Show the equipped preset in the Claude Code status line (enable, disable, status)",
		Long: "Without a subcommand this is the command Claude Code runs to draw its status line: it runs your own status line\n" +
			"command first, if you had one, and appends ◆ and the name of the equipped preset.\n" +
			"`enable` installs it, `disable` brings your own line back.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			a.drawStatusline(cmd.Context())
			return nil
		},
	}
	cmd.AddCommand(a.newStatuslineEnable(), a.newStatuslineDisable(), a.newStatuslineStatus())
	return cmd
}

// drawStatusline prints the line. It never fails: a blank status line is worse
// than a plain one.
func (a *app) drawStatusline(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, statuslineTimeout)
	defer cancel()

	var stdin []byte
	if !a.io.Interactive {
		stdin, _ = io.ReadAll(io.LimitReader(a.io.In, 1<<20))
	}
	line := statusline.Render(ctx, a.paths, statusline.Input{
		Stdin:     stdin,
		RunPreset: os.Getenv(runner.EnvPreset),
		Color:     os.Getenv("NO_COLOR") == "" && !a.noColor,
	})
	if line != "" {
		printf(a.io.Out, "%s\n", line)
	}
}

// statuslineCommand is the command Claude Code is told to run.
func (a *app) statuslineCommand() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("find the squad binary: %w", err)
	}
	command := shellQuote(exe) + " statusline"
	if home, err := os.UserHomeDir(); err == nil && a.paths.Home != filepath.Join(home, ".squad-up") {
		command += " --home " + shellQuote(a.paths.Home)
	}
	return command, nil
}

func shellQuote(s string) string {
	if runtime.GOOS == "windows" {
		return `"` + s + `"`
	}
	return `'` + strings.ReplaceAll(s, `'`, `'\''`) + `'`
}

func (a *app) newStatuslineEnable() *cobra.Command {
	return &cobra.Command{
		Use:   "enable",
		Short: "Add the equipped preset to your Claude Code status line",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			command, err := a.statuslineCommand()
			if err != nil {
				return &ExitError{Code: 1, Err: err, Fix: "run squad from its installed location"}
			}
			outcome, err := statusline.Enable(a.paths, command, a.now())
			if err != nil {
				return statuslineError(err)
			}
			switch outcome {
			case statusline.AlreadyEnabled:
				printf(a.io.Out, "The squad status line is already enabled.\n")
				return nil
			case statusline.Refreshed:
				printf(a.io.Out, "The squad status line was already enabled; its command now points at this squad.\n")
				return nil
			}
			printf(a.io.Out, "Status line enabled: Claude Code will show ◆ and the equipped preset.\n")
			if rec, err := statusline.Load(a.paths); err == nil && rec != nil {
				if original := gjson.GetBytes(rec.Original, "command").String(); original != "" {
					printf(a.io.Out, "Your own status line (%s) is kept and runs first.\n", original)
				}
			}
			printf(a.io.Out, "A new Claude Code session shows it. Undo with `squad statusline disable`.\n")
			return nil
		},
	}
}

func (a *app) newStatuslineDisable() *cobra.Command {
	return &cobra.Command{
		Use:   "disable",
		Short: "Take the preset out of the status line and bring your own line back",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			rec, _ := statusline.Load(a.paths)
			outcome, err := statusline.Disable(a.paths)
			if err != nil {
				return statuslineError(err)
			}
			switch {
			case outcome == statusline.NotOurs:
				printf(a.io.Out, "Your status line was changed after it was enabled, so it is left as it is. The squad record was removed.\n")
			case rec != nil && len(rec.Original) > 0:
				printf(a.io.Out, "Status line disabled: your own line is back.\n")
			default:
				printf(a.io.Out, "Status line disabled: the line squad added was removed.\n")
			}
			return nil
		},
	}
}

func (a *app) newStatuslineStatus() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Say whether the squad status line is enabled",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			rec, err := statusline.Load(a.paths)
			if err != nil {
				return statuslineError(err)
			}
			if rec == nil {
				printf(a.io.Out, "The squad status line is not enabled. `squad statusline enable` turns it on.\n")
				return nil
			}
			printf(a.io.Out, "The squad status line is enabled (since %s).\n", rec.EnabledAt.Local().Format("2006-01-02 15:04"))
			if original := gjson.GetBytes(rec.Original, "command").String(); original != "" {
				printf(a.io.Out, "Your own line, run first: %s\n", original)
			}
			if gjson.GetBytes(mustRead(a.paths.SettingsFile()), "statusLine.command").String() != gjson.GetBytes(rec.Installed, "command").String() {
				printf(a.io.Out, "! settings.json no longer has it (it was restored or edited). `squad statusline enable` installs it again.\n")
			}
			return nil
		},
	}
}

func mustRead(path string) []byte {
	data, _, _ := fsx.ReadFileIfExists(path)
	return data
}

func statuslineError(err error) error {
	switch {
	case errors.Is(err, statusline.ErrNotEnabled):
		return &ExitError{Code: 1, Err: err, Fix: "turn it on with `squad statusline enable`"}
	case errors.Is(err, fsx.ErrLocked):
		return &ExitError{Code: 1, Err: err, Fix: "wait for the other squad to finish"}
	}
	return &ExitError{Code: 1, Err: err, Fix: "check settings.json in your Claude config folder"}
}

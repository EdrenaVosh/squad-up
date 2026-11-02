package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/EdrenaVosh/squad-up/internal/updatecheck"
	"github.com/EdrenaVosh/squad-up/internal/version"
)

const installScript = "curl -fsSL https://raw.githubusercontent.com/EdrenaVosh/squad-up/main/scripts/install.sh | sh"

const changelogURL = "https://github.com/EdrenaVosh/squad-up/blob/main/CHANGELOG.md"

// advice is how a copy of squad is updated.
type advice struct {
	how     string   // how it was installed, such as "Homebrew"
	command string   // the command to show
	argv    []string // the same command to run; nil when squad must not run it itself
}

// updateAdvice says how a program installed at path is updated. It looks at
// where the file is, because that tells how it got there. A build from source
// or an unknown install is only told what to run: pulling a checkout or
// installing a second copy is not squad's to decide.
func updateAdvice(exe string) advice {
	slashed := filepath.ToSlash(exe)
	switch {
	case strings.Contains(slashed, "/Cellar/"), strings.Contains(slashed, "/Caskroom/"), strings.Contains(slashed, "/homebrew/"), strings.Contains(slashed, "/linuxbrew/"):
		return advice{"Homebrew", "brew upgrade EdrenaVosh/tap/squad-up", []string{"brew", "upgrade", "EdrenaVosh/tap/squad-up"}}
	case strings.Contains(slashed, "/go/bin/"):
		return advice{"go install", "go install github.com/EdrenaVosh/squad-up/cmd/squad@latest", []string{"go", "install", "github.com/EdrenaVosh/squad-up/cmd/squad@latest"}}
	case strings.HasSuffix(path.Dir(slashed), "/.local/bin"):
		return advice{"the install script", installScript, []string{"sh", "-c", installScript}}
	case strings.HasSuffix(path.Dir(slashed), "/bin") && strings.Contains(slashed, "squad-up/bin"):
		return advice{"a build from source", "git pull && make install", nil}
	}
	return advice{"an unknown source", installScript, nil}
}

// executable is the path of the running program, with links followed.
func executable() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		return real
	}
	return exe
}

func (a *app) newUpdate() *cobra.Command {
	var printOnly bool
	var autoCheck string
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update squad to the newest release",
		Long: "Updates this copy of squad the way it was installed, judging by where it is: brew upgrade,\n" +
			"go install or the install script. A build from source or an unknown install is only told\n" +
			"which command to run. With --print, squad only shows the command.\n\n" +
			"squad also looks for a new release on GitHub once a day, from a terminal only, and says\n" +
			"so on the main screen and after commands. Turn that off with --auto-check off, or by\n" +
			"setting " + updatecheck.EnvOff + "=1.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c := a.checker()
			if cmd.Flags().Changed("auto-check") {
				return a.setAutoCheck(c, autoCheck)
			}
			adv := updateAdvice(executable())
			printf(a.io.Out, "You have %s, installed with %s.\n\n", version.String(), adv.how)
			if updatecheck.Release(version.Version) {
				latest, err := c.Latest(context.Background())
				switch {
				case err != nil:
					printf(a.io.Out, "GitHub could not be asked for the newest release (%v).\n\n", err)
				case !updatecheck.Newer(latest, version.Version):
					printf(a.io.Out, "This is the newest release.\n")
					a.autoCheckLine(c)
					return nil
				default:
					printf(a.io.Out, "squad %s is out.\n\n", latest)
				}
			}
			if printOnly || adv.argv == nil {
				printf(a.io.Out, "To update, run:\n  %s\n\nWhat is new: %s\n", adv.command, changelogURL)
				a.autoCheckLine(c)
				return nil
			}
			printf(a.io.Out, "Updating: %s\n\n", adv.command)
			run := exec.Command(adv.argv[0], adv.argv[1:]...)
			run.Stdin, run.Stdout, run.Stderr = a.io.In, a.io.Out, a.io.Err
			if err := run.Run(); err != nil {
				return &ExitError{Code: 1, Err: err, Fix: "run it yourself: " + adv.command}
			}
			printf(a.io.Out, "\nUpdated. What is new: %s\n", changelogURL)
			return nil
		},
	}
	cmd.Flags().BoolVar(&printOnly, "print", false, "only show the command that updates squad")
	cmd.Flags().StringVar(&autoCheck, "auto-check", "", "turn the daily check for a new release on or off")
	return cmd
}

// setAutoCheck turns the daily check on or off.
func (a *app) setAutoCheck(c updatecheck.Checker, value string) error {
	var off bool
	switch value {
	case "on":
	case "off":
		off = true
	default:
		return &ExitError{Code: 2, Err: fmt.Errorf("--auto-check takes on or off, not %q", value), Fix: autoCheckOff}
	}
	if err := c.SetOff(off); err != nil {
		return &ExitError{Code: 1, Err: fmt.Errorf("remember the setting: %w", err), Fix: "set " + updatecheck.EnvOff + "=1 in your shell instead"}
	}
	if off {
		printf(a.io.Out, "squad no longer looks for new releases. Turn it back on: %s\n", autoCheckOn)
		return nil
	}
	printf(a.io.Out, "squad looks for a new release once a day again.\n")
	if os.Getenv(updatecheck.EnvOff) != "" {
		printf(a.io.Out, "%s is set in your shell, though, and keeps it off.\n", updatecheck.EnvOff)
	}
	return nil
}

// autoCheckLine says whether the daily check is on, and how to change that.
func (a *app) autoCheckLine(c updatecheck.Checker) {
	if c.Enabled() {
		printf(a.io.Out, "\nsquad looks for a new release once a day. Turn it off: %s\n", autoCheckOff)
	} else if updatecheck.Release(version.Version) {
		printf(a.io.Out, "\nThe daily check for a new release is off. Turn it on: %s\n", autoCheckOn)
	}
}

package cli

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/EdrenaVosh/squad-up/internal/version"
)

const installScript = "curl -fsSL https://raw.githubusercontent.com/EdrenaVosh/squad-up/main/scripts/install.sh | sh"

// updateAdvice says how a program installed at path is updated. It looks at
// where the file is, because that tells how it got there.
func updateAdvice(path string) (how, command string) {
	slashed := filepath.ToSlash(path)
	switch {
	case strings.Contains(slashed, "/Cellar/"), strings.Contains(slashed, "/Caskroom/"), strings.Contains(slashed, "/homebrew/"), strings.Contains(slashed, "/linuxbrew/"):
		return "Homebrew", "brew upgrade EdrenaVosh/tap/squad-up"
	case strings.Contains(slashed, "/go/bin/"):
		return "go install", "go install github.com/EdrenaVosh/squad-up/cmd/squad@latest"
	case strings.HasSuffix(filepath.Dir(slashed), "/.local/bin"):
		return "the install script", installScript
	case strings.HasSuffix(filepath.Dir(slashed), "/bin") && strings.Contains(slashed, "squad-up/bin"):
		return "a build from source", "git pull && make install"
	}
	return "an unknown source", installScript
}

func (a *app) newUpdate() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Show how to update squad to the newest release",
		Long: "Says which command updates this copy of squad, judging by where it is installed.\n" +
			"It downloads nothing and changes nothing: squad never contacts the network on its own.",
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			exe, err := os.Executable()
			if err == nil {
				if real, err := filepath.EvalSymlinks(exe); err == nil {
					exe = real
				}
			}
			how, command := updateAdvice(exe)
			printf(a.io.Out, "You have %s, installed with %s.\n\nTo update, run:\n  %s\n\nWhat is new: https://github.com/EdrenaVosh/squad-up/blob/main/CHANGELOG.md\n",
				version.String(), how, command)
			return nil
		},
	}
}

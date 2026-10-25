package cli

import (
	"strings"
	"testing"
)

func TestUpdateAdviceFollowsWhereTheProgramIs(t *testing.T) {
	cases := map[string]string{
		"/opt/homebrew/Cellar/squad-up/0.1.0/bin/squad": "brew upgrade",
		"/opt/homebrew/Caskroom/squad-up/0.1.0/squad":   "brew upgrade",
		"/home/me/go/bin/squad":                         "go install",
		"/home/me/.local/bin/squad":                     "install.sh",
		"/Users/me/dev/squad-up/bin/squad":              "make install",
		"/usr/local/bin/squad":                          "install.sh",
	}
	for path, want := range cases {
		if _, command := updateAdvice(path); !strings.Contains(command, want) {
			t.Errorf("%s: %q does not contain %q", path, command, want)
		}
	}
}

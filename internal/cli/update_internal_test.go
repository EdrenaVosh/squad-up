package cli

import (
	"strings"
	"testing"
)

func TestUpdateAdviceFollowsWhereTheProgramIs(t *testing.T) {
	cases := []struct {
		path, want string
		runs       bool // squad runs the command itself
	}{
		{"/opt/homebrew/Cellar/squad-up/0.1.0/bin/squad", "brew upgrade", true},
		{"/opt/homebrew/Caskroom/squad-up/0.1.0/squad", "brew upgrade", true},
		{"/home/me/go/bin/squad", "go install", true},
		{"/home/me/.local/bin/squad", "install.sh", true},
		{"/Users/me/dev/squad-up/bin/squad", "make install", false},
		{"/usr/local/bin/squad", "install.sh", false},
	}
	for _, c := range cases {
		adv := updateAdvice(c.path)
		if !strings.Contains(adv.command, c.want) || (adv.argv != nil) != c.runs {
			t.Errorf("%s: %+v, want a command with %q, run by squad: %v", c.path, adv, c.want, c.runs)
		}
		if adv.argv != nil && !strings.Contains(strings.Join(adv.argv, " "), c.want) {
			t.Errorf("%s: runs %q, not the command it shows (%q)", c.path, adv.argv, adv.command)
		}
	}
}

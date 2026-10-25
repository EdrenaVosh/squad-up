package cli_test

import (
	"strings"
	"testing"
)

func TestUpdateSaysHowAndTouchesNothing(t *testing.T) {
	e := newEnv(t)

	r := e.run("update")

	if r.code != 0 || !strings.Contains(r.stdout, "To update, run:") || !strings.Contains(r.stdout, "CHANGELOG.md") {
		t.Errorf("%d %q %q", r.code, r.stdout, r.stderr)
	}
}

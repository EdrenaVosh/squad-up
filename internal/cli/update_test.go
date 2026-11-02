package cli_test

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/EdrenaVosh/squad-up/internal/updatecheck"
	"github.com/EdrenaVosh/squad-up/internal/version"
)

// release makes the running squad the given release for the test.
func release(t *testing.T, v string) {
	t.Helper()
	old := version.Version
	version.Version = v
	t.Cleanup(func() { version.Version = old })
	t.Setenv(updatecheck.EnvOff, "")
	t.Setenv(updatecheck.EnvFake, "")
}

// github answers every request with the newest release being tag, and counts
// the requests.
func github(tag string) (*http.Client, *int) {
	asked := 0
	return &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		asked++
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: http.Header{},
			Body: io.NopCloser(strings.NewReader(`{"tag_name":"` + tag + `"}`)), Request: r}, nil
	})}, &asked
}

// The test binary is in no known install location, so update says what to
// run and runs nothing.
func TestUpdateSaysWhatToRunForANewerRelease(t *testing.T) {
	release(t, "0.2.0")
	e := newEnv(t)
	e.http, _ = github("v0.3.0")

	r := e.run("update")

	for _, want := range []string{"squad 0.3.0 is out.", "To update, run:", "CHANGELOG.md", "Turn it off: squad update --auto-check off"} {
		if r.code != 0 || !strings.Contains(r.stdout, want) {
			t.Errorf("no %q: %d %q %q", want, r.code, r.stdout, r.stderr)
		}
	}
}

func TestUpdateOnTheNewestReleaseDoesNothing(t *testing.T) {
	release(t, "0.2.0")
	e := newEnv(t)
	e.http, _ = github("v0.2.0")

	r := e.run("update")

	if r.code != 0 || !strings.Contains(r.stdout, "This is the newest release.") || strings.Contains(r.stdout, "To update") {
		t.Errorf("%d %q %q", r.code, r.stdout, r.stderr)
	}
}

func TestUpdateWithoutGitHubStillSaysHow(t *testing.T) {
	release(t, "0.2.0")
	e := newEnv(t)

	r := e.run("update")

	if r.code != 0 || !strings.Contains(r.stdout, "GitHub could not be asked") || !strings.Contains(r.stdout, "To update, run:") {
		t.Errorf("%d %q %q", r.code, r.stdout, r.stderr)
	}
}

func TestUpdatePrintOnlyShowsTheCommand(t *testing.T) {
	release(t, "0.2.0")
	e := newEnv(t)
	e.http, _ = github("v0.3.0")

	r := e.run("update", "--print")

	if r.code != 0 || !strings.Contains(r.stdout, "To update, run:") || strings.Contains(r.stdout, "Updating:") {
		t.Errorf("%d %q %q", r.code, r.stdout, r.stderr)
	}
}

func TestADevelopmentBuildDoesNotAskGitHub(t *testing.T) {
	release(t, "dev")
	e := newEnv(t)
	var asked *int
	e.http, asked = github("v0.3.0")

	r := e.run("update")

	if r.code != 0 || *asked != 0 || !strings.Contains(r.stdout, "To update, run:") {
		t.Errorf("asked %d times: %d %q", *asked, r.code, r.stdout)
	}
}

func TestTheDailyCheckIsTurnedOffAndOn(t *testing.T) {
	release(t, "0.2.0")
	e := newEnv(t)
	e.http, _ = github("v0.2.0")

	r := e.run("update", "--auto-check", "off")
	if r.code != 0 || !strings.Contains(r.stdout, "no longer looks for new releases") || !strings.Contains(r.stdout, "--auto-check on") {
		t.Fatalf("%d %q %q", r.code, r.stdout, r.stderr)
	}
	data, err := os.ReadFile(filepath.Join(e.home, "update-check.json"))
	if err != nil || !strings.Contains(string(data), `"off": true`) {
		t.Fatalf("the setting was not kept: %q %v", data, err)
	}
	if r := e.run("update"); !strings.Contains(r.stdout, "The daily check for a new release is off. Turn it on: squad update --auto-check on") {
		t.Errorf("update does not say the check is off: %q", r.stdout)
	}

	r = e.run("update", "--auto-check", "on")
	if r.code != 0 || !strings.Contains(r.stdout, "once a day again") {
		t.Fatalf("%d %q %q", r.code, r.stdout, r.stderr)
	}
	if r := e.run("update", "--auto-check", "maybe"); r.code != 2 {
		t.Errorf("--auto-check maybe: exit %d", r.code)
	}
}

func TestCommandsInAPipeNeverAskGitHub(t *testing.T) {
	release(t, "0.2.0")
	e := ready(t)
	var asked *int
	e.http, asked = github("v0.3.0")

	r := e.run("list")

	if r.code != 0 || *asked != 0 || strings.Contains(r.stderr, "is out") {
		t.Errorf("asked %d times: %q", *asked, r.stderr)
	}
}

func TestTheInterfaceChecksInTheBackgroundAndTellsOnce(t *testing.T) {
	release(t, "0.2.0")
	e := ready(t)
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("NO_COLOR", "")
	t.Setenv("CI", "")
	var asked *int
	e.http, asked = github("v0.3.0")

	r := e.runOnTerminal(nil)

	if r.env.CheckUpdate == nil || *asked != 0 {
		t.Fatalf("no background check, or GitHub asked before the interface opened (%d)", *asked)
	}
	if !strings.Contains(r.env.Note, "turn off: squad update --auto-check off") {
		t.Errorf("the first time, the interface is not told to say how to turn the check off: %q", r.env.Note)
	}
	if u := r.env.CheckUpdate(); u == nil || u.Latest != "0.3.0" || *asked != 1 {
		t.Errorf("the check found %+v, asking %d times", u, *asked)
	}
	if again := e.runOnTerminal(nil); again.env.Note != "" {
		t.Errorf("told twice: %q", again.env.Note)
	}
}

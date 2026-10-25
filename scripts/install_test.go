//go:build !windows

package scripts_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// run executes install.sh with a fake curl that answers the lookup of the latest
// release with the given final URL, and fails every download.
func run(t *testing.T, finalURL string) (string, error) {
	t.Helper()
	bin := t.TempDir()
	fake := "#!/bin/sh\ncase \"$*\" in *url_effective*) printf '%s' '" + finalURL + "' ;; *) exit 22 ;; esac\n"
	if err := os.WriteFile(filepath.Join(bin, "curl"), []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", "install.sh")
	cmd.Env = []string{"PATH=" + bin + ":/usr/bin:/bin", "HOME=" + t.TempDir(), "SQUAD_UP_INSTALL_DIR=" + t.TempDir()}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestWithoutAReleaseTheScriptSaysSo(t *testing.T) {
	// GitHub answers /releases/latest with a redirect to /releases when there is none.
	for _, url := range []string{
		"https://github.com/EdrenaVosh/squad-up/releases",
		"https://github.com/EdrenaVosh/squad-up/releases/latest",
	} {
		out, err := run(t, url)
		if err == nil || !strings.Contains(out, "there is no release yet") {
			t.Errorf("%s: err = %v, output %q", url, err, out)
		}
	}
}

func TestAReleaseTagIsTakenAsTheVersion(t *testing.T) {
	out, err := run(t, "https://github.com/EdrenaVosh/squad-up/releases/tag/v0.1.0")

	if err == nil || !strings.Contains(out, "Downloading squad-up 0.1.0") {
		t.Errorf("err = %v, output %q: want it to go on to download 0.1.0 (the fake curl then fails)", err, out)
	}
}

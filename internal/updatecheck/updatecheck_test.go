package updatecheck_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/EdrenaVosh/squad-up/internal/updatecheck"
)

var start = time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)

// github answers with the tag and counts the questions.
func github(t *testing.T, tag string, status int) (*httptest.Server, *int) {
	t.Helper()
	asked := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked++
		if r.Header.Get("User-Agent") == "" {
			t.Error("no User-Agent: GitHub refuses requests without one")
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"tag_name":"` + tag + `"}`))
	}))
	t.Cleanup(srv.Close)
	return srv, &asked
}

func checker(t *testing.T, url, current string, now *time.Time) updatecheck.Checker {
	t.Helper()
	t.Setenv(updatecheck.EnvOff, "")
	t.Setenv(updatecheck.EnvFake, "")
	return updatecheck.Checker{
		File: filepath.Join(t.TempDir(), "update-check.json"), Current: current, URL: url,
		Now: func() time.Time { return *now },
	}
}

func TestNewer(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"0.3.0", "0.2.0", true},
		{"v0.2.1", "0.2.0", true},
		{"1.0.0", "0.9.9", true},
		{"0.2.0", "0.2.0", false},
		{"0.1.9", "0.2.0", false},
		{"0.10.0", "0.9.0", true},
		{"0.3.0", "dev", false},
		{"0.3.0", "0.1.0-3-g42e4695-dirty", false},
		{"", "0.2.0", false},
	}
	for _, c := range cases {
		if got := updatecheck.Newer(c.latest, c.current); got != c.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", c.latest, c.current, got, c.want)
		}
	}
}

func TestAsksOnceADayAndRemembersTheAnswer(t *testing.T) {
	srv, asked := github(t, "v0.3.0", http.StatusOK)
	now := start
	c := checker(t, srv.URL, "0.2.0", &now)

	if c.Known() != "" {
		t.Fatal("a release is known before any check")
	}
	c.Refresh(context.Background())
	if *asked != 1 || c.Known() != "0.3.0" {
		t.Fatalf("asked %d times, known %q", *asked, c.Known())
	}

	now = start.Add(23 * time.Hour)
	c.Refresh(context.Background())
	if *asked != 1 {
		t.Fatalf("asked again within a day: %d", *asked)
	}

	now = start.Add(25 * time.Hour)
	c.Refresh(context.Background())
	if *asked != 2 {
		t.Fatalf("not asked again after a day: %d", *asked)
	}
}

func TestTheNewestReleaseIsNotAnnounced(t *testing.T) {
	srv, _ := github(t, "v0.2.0", http.StatusOK)
	now := start
	c := checker(t, srv.URL, "0.2.0", &now)

	c.Refresh(context.Background())

	if got := c.Known(); got != "" {
		t.Errorf("the running release is announced as new: %q", got)
	}
}

func TestOfflineWaitsADayToo(t *testing.T) {
	srv, asked := github(t, "", http.StatusInternalServerError)
	now := start
	c := checker(t, srv.URL, "0.2.0", &now)

	c.Refresh(context.Background())
	now = start.Add(time.Hour)
	c.Refresh(context.Background())

	if *asked != 1 || c.Known() != "" {
		t.Errorf("asked %d times after a failure, known %q", *asked, c.Known())
	}
}

func TestTurnedOffAsksNothing(t *testing.T) {
	srv, asked := github(t, "v0.3.0", http.StatusOK)
	now := start

	byEnv := checker(t, srv.URL, "0.2.0", &now)
	t.Setenv(updatecheck.EnvOff, "1")
	byEnv.Refresh(context.Background())
	if *asked != 0 || byEnv.Enabled() {
		t.Fatalf("%s=1: asked %d times", updatecheck.EnvOff, *asked)
	}

	bySetting := checker(t, srv.URL, "0.2.0", &now)
	if err := bySetting.SetOff(true); err != nil {
		t.Fatal(err)
	}
	bySetting.Refresh(context.Background())
	if *asked != 0 || bySetting.Known() != "" {
		t.Fatalf("turned off: asked %d times", *asked)
	}
	if err := bySetting.SetOff(false); err != nil {
		t.Fatal(err)
	}
	bySetting.Refresh(context.Background())
	if *asked != 1 || bySetting.Known() != "0.3.0" {
		t.Fatalf("turned back on: asked %d times, known %q", *asked, bySetting.Known())
	}
}

func TestABuildThatIsNotAReleaseAsksNothing(t *testing.T) {
	srv, asked := github(t, "v0.3.0", http.StatusOK)
	now := start
	for _, v := range []string{"dev", "0.1.0-3-g42e4695-dirty"} {
		checker(t, srv.URL, v, &now).Refresh(context.Background())
	}
	if *asked != 0 {
		t.Errorf("a development build asked %d times", *asked)
	}
}

func TestTheFakeReleaseIsKnownWithoutAsking(t *testing.T) {
	srv, asked := github(t, "v0.2.0", http.StatusOK)
	now := start
	c := checker(t, srv.URL, "0.2.0", &now)
	t.Setenv(updatecheck.EnvFake, "v0.3.0")

	if got := c.Known(); got != "0.3.0" || *asked != 0 {
		t.Errorf("known %q, asked %d times", got, *asked)
	}
}

func TestThePersonIsToldOnce(t *testing.T) {
	now := start
	c := checker(t, "", "0.2.0", &now)

	if !c.MarkNoticed() || c.MarkNoticed() {
		t.Error("told twice, or never")
	}
}

// Package updatecheck learns whether a newer release of squad is out. It asks
// GitHub at most once a day, remembers the answer in a small file, and can be
// turned off.
package updatecheck

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/EdrenaVosh/squad-up/internal/fsx"
)

const (
	// EnvOff turns the check off when set to anything.
	EnvOff = "SQUAD_UP_NO_UPDATE_CHECK"
	// EnvFake pretends the given release is out, to see the notice; it
	// asks nothing.
	EnvFake = "SQUAD_UP_FAKE_UPDATE"
	// Interval is how often GitHub is asked at most.
	Interval = 24 * time.Hour
	// LatestURL is GitHub's answer about the newest release.
	LatestURL = "https://api.github.com/repos/EdrenaVosh/squad-up/releases/latest"
	// Timeout bounds one question to GitHub.
	Timeout = 5 * time.Second
)

// State is what the file remembers.
type State struct {
	CheckedAt time.Time `json:"checkedAt,omitzero"`
	Latest    string    `json:"latest,omitempty"`
	Off       bool      `json:"off,omitempty"`     // turned off with squad update --auto-check off
	Noticed   bool      `json:"noticed,omitempty"` // the person has been told about the check
}

// Checker asks about releases newer than Current.
type Checker struct {
	File    string // the state file
	Current string // the running version, such as "0.2.0"
	HTTP    *http.Client
	Now     func() time.Time
	URL     string // LatestURL when empty
}

func (c Checker) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

// Load reads the state; a missing or broken file is an empty state.
func (c Checker) Load() State {
	var s State
	if data, ok, err := fsx.ReadFileIfExists(c.File); err == nil && ok {
		_ = json.Unmarshal(data, &s)
	}
	return s
}

func (c Checker) save(s State) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return fsx.WriteFileAtomic(c.File, append(data, '\n'), 0o644)
}

// Enabled is false when the check is turned off, by EnvOff or by the saved
// setting, and for a build that is not a release (dev, or a commit after one):
// there is nothing to compare it with.
func (c Checker) Enabled() bool {
	if os.Getenv(EnvOff) != "" || c.Load().Off {
		return false
	}
	return Release(c.Current)
}

// SetOff turns the automatic check off or back on.
func (c Checker) SetOff(off bool) error {
	s := c.Load()
	s.Off = off
	return c.save(s)
}

// MarkNoticed records that the person has been told about the check, and
// says whether this is the first time.
func (c Checker) MarkNoticed() bool {
	s := c.Load()
	if s.Noticed {
		return false
	}
	s.Noticed = true
	return c.save(s) == nil
}

// Due is true when the check is on and the last one is a day old.
func (c Checker) Due() bool {
	return c.Enabled() && c.now().Sub(c.Load().CheckedAt) >= Interval
}

// Known is the newer release learned by the last check, without asking
// anything; "" when there is none. EnvFake stands in for it.
func (c Checker) Known() string {
	if fake := os.Getenv(EnvFake); fake != "" {
		return strings.TrimPrefix(fake, "v")
	}
	if !c.Enabled() {
		return ""
	}
	if latest := c.Load().Latest; Newer(latest, c.Current) {
		return latest
	}
	return ""
}

// Refresh asks GitHub when the check is due and remembers the answer. The
// time is remembered also when GitHub cannot be reached, so that an offline
// machine is not slowed down by a try on every run.
func (c Checker) Refresh(ctx context.Context) {
	if !c.Due() {
		return
	}
	latest, err := c.Latest(ctx)
	s := c.Load()
	s.CheckedAt = c.now()
	if err == nil {
		s.Latest = latest
	}
	_ = c.save(s)
}

// Latest asks GitHub for the newest release now, such as "0.3.0".
func (c Checker) Latest(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	url := c.URL
	if url == "" {
		url = LatestURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "squad-up/"+c.Current)
	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub answered %s", resp.Status)
	}
	var release struct {
		Tag string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}
	latest := strings.TrimPrefix(release.Tag, "v")
	if _, ok := parse(latest); !ok {
		return "", fmt.Errorf("GitHub named the release %q, not a version", release.Tag)
	}
	return latest, nil
}

// Release is true for a release version such as "0.2.0", and false for a
// development build.
func Release(v string) bool {
	_, ok := parse(v)
	return ok
}

// Newer is true when latest is a later release than current.
func Newer(latest, current string) bool {
	l, ok1 := parse(latest)
	c, ok2 := parse(current)
	if !ok1 || !ok2 {
		return false
	}
	for i := range l {
		if l[i] != c[i] {
			return l[i] > c[i]
		}
	}
	return false
}

// parse reads a release version, "1.2.3" or "v1.2.3"; anything else, such as
// "dev" or "0.1.0-3-g42e4695-dirty", is not a release.
func parse(v string) ([3]int, bool) {
	var out [3]int
	parts := strings.Split(strings.TrimPrefix(v, "v"), ".")
	if len(parts) != 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, false
		}
		out[i] = n
	}
	return out, true
}

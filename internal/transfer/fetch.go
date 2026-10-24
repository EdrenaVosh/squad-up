package transfer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// ErrNoGH means the GitHub CLI is not installed; gists need it.
var ErrNoGH = errors.New("the GitHub CLI (gh) was not found")

var gistIDRE = regexp.MustCompile(`^[A-Za-z0-9]{1,64}$`)

// Fetcher reads a document from where the user pointed at: a file, `-` for
// standard input, an https link, or `gist:<id>`.
type Fetcher struct {
	HTTP  *http.Client // nil: a client with a 30 s timeout
	Stdin io.Reader
	GH    string // path to gh; empty: look it up on PATH
}

// Fetch returns at most MaxSize bytes, or ErrTooLarge.
func (f Fetcher) Fetch(ctx context.Context, source string) ([]byte, error) {
	switch {
	case source == "-":
		if f.Stdin == nil {
			return nil, errors.New("nothing to read on standard input")
		}
		return readLimited(f.Stdin)
	case strings.HasPrefix(source, "gist:"):
		return f.fetchGist(ctx, strings.TrimPrefix(source, "gist:"))
	case strings.HasPrefix(source, "https://"):
		return f.fetchHTTPS(ctx, source)
	case strings.Contains(source, "://"):
		return nil, fmt.Errorf("%s: only https:// links are accepted", source)
	}
	file, err := os.Open(source)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", source, err)
	}
	defer func() { _ = file.Close() }()
	data, err := readLimited(file)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", source, err)
	}
	return data, nil
}

func readLimited(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxSize {
		return nil, ErrTooLarge
	}
	return data, nil
}

func (f Fetcher) fetchHTTPS(ctx context.Context, url string) ([]byte, error) {
	client := http.Client{Timeout: 30 * time.Second}
	if f.HTTP != nil {
		client = *f.HTTP
	}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if req.URL.Scheme != "https" {
			return fmt.Errorf("refusing to follow a redirect to %s: only https:// links are accepted", req.URL)
		}
		if len(via) >= 5 {
			return errors.New("too many redirects")
		}
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", url, err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: the server answered %s", url, resp.Status)
	}
	data, err := readLimited(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	return data, nil
}

func (f Fetcher) gh() (string, error) {
	if f.GH != "" {
		return f.GH, nil
	}
	bin, err := exec.LookPath("gh")
	if err != nil {
		return "", ErrNoGH
	}
	return bin, nil
}

func (f Fetcher) fetchGist(ctx context.Context, id string) ([]byte, error) {
	if !gistIDRE.MatchString(id) {
		return nil, fmt.Errorf("%q is not a gist id (use gist:<id>, the last part of the gist's address)", id)
	}
	bin, err := f.gh()
	if err != nil {
		return nil, err
	}
	listing, err := runGH(ctx, bin, nil, "gist", "view", id, "--files")
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(listing, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			files = append(files, line)
		}
	}
	pick := ""
	for _, name := range files {
		if strings.HasSuffix(name, ".squad.json") {
			pick = name
			break
		}
	}
	if pick == "" && len(files) == 1 {
		pick = files[0]
	}
	if pick == "" {
		return nil, fmt.Errorf("gist %s has no file ending in .squad.json", id)
	}
	body, err := runGH(ctx, bin, nil, "gist", "view", id, "--filename", pick, "--raw")
	if err != nil {
		return nil, err
	}
	if len(body) > MaxSize {
		return nil, ErrTooLarge
	}
	return []byte(body), nil
}

// CreateGist uploads data as a secret gist (only people with the link can
// open it) and returns the gist's address. gh is the binary to use, or "" to
// look it up on PATH.
func CreateGist(ctx context.Context, gh, filename string, data []byte, desc string) (string, error) {
	bin, err := Fetcher{GH: gh}.gh()
	if err != nil {
		return "", err
	}
	out, err := runGH(ctx, bin, data, "gist", "create", "--filename", filename, "--desc", desc, "-")
	if err != nil {
		return "", err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	last := strings.TrimSpace(lines[len(lines)-1])
	if !strings.HasPrefix(last, "https://") {
		return "", fmt.Errorf("gh did not print the address of the gist: %q", last)
	}
	return last, nil
}

func runGH(ctx context.Context, bin string, stdin []byte, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = append(os.Environ(), "GH_PROMPT_DISABLED=1")
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("gh %s failed: %s", strings.Join(args[:2], " "), firstLine(msg))
	}
	return stdout.String(), nil
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

package transfer

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/EdrenaVosh/squad-up/internal/testutil"
)

func TestFetchReadsAFileAndStdin(t *testing.T) {
	path := filepath.Join(t.TempDir(), "solo.squad.json")
	if err := os.WriteFile(path, []byte("from file"), 0o600); err != nil {
		t.Fatal(err)
	}
	f := Fetcher{Stdin: strings.NewReader("from stdin")}

	if got, err := f.Fetch(context.Background(), path); err != nil || string(got) != "from file" {
		t.Errorf("file: %q, %v", got, err)
	}
	if got, err := f.Fetch(context.Background(), "-"); err != nil || string(got) != "from stdin" {
		t.Errorf("stdin: %q, %v", got, err)
	}
}

func TestFetchSaysWhenTheFileIsMissing(t *testing.T) {
	_, err := Fetcher{}.Fetch(context.Background(), filepath.Join(t.TempDir(), "nope.json"))
	if err == nil || !strings.Contains(err.Error(), "nope.json") {
		t.Fatalf("err = %v", err)
	}
}

func TestFetchRefusesAFileOverTheLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "big.json")
	if err := os.WriteFile(path, make([]byte, MaxSize+1), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Fetcher{}.Fetch(context.Background(), path)
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("err = %v", err)
	}
	_, err = Fetcher{Stdin: strings.NewReader(strings.Repeat("x", MaxSize+1))}.Fetch(context.Background(), "-")
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("stdin err = %v", err)
	}
}

func TestFetchGetsHTTPS(t *testing.T) {
	srv := newTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
	}))
	defer srv.Close()

	got, err := Fetcher{HTTP: srv.Client()}.Fetch(context.Background(), srv.URL+"/x.squad.json")
	if err != nil || string(got) != "hello" {
		t.Fatalf("%q, %v", got, err)
	}
}

func TestFetchRefusesPlainHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("the server must not be contacted")
	}))
	defer srv.Close()

	_, err := Fetcher{}.Fetch(context.Background(), srv.URL)
	if err == nil || !strings.Contains(err.Error(), "https") {
		t.Fatalf("err = %v", err)
	}
}

func TestFetchRefusesARedirectToPlainHTTP(t *testing.T) {
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("the plain server must not be contacted")
	}))
	defer plain.Close()
	srv := newTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, plain.URL, http.StatusFound)
	}))
	defer srv.Close()

	_, err := Fetcher{HTTP: srv.Client()}.Fetch(context.Background(), srv.URL)
	if err == nil || !strings.Contains(err.Error(), "https") {
		t.Fatalf("err = %v", err)
	}
}

func TestFetchReportsHTTPErrorsAndOversizeBodies(t *testing.T) {
	srv := newTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/big" {
			_, _ = w.Write(make([]byte, MaxSize+1))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	f := Fetcher{HTTP: srv.Client()}

	if _, err := f.Fetch(context.Background(), srv.URL+"/missing"); err == nil || !strings.Contains(err.Error(), "404") {
		t.Errorf("404: %v", err)
	}
	if _, err := f.Fetch(context.Background(), srv.URL+"/big"); !errors.Is(err, ErrTooLarge) {
		t.Errorf("big: %v", err)
	}
}

func TestFetchGistPicksTheSquadFile(t *testing.T) {
	log, _ := testutil.FakeGH(t)

	got, err := Fetcher{}.Fetch(context.Background(), "gist:abc123")
	if err != nil || string(got) != "gist body" {
		t.Fatalf("%q, %v", got, err)
	}
	calls, _ := os.ReadFile(log)
	if !strings.Contains(string(calls), "gist view abc123 --files") ||
		!strings.Contains(string(calls), "gist view abc123 --filename solo.squad.json --raw") {
		t.Errorf("gh calls:\n%s", calls)
	}
}

func TestFetchGistWithASingleOtherFileTakesIt(t *testing.T) {
	testutil.FakeGH(t)
	t.Setenv("FAKE_GH_FILES", "preset.json")
	if got, err := (Fetcher{}).Fetch(context.Background(), "gist:abc123"); err != nil || string(got) != "gist body" {
		t.Fatalf("%q, %v", got, err)
	}
}

func TestFetchGistWithSeveralUnrelatedFilesIsRefused(t *testing.T) {
	testutil.FakeGH(t)
	t.Setenv("FAKE_GH_FILES", "a.txt\nb.txt")
	if _, err := (Fetcher{}).Fetch(context.Background(), "gist:abc123"); err == nil || !strings.Contains(err.Error(), ".squad.json") {
		t.Fatalf("err = %v", err)
	}
}

func TestFetchGistRejectsAnOddID(t *testing.T) {
	testutil.FakeGH(t)
	for _, id := range []string{"", "a b", "../x", "--web", "a;b"} {
		if _, err := (Fetcher{}).Fetch(context.Background(), "gist:"+id); err == nil {
			t.Errorf("gist:%q accepted", id)
		}
	}
}

func TestGistNeedsGH(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := Fetcher{}.Fetch(context.Background(), "gist:abc123")
	if !errors.Is(err, ErrNoGH) {
		t.Fatalf("fetch err = %v", err)
	}
	if _, err := CreateGist(context.Background(), "", "solo.squad.json", []byte("x"), "d"); !errors.Is(err, ErrNoGH) {
		t.Fatalf("create err = %v", err)
	}
}

func TestCreateGistSendsTheFileOnStdinAndReturnsTheURL(t *testing.T) {
	log, stdin := testutil.FakeGH(t)

	url, err := CreateGist(context.Background(), "", "solo.squad.json", []byte("payload"), "A squad-up preset")
	if err != nil || url != "https://gist.github.com/me/abc123" {
		t.Fatalf("%q, %v", url, err)
	}
	if got, _ := os.ReadFile(stdin); string(got) != "payload" {
		t.Errorf("stdin = %q", got)
	}
	calls, _ := os.ReadFile(log)
	if !strings.Contains(string(calls), "gist create --filename solo.squad.json --desc A squad-up preset -") {
		t.Errorf("gh call: %s", calls)
	}
	if strings.Contains(string(calls), "--public") {
		t.Errorf("the gist must stay secret: %s", calls)
	}
}

// newTLSServer is a TLS test server whose log stays quiet when a client
// walks away from the handshake, as the redirect tests make it do.
func newTLSServer(h http.Handler) *httptest.Server {
	srv := httptest.NewUnstartedServer(h)
	srv.Config.ErrorLog = log.New(io.Discard, "", 0)
	srv.StartTLS()
	return srv
}

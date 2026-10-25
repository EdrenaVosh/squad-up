package usage_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/EdrenaVosh/squad-up/internal/usage"
)

var now = time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)

// project is a Claude config dir with the transcripts of one project, written
// in the shape Claude Code 2.1.310 writes them.
type project struct {
	t       *testing.T
	claude  string
	cwd     string
	session string
}

func newProject(t *testing.T) *project {
	t.Helper()
	p := &project{t: t, claude: t.TempDir(), cwd: "/Users/me/dev/my.app", session: "s1"}
	if err := os.MkdirAll(p.dir(), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func (p *project) dir() string {
	return filepath.Join(p.claude, "projects", "-Users-me-dev-my-app")
}

// record is one assistant record: Claude Code writes one per content block,
// each carrying the usage of the whole message.
func record(id, model string, in, write, read, out int, at time.Time) string {
	return fmt.Sprintf(`{"type":"assistant","timestamp":%q,"message":{"id":%q,"model":%q,"usage":{"input_tokens":%d,"cache_creation_input_tokens":%d,"cache_read_input_tokens":%d,"output_tokens":%d}}}`,
		at.Format(time.RFC3339), id, model, in, write, read, out) + "\n"
}

func (p *project) main(lines ...string) {
	p.t.Helper()
	p.append(filepath.Join(p.dir(), p.session+".jsonl"), lines...)
}

func (p *project) sub(id, agentType string, finished bool, lines ...string) {
	p.t.Helper()
	dir := filepath.Join(p.dir(), p.session, "subagents")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		p.t.Fatal(err)
	}
	meta := fmt.Sprintf(`{"agentType":%q,"isBuiltIn":false,"finished":%v}`, agentType, finished)
	if err := os.WriteFile(filepath.Join(dir, "agent-"+id+".meta.json"), []byte(meta), 0o644); err != nil {
		p.t.Fatal(err)
	}
	p.append(filepath.Join(dir, "agent-"+id+".jsonl"), lines...)
}

func (p *project) append(path string, lines ...string) {
	p.t.Helper()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		p.t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	for _, l := range lines {
		if _, err := f.WriteString(l); err != nil {
			p.t.Fatal(err)
		}
	}
	if err := os.Chtimes(path, now, now); err != nil {
		p.t.Fatal(err)
	}
}

func (p *project) watcher() *usage.Watcher {
	w := usage.NewWatcher(p.dir(), p.session)
	w.Now = func() time.Time { return now }
	return w
}

func find(r usage.Report, name string) usage.Agent {
	for _, a := range r.Agents {
		if a.Name == name {
			return a
		}
	}
	return usage.Agent{}
}

func TestTheProjectFolderIsTheWorkingFolderWithEverythingOddTurnedIntoADash(t *testing.T) {
	got := usage.ProjectDir("/c", "/Users/me/dev/my.app")
	if want := filepath.Join("/c", "projects", "-Users-me-dev-my-app"); got != want {
		t.Errorf("ProjectDir = %q, want %q", got, want)
	}
	if got := usage.ProjectDir("/c", "/private/tmp/claude-501/-Users-x"); !strings.HasSuffix(got, "-private-tmp-claude-501--Users-x") {
		t.Errorf("ProjectDir = %q", got)
	}
}

func TestEachMessageCountsOnceEvenThoughItsBlocksRepeatTheUsage(t *testing.T) {
	p := newProject(t)
	p.main(
		record("m1", "claude-opus-5-5", 2, 17000, 9000, 262, now), // a thinking block
		record("m1", "claude-opus-5-5", 2, 17000, 9000, 262, now), // the tool_use block of the same message
		record("m2", "claude-opus-5-5", 3, 100, 26000, 129, now),
	)

	r, err := p.watcher().Refresh()
	if err != nil {
		t.Fatal(err)
	}

	m := find(r, usage.MainName)
	if m.Tokens.Input != 5 || m.Tokens.CacheWrite != 17100 || m.Tokens.CacheRead != 35000 || m.Tokens.Output != 391 || m.Tokens.Calls != 2 {
		t.Errorf("main = %+v", m.Tokens)
	}
	if m.Tokens.Total() != 5+17100+391 {
		t.Errorf("Total = %d: like Claude Code's totalTokens it leaves cache reads out", m.Tokens.Total())
	}
}

func TestSubagentsAreNamedByTheirMetaFileAndSummedPerAgent(t *testing.T) {
	p := newProject(t)
	p.main(record("m1", "claude-opus-5-5", 2, 100, 0, 50, now))
	p.sub("a1", "reviewer", true, record("r1", "claude-sonnet-5-5", 2, 20000, 0, 55, now))
	p.sub("a2", "reviewer", true, record("r2", "claude-sonnet-5-5", 1, 500, 20000, 80, now))
	p.sub("b1", "tester", false, record("t1", "claude-haiku-5-5", 4, 3000, 0, 700, now))

	r, err := p.watcher().Refresh()
	if err != nil {
		t.Fatal(err)
	}

	rev := find(r, "reviewer")
	if rev.Runs != 2 || rev.Tokens.Total() != 2+20000+55+1+500+80 || rev.Model != "claude-sonnet-5-5" {
		t.Errorf("reviewer = %+v", rev)
	}
	if rev.Running != 0 {
		t.Errorf("both reviews are finished: %+v", rev)
	}
	if tester := find(r, "tester"); tester.Running != 1 || tester.Runs != 1 {
		t.Errorf("tester = %+v: its meta says it has not finished and its file is fresh", tester)
	}
	want := 2 + 100 + 50 + 20057 + 581 + 3704
	if r.Total.Total() != want {
		t.Errorf("squad total = %d, want %d", r.Total.Total(), want)
	}
	if r.Agents[0].Name != usage.MainName {
		t.Errorf("the main session comes first: %v", names(r))
	}
	if names(r)[1] != "reviewer" { // the heaviest subagent follows
		t.Errorf("order = %v", names(r))
	}
}

func names(r usage.Report) []string {
	var out []string
	for _, a := range r.Agents {
		out = append(out, a.Name)
	}
	return out
}

func TestARunThatNeverFinishedIsNotShownAsWorkingForever(t *testing.T) {
	p := newProject(t)
	p.sub("a1", "reviewer", false, record("r1", "m", 1, 1, 1, 1, now.Add(-time.Hour)))
	old := now.Add(-time.Hour)
	if err := os.Chtimes(filepath.Join(p.dir(), p.session, "subagents", "agent-a1.jsonl"), old, old); err != nil {
		t.Fatal(err)
	}

	r, err := p.watcher().Refresh()
	if err != nil {
		t.Fatal(err)
	}

	if got := find(r, "reviewer"); got.Running != 0 || got.Runs != 1 {
		t.Errorf("reviewer = %+v", got)
	}
}

func TestRefreshingReadsOnlyWhatIsNew(t *testing.T) {
	p := newProject(t)
	p.main(record("m1", "m", 1, 10, 0, 5, now))
	w := p.watcher()
	first, _ := w.Refresh()

	p.main(record("m2", "m", 1, 20, 0, 7, now))
	second, err := w.Refresh()
	if err != nil {
		t.Fatal(err)
	}

	if first.Total.Total() != 16 || second.Total.Total() != 16+28 {
		t.Errorf("totals %d then %d", first.Total.Total(), second.Total.Total())
	}
	if again, _ := w.Refresh(); again.Total != second.Total {
		t.Errorf("a refresh with nothing new changed the total: %+v vs %+v", again.Total, second.Total)
	}
}

func TestALineStillBeingWrittenIsLeftForTheNextRefresh(t *testing.T) {
	p := newProject(t)
	whole := record("m1", "m", 1, 10, 0, 5, now)
	half := record("m2", "m", 1, 20, 0, 7, now)
	p.main(whole, half[:len(half)/2])
	w := p.watcher()

	r, err := w.Refresh()
	if err != nil {
		t.Fatal(err)
	}
	if r.Total.Total() != 16 {
		t.Fatalf("total = %d with half a line at the end", r.Total.Total())
	}

	p.main(half[len(half)/2:])
	if r, _ = w.Refresh(); r.Total.Total() != 16+28 {
		t.Errorf("total = %d once the line is whole", r.Total.Total())
	}
}

func TestUsageUpdatedByAnotherBlockOfTheSameMessageReplacesTheOld(t *testing.T) {
	p := newProject(t)
	p.main(record("m1", "m", 1, 10, 0, 5, now))
	w := p.watcher()
	_, _ = w.Refresh()

	p.main(record("m1", "m", 1, 10, 0, 40, now)) // the output grew as the message streamed

	r, _ := w.Refresh()
	if m := find(r, usage.MainName); m.Tokens.Output != 40 || m.Tokens.Calls != 1 {
		t.Errorf("main = %+v", m.Tokens)
	}
}

func TestNoSessionYetIsNotAnError(t *testing.T) {
	w := usage.NewWatcher(filepath.Join(t.TempDir(), "nothing"), "s1")
	r, err := w.Refresh()
	if err != nil || r.Found {
		t.Errorf("Refresh = %+v, %v", r, err)
	}
}

func TestLatestSessionIsTheNewestTranscriptOfTheProject(t *testing.T) {
	p := newProject(t)
	p.session = "old"
	p.main(record("m1", "m", 1, 1, 1, 1, now))
	older := now.Add(-time.Hour)
	if err := os.Chtimes(filepath.Join(p.dir(), "old.jsonl"), older, older); err != nil {
		t.Fatal(err)
	}
	p.session = "new"
	p.main(record("m1", "m", 1, 1, 1, 1, now))

	got, ok := usage.LatestSession(p.claude, p.cwd)

	if !ok || got != "new" {
		t.Errorf("LatestSession = %q, %v", got, ok)
	}
	if _, ok := usage.LatestSession(p.claude, "/somewhere/else"); ok {
		t.Error("a project with no transcripts has no session")
	}
}

func TestGarbageLinesAreSkipped(t *testing.T) {
	p := newProject(t)
	p.main("not json\n", "{\"type\":\"assistant\"}\n", record("m1", "m", 1, 10, 0, 5, now))

	r, err := p.watcher().Refresh()
	if err != nil || r.Total.Total() != 16 {
		t.Errorf("Refresh = %+v, %v", r.Total, err)
	}
}

func TestShortCounts(t *testing.T) {
	for n, want := range map[int]string{0: "0", 950: "950", 1000: "1k", 1234: "1.2k", 9999: "9.9k", 21616: "21k", 999999: "999k", 1_000_000: "1M", 1_250_000: "1.2M"} {
		if got := usage.Short(n); got != want {
			t.Errorf("Short(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestASessionSinceAMomentIgnoresOlderOnes(t *testing.T) {
	p := newProject(t)
	p.session = "old"
	p.main(record("m1", "m", 1, 1, 1, 1, now))
	older := now.Add(-time.Hour)
	if err := os.Chtimes(filepath.Join(p.dir(), "old.jsonl"), older, older); err != nil {
		t.Fatal(err)
	}

	if _, ok := usage.SessionSince(p.claude, p.cwd, now.Add(-time.Minute)); ok {
		t.Error("a session from an hour ago was taken as new")
	}
	if got, ok := usage.SessionSince(p.claude, p.cwd, now.Add(-2*time.Hour)); !ok || got != "old" {
		t.Errorf("SessionSince = %q, %v", got, ok)
	}
}

func TestTheProjectFolderFollowsASymbolicLinkLikeClaudeCodeDoes(t *testing.T) {
	real := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skip("symbolic links are not available here")
	}

	if got, want := usage.ProjectDir("/c", link), usage.ProjectDir("/c", real); got != want {
		t.Errorf("ProjectDir(link) = %q, want %q", got, want)
	}
}

// ---- several sessions ----

func userRecord(at time.Time, content string, meta bool) string {
	return fmt.Sprintf(`{"type":"user","isMeta":%v,"timestamp":%q,"message":{"role":"user","content":%q}}`+"\n", meta, at.Format(time.RFC3339), content)
}

func TestSessionsOfAFolderAreListedNewestFirst(t *testing.T) {
	p := newProject(t)
	for i, id := range []string{"old", "mid", "new"} {
		p.session = id
		p.main(record("m1", "m", 1, 1, 1, 1, now))
		at := now.Add(time.Duration(i-3) * time.Hour)
		if err := os.Chtimes(filepath.Join(p.dir(), id+".jsonl"), at, at); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(p.dir(), "notes.txt"), []byte("not a session"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := usage.ListSessions(p.claude, p.cwd, 2)

	if len(got) != 2 || got[0].ID != "new" || got[1].ID != "mid" {
		t.Fatalf("sessions = %+v", got)
	}
	if !got[0].Last.After(got[1].Last) {
		t.Errorf("Last = %v, %v", got[0].Last, got[1].Last)
	}
	if usage.ListSessions(p.claude, "/nowhere", 5) != nil {
		t.Error("a folder with no sessions has no list")
	}
}

func TestASessionIsKnownByWhenItStartedAndWhatItWasAskedFirst(t *testing.T) {
	p := newProject(t)
	started := now.Add(-2 * time.Hour)
	p.main(
		userRecord(started, "<command-name>/model</command-name>", false), // a slash command is not a prompt
		userRecord(started.Add(time.Second), "System reminder", true),     // a meta record is not either
		userRecord(started.Add(2*time.Second), "Make the login   page\nfaster, please", false),
		userRecord(started.Add(time.Minute), "A later question", false),
		record("m1", "m", 1, 1, 1, 1, now),
	)

	head := usage.ReadHead(filepath.Join(p.dir(), p.session+".jsonl"))

	if !head.Started.Equal(started) {
		t.Errorf("Started = %v, want %v", head.Started, started)
	}
	if head.Prompt != "Make the login page faster, please" {
		t.Errorf("Prompt = %q: the first real prompt, on one line", head.Prompt)
	}
	if got := usage.ReadHead(filepath.Join(p.dir(), "missing.jsonl")); got.Prompt != "" || !got.Started.IsZero() {
		t.Errorf("a missing file has no head: %+v", got)
	}
}

func TestAPromptIsCutToALineThatFits(t *testing.T) {
	p := newProject(t)
	p.main(userRecord(now, strings.Repeat("слово ", 100), false))

	head := usage.ReadHead(filepath.Join(p.dir(), p.session+".jsonl"))

	if r := []rune(head.Prompt); len(r) > 120 || !strings.HasSuffix(head.Prompt, "…") {
		t.Errorf("Prompt = %q (%d runes)", head.Prompt, len(r))
	}
}

func TestLinesWithoutUsageAreSkippedWithoutBeingParsed(t *testing.T) {
	p := newProject(t)
	big := `{"type":"attachment","payload":"` + strings.Repeat("x", 200_000) + `"}` + "\n"
	p.main(big, big, record("m1", "m", 1, 10, 0, 5, now))

	r, err := p.watcher().Refresh()
	if err != nil || r.Total.Total() != 16 {
		t.Errorf("Refresh = %+v, %v", r.Total, err)
	}
}

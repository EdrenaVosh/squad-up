package tui_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/EdrenaVosh/squadup/internal/tui"
	"github.com/EdrenaVosh/squadup/internal/usage"
)

const workCwd = "/work/my-app"

// session writes transcripts of a Claude Code session of workCwd, in the shape
// Claude Code writes them.
type session struct {
	t   *testing.T
	dir string
}

func (h *harness) session(t *testing.T, id ...string) *session {
	t.Helper()
	name := "s1"
	if len(id) > 0 {
		name = id[0]
	}
	dir := filepath.Join(usage.ProjectDir(h.paths.ClaudeDir, workCwd), name)
	if err := os.MkdirAll(filepath.Join(dir, "subagents"), 0o755); err != nil {
		t.Fatal(err)
	}
	return &session{t: t, dir: dir}
}

func message(id string, in, write, read, out int) string {
	return fmt.Sprintf(`{"type":"assistant","timestamp":%q,"message":{"id":%q,"model":"claude-sonnet-5-5","usage":{"input_tokens":%d,"cache_creation_input_tokens":%d,"cache_read_input_tokens":%d,"output_tokens":%d}}}`+"\n",
		fixedNow.Format(time.RFC3339), id, in, write, read, out)
}

func (s *session) write(path string, lines ...string) {
	s.t.Helper()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		s.t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	for _, l := range lines {
		_, _ = f.WriteString(l)
	}
	_ = os.Chtimes(path, fixedNow, fixedNow)
}

func (s *session) main(lines ...string) { s.write(s.dir+".jsonl", lines...) }

// ask is the first thing a person said in the session, which names it in the list.
func (s *session) ask(at time.Time, text string) {
	s.main(fmt.Sprintf(`{"type":"user","timestamp":%q,"message":{"role":"user","content":%q}}`+"\n", at.Format(time.RFC3339), text))
}

// at sets when the session was last written to.
func (s *session) at(t time.Time) {
	_ = os.Chtimes(s.dir+".jsonl", t, t)
}

// remove deletes the session from the disk.
func (s *session) remove() {
	_ = os.Remove(s.dir + ".jsonl")
	_ = os.RemoveAll(s.dir)
}

func (s *session) sub(id, agent string, finished bool, lines ...string) {
	s.t.Helper()
	meta := fmt.Sprintf(`{"agentType":%q,"finished":%v}`, agent, finished)
	if err := os.WriteFile(filepath.Join(s.dir, "subagents", "agent-"+id+".meta.json"), []byte(meta), 0o644); err != nil {
		s.t.Fatal(err)
	}
	s.write(filepath.Join(s.dir, "subagents", "agent-"+id+".jsonl"), lines...)
}

func usageHarness(t *testing.T) *harness {
	t.Helper()
	ws, _ := newWorkspace(t)
	return harnessFor(t, ws, tui.Env{Paths: ws.Paths, Now: ws.Now, ClaudeVersion: "2.1.309", Cwd: workCwd})
}

func TestTheUsageReportSaysWhenThereIsNoSessionYet(t *testing.T) {
	h := usageHarness(t)

	h.press("u")

	h.contains("Usage", "No Claude Code session in this folder yet", "squad run")
}

func TestTheUsageReportShowsEveryAgentOfASessionLive(t *testing.T) {
	h := usageHarness(t)
	h.resize(140, 40)
	s := h.session(t)
	s.ask(fixedNow.Add(-time.Hour), "Make the login page faster")
	s.main(message("m1", 3, 17000, 9000, 400), message("m2", 2, 100, 26000, 130))
	s.sub("a1", "reviewer", true, message("r1", 2, 20872, 0, 55))
	s.sub("b1", "tester", false, message("t1", 4, 3000, 12000, 700))
	h.send(tui.UsageTickMsg())

	h.press("u")

	h.contains("SESSIONS", "1 session in this folder", "Make the login page faster")
	h.contains("AGENT", "STATE", "RUNS", "SENT", "CACHED", "OUT", "TOTAL", "SHARE")
	h.contains("main", "reviewer", "tester", "working", "done", "SQUAD")
	h.contains("1 agent working", "CACHE HIT", "began", "last activity")
	h.lacks("PACE", "s1")
	h.goldenAt("usage", 140, 40)
}

func TestSeveralSessionsAreListedAndEachHasItsOwnTable(t *testing.T) {
	h := usageHarness(t)
	h.resize(140, 40)
	older := h.session(t, "aaa111")
	older.ask(fixedNow.Add(-5*time.Hour), "Refactor the billing module")
	older.main(message("o1", 1, 8000, 0, 2000))
	older.at(fixedNow.Add(-4 * time.Hour))
	newer := h.session(t, "bbb222")
	newer.ask(fixedNow.Add(-time.Hour), "Fix the flaky checkout test")
	newer.main(message("n1", 1, 500, 0, 100))
	newer.sub("a1", "reviewer", true, message("r1", 1, 3000, 0, 400))
	newer.at(fixedNow)
	h.send(tui.UsageTickMsg())

	h.press("u")

	h.contains("2 sessions in this folder", "Fix the flaky checkout test", "Refactor the billing module", "reviewer")
	h.lacks("aaa111", "bbb222") // ids are not shown

	h.press("down")
	h.contains("Refactor the billing module", "10k tokens")
	h.lacks("reviewer") // the other session's agent
	h.press("up")
	h.contains("reviewer")
}

func TestTheListFollowsSessionsThatAppearAndGoAway(t *testing.T) {
	h := usageHarness(t)
	h.resize(140, 40)
	first := h.session(t, "first")
	first.ask(fixedNow.Add(-2*time.Hour), "The first task")
	first.main(message("f1", 1, 1000, 0, 100))
	first.at(fixedNow.Add(-time.Hour))
	h.send(tui.UsageTickMsg())
	h.press("u")
	h.contains("1 session in this folder", "The first task")

	// a new session starts while the screen is open
	second := h.session(t, "second")
	second.ask(fixedNow, "A task that just began")
	second.main(message("s1", 1, 200, 0, 50))
	second.at(fixedNow)
	h.send(tui.UsageTickMsg())
	h.contains("2 sessions in this folder", "A task that just began", "The first task")
	h.contains("A task that just began") // the selection is still on the first one it showed, so the table is that of the newest

	// the selected one is deleted: the selection moves to what is left
	second.remove()
	h.send(tui.UsageTickMsg())
	h.contains("1 session in this folder", "The first task")
	h.lacks("A task that just began")
}

func TestTheSelectionStaysOnItsSessionWhileTheListChanges(t *testing.T) {
	h := usageHarness(t)
	h.resize(140, 40)
	a := h.session(t, "a")
	a.ask(fixedNow.Add(-3*time.Hour), "Task A")
	a.main(message("a1", 1, 1000, 0, 100))
	a.at(fixedNow.Add(-2 * time.Hour))
	h.send(tui.UsageTickMsg())
	h.press("u")
	h.contains("Task A") // the screen has been drawn, with A selected

	// a newer session appears; the person has A selected, and stays on A
	b := h.session(t, "b")
	b.ask(fixedNow, "Task B")
	b.main(message("b1", 1, 9000, 0, 900))
	b.sub("t1", "tester", true, message("t1", 1, 500, 0, 50))
	b.at(fixedNow)
	h.send(tui.UsageTickMsg())

	h.contains("Task A", "Task B", "SQUAD")
	h.lacks("tester") // the table is still A's: it has no tester
}

func TestTheNumbersFollowTheSessionAsItWrites(t *testing.T) {
	h := usageHarness(t)
	h.resize(140, 40)
	s := h.session(t)
	s.main(message("m1", 1, 1000, 0, 100))
	h.send(tui.UsageTickMsg())
	h.press("u")
	h.contains("1.1k tokens")

	s.sub("a1", "reviewer", false, message("r1", 1, 4000, 0, 900))
	h.send(tui.UsageTickMsg())

	h.contains("reviewer", "working", "6k tokens")
}

func TestZStartsCountingTheSelectedSessionFromNow(t *testing.T) {
	h := usageHarness(t)
	h.resize(140, 40)
	s := h.session(t)
	s.main(message("m1", 1, 9000, 0, 1000))
	h.send(tui.UsageTickMsg())
	h.press("u")
	h.contains("10k tokens")

	h.press("z")
	h.contains("0 tokens", "counting from the mark")
	s.main(message("m2", 1, 400, 0, 100))
	h.send(tui.UsageTickMsg())
	h.contains("501 tokens")

	h.press("z")
	h.lacks("counting from the mark")
	h.contains("10k tokens")
}

func TestTheMainScreenCarriesALiveTokenPanelOfTheNewestSession(t *testing.T) {
	h := usageHarness(t)
	h.resize(160, 44)
	h.contains("TOKENS · LIVE", "no session in this folder yet")

	s := h.session(t)
	s.main(message("m1", 1, 5000, 0, 500))
	s.sub("a1", "reviewer", false, message("r1", 1, 2000, 0, 300))
	h.send(tui.UsageTickMsg())
	h.contains("TOKENS · LIVE", "main", "reviewer", "7.8k tokens")

	other := h.session(t, "other")
	other.main(message("o1", 1, 100, 0, 10))
	other.at(fixedNow.Add(-time.Hour))
	h.send(tui.UsageTickMsg())
	h.lacks("other session") // an old one is not running
}

func TestTheUsageReportOpensOnItsOwnWhenAsked(t *testing.T) {
	ws, _ := newWorkspace(t)
	h := harnessFor(t, ws, tui.Env{Paths: ws.Paths, Now: ws.Now, ClaudeVersion: "2.1.309", Cwd: workCwd, Start: tui.Start{Kind: tui.StartUsage}})

	h.contains("Usage", "No Claude Code session")
	h.press("esc")
	h.contains("PRESETS")
}

func TestOnANarrowWindowTheSessionsAreAListAboveTheTable(t *testing.T) {
	h := usageHarness(t)
	s := h.session(t)
	s.ask(fixedNow, "Fix the login")
	s.main(message("m1", 1, 1000, 0, 100))
	h.send(tui.UsageTickMsg())

	h.press("u")

	h.contains("SESSIONS", "Fix the login", "AGENT", "TOTAL")
	h.fits(80, 24)
}

func TestOnABigTerminalTheUsageScreenIsAsBigAsTheMainScreen(t *testing.T) {
	h := usageHarness(t)
	h.resize(260, 80)
	for i, id := range []string{"s1", "s2", "s3"} {
		s := h.session(t, id)
		s.ask(fixedNow.Add(-time.Duration(i+1)*time.Hour), fmt.Sprintf("Task number %d", i+1))
		s.main(message("m1", 1, 1000*(i+1), 0, 100))
		s.sub("a1", "reviewer", true, message("r1", 1, 500, 0, 50))
		s.at(fixedNow.Add(-time.Duration(i) * time.Minute))
	}
	h.send(tui.UsageTickMsg())

	h.press("u")

	h.contains("Task number 1", "Task number 2", "Task number 3", "reviewer")
	h.contains("Where the tokens went", "■ sent", "■ cached", "■ out") // a tall window has room for the chart
	lines := strings.Split(h.view(), "\n")
	width, height := 0, 0
	for _, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "╭") || strings.HasPrefix(strings.TrimSpace(l), "│") || strings.HasPrefix(strings.TrimSpace(l), "╰") {
			width = max(width, len([]rune(strings.TrimSpace(l))))
			height++
		}
	}
	if width != 200 || height != 44 {
		t.Errorf("the screen is %d×%d, want %d×%d (the size of the main screen)", width, height, 200, 44)
	}
	h.fits(260, 80)
}

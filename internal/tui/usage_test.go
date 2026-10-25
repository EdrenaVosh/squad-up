package tui_test

import (
	"fmt"
	"os"
	"path/filepath"
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

func (h *harness) session(t *testing.T) *session {
	t.Helper()
	dir := filepath.Join(usage.ProjectDir(h.paths.ClaudeDir, workCwd), "s1")
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
	ws, meta := newWorkspace(t)
	_ = meta
	h := harnessFor(t, ws, tui.Env{Paths: ws.Paths, Now: ws.Now, ClaudeVersion: "2.1.309", Cwd: workCwd})
	return h
}

func TestTheUsageReportSaysWhenThereIsNoSessionYet(t *testing.T) {
	h := usageHarness(t)

	h.press("u")

	h.contains("Usage", "No Claude Code session in this folder yet", "squad run")
}

func TestTheUsageReportShowsEveryAgentLive(t *testing.T) {
	h := usageHarness(t)
	h.resize(140, 40)
	s := h.session(t)
	s.main(message("m1", 3, 17000, 9000, 400), message("m2", 2, 100, 26000, 130))
	s.sub("a1", "reviewer", true, message("r1", 2, 20872, 0, 55))
	s.sub("b1", "tester", false, message("t1", 4, 3000, 12000, 700))
	h.send(tui.UsageTickMsg())

	h.press("u")

	h.contains("AGENT", "STATE", "RUNS", "SENT", "CACHED", "OUT", "TOTAL", "SHARE")
	h.contains("main", "reviewer", "tester", "working", "done", "SQUAD")
	h.contains("session s1", "1 agent working", "CACHE HIT")
	h.lacks("PACE")
	h.goldenAt("usage", 140, 40)
}

func TestTheNumbersFollowTheSessionAsItWrites(t *testing.T) {
	h := usageHarness(t)
	h.resize(140, 40)
	s := h.session(t)
	s.main(message("m1", 1, 1000, 0, 100))
	h.send(tui.UsageTickMsg())
	h.press("u")
	h.contains("1.1k tokens so far")

	s.sub("a1", "reviewer", false, message("r1", 1, 4000, 0, 900))
	h.send(tui.UsageTickMsg())

	h.contains("reviewer", "working", "6k tokens so far")
}

func TestZStartsCountingFromNow(t *testing.T) {
	h := usageHarness(t)
	h.resize(140, 40)
	s := h.session(t)
	s.main(message("m1", 1, 9000, 0, 1000))
	h.send(tui.UsageTickMsg())
	h.press("u")
	h.contains("10k tokens so far")

	h.press("z")
	h.contains("0 tokens so far", "counting from the mark")
	s.main(message("m2", 1, 400, 0, 100))
	h.send(tui.UsageTickMsg())
	h.contains("501 tokens so far")

	h.press("z")
	h.lacks("counting from the mark")
	h.contains("10k tokens so far")
}

func TestTheMainScreenCarriesALiveTokenPanel(t *testing.T) {
	h := usageHarness(t)
	h.resize(160, 44)
	h.contains("TOKENS · LIVE", "no session in this folder yet")

	s := h.session(t)
	s.main(message("m1", 1, 5000, 0, 500))
	s.sub("a1", "reviewer", false, message("r1", 1, 2000, 0, 300))
	h.send(tui.UsageTickMsg())

	h.contains("TOKENS · LIVE", "main", "reviewer", "7.8k tokens")
}

func TestTheUsageReportOpensOnItsOwnWhenAsked(t *testing.T) {
	ws, _ := newWorkspace(t)
	h := harnessFor(t, ws, tui.Env{Paths: ws.Paths, Now: ws.Now, ClaudeVersion: "2.1.309", Cwd: workCwd, Start: tui.Start{Kind: tui.StartUsage}})

	h.contains("Usage", "No Claude Code session")
	h.press("esc")
	h.contains("PRESETS")
}

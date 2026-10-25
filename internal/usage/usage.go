// Package usage reports the tokens a Claude Code session spent, per agent.
//
// Claude Code writes every message of a session to a transcript under
// ~/.claude/projects/<project>/<session>.jsonl, and every subagent run to
// <session>/subagents/agent-<id>.jsonl beside a meta file that names the
// agent. Each assistant record carries the API's own token counts, so nothing
// here is estimated: the numbers are read from what was really sent.
package usage

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

// MainName is the name of the session itself, the one the user talks to.
const MainName = "main"

// runningWindow is how recently a transcript must have been written for an
// unfinished run to count as working: a session that was closed mid-run leaves
// its meta file saying "not finished" for good.
const runningWindow = 2 * time.Minute

// Tokens are the counts the API reports, summed.
type Tokens struct {
	Input      int // fresh input
	CacheWrite int // input written to the prompt cache
	CacheRead  int // input served from the prompt cache
	Output     int
	Calls      int // messages
}

// Total is what Claude Code itself calls a run's totalTokens: everything sent
// and received, cache reads left out (they are re-read at a fraction of the
// price).
func (t Tokens) Total() int { return t.Input + t.CacheWrite + t.Output }

func (t *Tokens) add(o Tokens) {
	t.Input += o.Input
	t.CacheWrite += o.CacheWrite
	t.CacheRead += o.CacheRead
	t.Output += o.Output
	t.Calls += o.Calls
}

// Agent is what one agent spent, over all its runs.
type Agent struct {
	Name    string
	Runs    int // subagent runs; 0 for the main session
	Running int // runs that are working now
	Tokens  Tokens
	Model   string    // of its latest message
	Last    time.Time // of its latest message
}

// Report is the state of a session.
type Report struct {
	Found   bool // there is a transcript to read
	Project string
	Session string
	Agents  []Agent // the main session first, then the heaviest agent first
	Total   Tokens
	Updated time.Time
}

var oddChars = regexp.MustCompile(`[^a-zA-Z0-9]`)

// ProjectDir is the folder Claude Code keeps the sessions of the project in
// cwd: the path with every character that is not a letter or a digit
// replaced by a dash.
//
// Claude Code names the folder after the real path, so a symbolic link in cwd
// (/tmp is one on macOS) is followed first.
func ProjectDir(claudeDir, cwd string) string {
	if real, err := filepath.EvalSymlinks(cwd); err == nil {
		cwd = real
	}
	return filepath.Join(claudeDir, "projects", oddChars.ReplaceAllString(cwd, "-"))
}

// LatestSession is the newest session of the project in cwd.
func LatestSession(claudeDir, cwd string) (string, bool) {
	return SessionSince(claudeDir, cwd, time.Time{})
}

// SessionSince is the newest session of the project in cwd that was written
// to at or after since.
func SessionSince(claudeDir, cwd string, since time.Time) (string, bool) {
	entries, err := os.ReadDir(ProjectDir(claudeDir, cwd))
	if err != nil {
		return "", false
	}
	var best string
	var bestTime time.Time
	for _, e := range entries {
		name, ok := strings.CutSuffix(e.Name(), ".jsonl")
		if !ok || e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(since) {
			continue
		}
		if best == "" || info.ModTime().After(bestTime) {
			best, bestTime = name, info.ModTime()
		}
	}
	return best, best != ""
}

type message struct {
	tokens Tokens
	model  string
	at     time.Time
}

// transcript is a file read so far.
type transcript struct {
	offset int64
	msgs   map[string]message
	mtime  time.Time
}

// Watcher reads a session again and again, taking from each transcript only
// what was written since the last time.
type Watcher struct {
	Now func() time.Time

	dir, session string
	files        map[string]*transcript
}

// NewWatcher watches one session of the project folder dir.
func NewWatcher(dir, session string) *Watcher {
	return &Watcher{Now: time.Now, dir: dir, session: session, files: map[string]*transcript{}}
}

// Refresh reads what is new and returns the whole picture. A session that has
// no transcript yet is a report with Found false, not an error.
func (w *Watcher) Refresh() (Report, error) {
	r := Report{Project: w.dir, Session: w.session, Updated: w.Now()}

	byName := map[string]*Agent{}
	agent := func(name string) *Agent {
		if byName[name] == nil {
			byName[name] = &Agent{Name: name}
		}
		return byName[name]
	}
	fold := func(a *Agent, t *transcript) {
		for _, m := range t.msgs {
			a.Tokens.add(m.tokens)
			if m.at.After(a.Last) {
				a.Last, a.Model = m.at, m.model
			}
		}
	}

	main, err := w.read(filepath.Join(w.dir, w.session+".jsonl"))
	if err != nil {
		return r, err
	}
	if main != nil {
		r.Found = true
		fold(agent(MainName), main)
	}

	subs, _ := filepath.Glob(filepath.Join(w.dir, w.session, "subagents", "agent-*.jsonl"))
	for _, path := range subs {
		t, err := w.read(path)
		if err != nil || t == nil {
			continue
		}
		r.Found = true
		name, finished := readMeta(strings.TrimSuffix(path, ".jsonl") + ".meta.json")
		a := agent(name)
		a.Runs++
		if !finished && w.Now().Sub(t.mtime) < runningWindow {
			a.Running++
		}
		fold(a, t)
	}

	for _, a := range byName {
		r.Agents = append(r.Agents, *a)
		r.Total.add(a.Tokens)
	}
	slices.SortFunc(r.Agents, func(a, b Agent) int {
		switch {
		case a.Name == MainName:
			return -1
		case b.Name == MainName:
			return 1
		case a.Tokens.Total() != b.Tokens.Total():
			return b.Tokens.Total() - a.Tokens.Total()
		}
		return strings.Compare(a.Name, b.Name)
	})
	return r, nil
}

// readMeta returns the agent's name and whether its run has finished.
func readMeta(path string) (name string, finished bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "subagent", true
	}
	name = gjson.GetBytes(data, "agentType").String()
	if name == "" {
		name = "subagent"
	}
	return name, gjson.GetBytes(data, "finished").Bool()
}

// read takes the new complete lines of a transcript. It returns nil when the
// file does not exist.
func (w *Watcher) read(path string) (*transcript, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	t := w.files[path]
	if t == nil {
		t = &transcript{msgs: map[string]message{}}
		w.files[path] = t
	}
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	t.mtime = info.ModTime()
	if info.Size() < t.offset { // the file was replaced
		t.offset, t.msgs = 0, map[string]message{}
	}
	if _, err := f.Seek(t.offset, io.SeekStart); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	end := bytes.LastIndexByte(data, '\n') + 1 // a line still being written waits
	for _, line := range bytes.Split(data[:end], []byte{'\n'}) {
		t.take(line)
	}
	t.offset += int64(end)
	return t, nil
}

// take records the usage of an assistant record. Claude Code writes one record
// per content block of a message, each with the usage of the whole message,
// so a message counts once; a later record of the same message replaces the
// earlier one, because the output grows as it streams.
func (t *transcript) take(line []byte) {
	// Most lines of a transcript carry no usage; skip them without parsing.
	if len(line) == 0 || !bytes.Contains(line, []byte(`"usage"`)) || !gjson.ValidBytes(line) {
		return
	}
	rec := gjson.ParseBytes(line)
	if rec.Get("type").String() != "assistant" {
		return
	}
	id, u := rec.Get("message.id").String(), rec.Get("message.usage")
	if id == "" || !u.Exists() {
		return
	}
	at, _ := time.Parse(time.RFC3339Nano, rec.Get("timestamp").String())
	t.msgs[id] = message{
		tokens: Tokens{
			Input:      int(u.Get("input_tokens").Int()),
			CacheWrite: int(u.Get("cache_creation_input_tokens").Int()),
			CacheRead:  int(u.Get("cache_read_input_tokens").Int()),
			Output:     int(u.Get("output_tokens").Int()),
			Calls:      1,
		},
		model: rec.Get("message.model").String(),
		at:    at,
	}
}

// Short writes a count the short way: 950, 1.2k, 21.6k, 1.2M.
func Short(n int) string {
	switch {
	case n < 1000:
		return strconv.Itoa(n)
	case n < 10_000:
		return tenth(n, 1000) + "k"
	case n < 1_000_000:
		return strconv.Itoa(n/1000) + "k"
	}
	return tenth(n, 1_000_000) + "M"
}

func tenth(n, unit int) string {
	whole, frac := n/unit, (n%unit)*10/unit
	if frac == 0 {
		return strconv.Itoa(whole)
	}
	return strconv.Itoa(whole) + "." + strconv.Itoa(frac)
}

// Session is a session of a folder, as listed.
type Session struct {
	ID   string
	Last time.Time // when its transcript was last written to
}

// ListSessions lists the newest sessions of the project in cwd, at most limit,
// newest first. A folder with no sessions gives nil.
func ListSessions(claudeDir, cwd string, limit int) []Session {
	entries, err := os.ReadDir(ProjectDir(claudeDir, cwd))
	if err != nil {
		return nil
	}
	var out []Session
	for _, e := range entries {
		name, ok := strings.CutSuffix(e.Name(), ".jsonl")
		if !ok || e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, Session{ID: name, Last: info.ModTime()})
	}
	slices.SortFunc(out, func(a, b Session) int {
		if c := b.Last.Compare(a.Last); c != 0 {
			return c
		}
		return strings.Compare(a.ID, b.ID)
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// Head is what the start of a transcript says about the session.
type Head struct {
	Started time.Time
	Prompt  string // the first thing the person asked, on one line
}

// maxHead is how much of a transcript is read to find its head.
const maxHead = 256 * 1024

// maxPrompt is the longest prompt kept, in characters.
const maxPrompt = 100

// ReadHead reads when a session started and what it was first asked. Lines that
// are not a person's own words are skipped: meta records, and the wrappers
// Claude Code puts around slash commands.
func ReadHead(path string) Head {
	f, err := os.Open(path)
	if err != nil {
		return Head{}
	}
	defer func() { _ = f.Close() }()
	buf := make([]byte, maxHead)
	n, _ := io.ReadFull(f, buf)
	buf = buf[:n]

	var h Head
	for _, line := range bytes.Split(buf, []byte{'\n'}) {
		if len(line) == 0 || !gjson.ValidBytes(line) {
			continue
		}
		rec := gjson.ParseBytes(line)
		if h.Started.IsZero() {
			h.Started, _ = time.Parse(time.RFC3339Nano, rec.Get("timestamp").String())
		}
		if rec.Get("type").String() != "user" || rec.Get("isMeta").Bool() {
			continue
		}
		if text := promptText(rec.Get("message.content")); text != "" {
			h.Prompt = text
			break
		}
	}
	return h
}

// promptText is the text of a user message, or "" when it is not a person's own words.
func promptText(content gjson.Result) string {
	text := content.String()
	if content.IsArray() {
		text = ""
		content.ForEach(func(_, part gjson.Result) bool {
			if part.Get("type").String() == "text" {
				text = part.Get("text").String()
				return false
			}
			return true
		})
	}
	text = strings.Join(strings.Fields(text), " ")
	if text == "" || strings.HasPrefix(text, "<") || strings.HasPrefix(text, "Caveat:") {
		return ""
	}
	if r := []rune(text); len(r) > maxPrompt {
		text = string(r[:maxPrompt-1]) + "…"
	}
	return text
}

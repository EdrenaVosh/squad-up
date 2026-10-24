// Package team turns a preset's agents into a squad: agents that share a tiny
// notes file, and a main agent that hands work down the slots and sends
// defects back until the gate is satisfied.
//
// Claude Code has no switch for a subagent's context: every one starts with a
// clean window and sees only the task it is given. Squad adds the sharing
// itself, in the prompts Claude Code receives. The files stored in a preset
// stay as authored; Compile produces the text that is written or passed on.
package team

import (
	"fmt"
	"slices"
	"strings"

	"github.com/EdrenaVosh/squadup/internal/agentfile"
	"github.com/EdrenaVosh/squadup/internal/preset"
)

const (
	// NotesFile is the squad's shared notes, relative to the project.
	NotesFile = ".squad/notes.md"
	// ShareKey is the frontmatter field that isolates an agent. Claude Code
	// would ignore it; Compile removes it from what Claude Code gets.
	ShareKey = "squadShare"

	notesOpen, notesClose = "<!-- squad:notes -->", "<!-- /squad:notes -->"
	leadOpen, leadClose   = "<!-- squad:lead -->", "<!-- /squad:lead -->"
)

// The notes blocks are what a sharing agent is told. Every sharing agent pays
// for one on every run, so they stay under 140 words. The format is the
// cheapest one that keeps what git cannot tell: why, what failed, what is
// still broken. A `!` line is a work item, so "no line starts with !" is the
// loop's exit.
const (
	notesFormat = "`D key: decision, why` · `X key: tried, ruled out` · `! key: open defect @path:line →owner` · `? key: question` · `> you: handoff`"
	notesRules  = "Same key: edit that line, never add a second. Delete lines that are fixed or stale. Over 25 lines: merge the oldest. Point to code with path:line, never paste it; git already holds what changed."

	// notesBlock is for an agent that can read and write the file.
	notesBlock = notesOpen + `
## Shared notes
File ` + "`" + NotesFile + "`" + `: read it before you start, update it before you answer. Create it if missing.
At most 25 lines of at most 100 characters: telegraphic, no prose. One tagged fact per line:
` + notesFormat + ` (one line, yours only)
` + notesRules + `
` + notesClose

	// readOnlyBlock is for an agent that can read the file but not change it:
	// it hands its lines to the lead.
	readOnlyBlock = notesOpen + `
## Shared notes
File ` + "`" + NotesFile + "`" + `: read it before you start. You cannot edit it: end your answer with the lines to add or change, and the lead records them.
Telegraphic, at most 100 characters per line, no prose. One tagged fact per line:
` + notesFormat + `
` + notesRules + `
` + notesClose
)

// Shared reports whether the agent shares the squad's notes. Agents do unless
// the file says squadShare: false.
func Shared(a *agentfile.Agent) bool {
	v, ok := a.Get(ShareKey)
	if !ok {
		return true
	}
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return !slices.Contains([]string{"false", "no", "off"}, strings.ToLower(x))
	}
	return true
}

// SetShared isolates the agent or returns it to the default. Sharing is the
// default, so it is never written out.
func SetShared(a *agentfile.Agent, shared bool) {
	if shared {
		a.Delete(ShareKey)
		return
	}
	a.Set(ShareKey, false)
}

// Compile returns the agent files as Claude Code should get them: without the
// squad-only field, with the notes block for the agents that share, and with
// the flow brief for the main agent. Untouched files keep their exact bytes.
func Compile(b preset.Bundle) ([]preset.AgentFile, error) {
	together := len(b.Agents) >= 2
	brief := ""
	if b.Preset.Main != nil {
		brief = Brief(b)
	}

	out := make([]preset.AgentFile, 0, len(b.Agents))
	for _, f := range b.Agents {
		a, err := agentfile.Parse(f.Data)
		if err != nil {
			return nil, fmt.Errorf("agent %q: %w", f.Name, err)
		}
		shared := Shared(a)
		read, write := Access(a)
		a.Delete(ShareKey)

		body := a.Prompt()
		clean := stripBlock(stripBlock(body, notesOpen, notesClose), leadOpen, leadClose)
		add := ""
		switch {
		case !together || !shared || !read:
		case write:
			add += notesBlock
		default:
			add += readOnlyBlock
		}
		if brief != "" && b.Preset.Main != nil && f.Name == *b.Preset.Main {
			if add != "" {
				add += "\n\n"
			}
			add += brief
		}
		if add != "" || clean != body {
			a.SetBody(join(clean, add))
		}
		data, err := a.Marshal()
		if err != nil {
			return nil, fmt.Errorf("agent %q: %w", f.Name, err)
		}
		out = append(out, preset.AgentFile{Name: f.Name, Data: data})
	}
	return out, nil
}

// Brief is the flow told to whoever leads the squad: the order, who works in
// isolation, and the loop. It is empty when the preset has no flow or fewer
// than two agents. The lead itself is not a step.
func Brief(b preset.Bundle) string {
	flow := b.Preset.Flow
	if flow == nil || len(b.Agents) < 2 {
		return ""
	}
	var steps, isolated []string
	notes, relay := false, false
	for _, f := range b.Agents {
		if b.Preset.Main != nil && f.Name == *b.Preset.Main {
			continue
		}
		steps = append(steps, fmt.Sprintf("%d %s", len(steps)+1, f.Name))
		a, err := agentfile.Parse(f.Data)
		switch {
		case err != nil:
		case !Shared(a):
			isolated = append(isolated, "`"+f.Name+"`")
		default:
			notes = true
			if _, write := Access(a); !write {
				relay = true
			}
		}
	}
	if len(steps) == 0 {
		return ""
	}

	var s strings.Builder
	s.WriteString(leadOpen + "\n## Squad flow\n")
	s.WriteString("Delegate in this order, one agent per step, giving each its task")
	if notes {
		s.WriteString(" and the path `" + NotesFile + "`")
	}
	s.WriteString(":\n" + strings.Join(steps, " → ") + "\n")
	if len(isolated) > 0 {
		s.WriteString("Isolated: " + strings.Join(isolated, ", ") + ". Give them the task or the diff only, never the notes or another agent's reasoning")
		if notes {
			s.WriteString("; write what they report into the notes yourself")
		}
		s.WriteString(".\n")
	}
	if notes {
		s.WriteString("First write `goal: …` (one line) at the top of the notes, replacing notes left from earlier work unless the user is continuing it.\n")
	}
	if relay {
		s.WriteString("Agents that cannot write end their answer with note lines; record those in the notes yourself.\n")
	}
	if flow.Loops() {
		s.WriteString(loopText(flow, notes))
	} else {
		s.WriteString("Finish with a summary of at most 5 lines.\n")
	}
	s.WriteString(leadClose)
	return s.String()
}

func loopText(f *preset.Flow, notes bool) string {
	defects := "if `" + f.Gate + "` reports defects"
	if notes {
		defects = "count the lines that start with `!` in the notes"
	}
	return fmt.Sprintf("Loop: after `%s` finishes, %s. None: finish with a summary of at most 5 lines. Some: give `%s` only those defects and run it, then run `%s` again. After %d rounds with defects left, stop and show the user what is still open.\n",
		f.Gate, defects, f.Fix, f.Gate, f.MaxRounds())
}

// stripBlock removes a marked block and the blank lines before it.
func stripBlock(body, open, closing string) string {
	i := strings.Index(body, open)
	if i < 0 {
		return body
	}
	j := strings.Index(body[i:], closing)
	if j < 0 {
		return body
	}
	rest := strings.TrimLeft(body[i+j+len(closing):], "\n")
	head := strings.TrimRight(body[:i], "\n")
	switch {
	case head == "":
		return rest
	case rest == "":
		return head + "\n"
	}
	return head + "\n\n" + rest
}

func join(body, add string) string {
	body = strings.TrimRight(body, "\n")
	switch {
	case add == "":
		return body + "\n"
	case body == "":
		return add + "\n"
	}
	return body + "\n\n" + add + "\n"
}

// Access reports whether the agent can read and change the notes file with the
// tools it has. An agent with no tools list has them all.
func Access(a *agentfile.Agent) (read, write bool) {
	tools, ok := a.Get("tools")
	if !ok || tools == nil {
		return true, !denied(a, "Write", "Edit", "Bash")
	}
	has := func(names ...string) bool {
		for _, t := range toolNames(tools) {
			if slices.Contains(names, t) {
				return !denied(a, t)
			}
		}
		return false
	}
	return has("Read", "Bash", "Grep"), has("Write", "Edit", "Bash")
}

// CanDelegate reports whether the agent's tools let it hand work to other
// agents, which a main agent running a flow must be able to do.
func CanDelegate(a *agentfile.Agent) bool {
	tools, ok := a.Get("tools")
	if !ok || tools == nil {
		return !denied(a, "Agent") && !denied(a, "Task")
	}
	for _, t := range toolNames(tools) {
		if (t == "Agent" || t == "Task") && !denied(a, t) {
			return true
		}
	}
	return false
}

// denied reports whether disallowedTools removes every one of the tools.
func denied(a *agentfile.Agent, names ...string) bool {
	v, _ := a.Get("disallowedTools")
	off := toolNames(v)
	for _, n := range names {
		if !slices.Contains(off, n) {
			return false
		}
	}
	return true
}

// toolNames lists the tools of a `tools` value, a comma-separated string or a
// list, without the argument patterns of a rule such as Bash(git *).
func toolNames(v any) []string {
	var raw []string
	switch x := v.(type) {
	case string:
		raw = strings.Split(x, ",")
	case []any:
		for _, item := range x {
			raw = append(raw, fmt.Sprint(item))
		}
	}
	var names []string
	for _, r := range raw {
		name, _, _ := strings.Cut(strings.TrimSpace(r), "(")
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

// NotesTokens estimates what the notes block adds to an agent's prompt.
func NotesTokens() int { return (len(notesBlock) + 3) / 4 }

// ExampleNotes shows what the notes look like, for the editor to display.
var ExampleNotes = []string{
	"goal: rate-limit the login route",
	"D auth: bcrypt cost 12, argon2 is not in deps",
	"X cache: redis 3x slower than an LRU, dropped",
	"! sec: token not rotated on refresh @api/auth.ts:88 →dev",
	"> tester: 14 pass, 1 fail, see ! sec",
}

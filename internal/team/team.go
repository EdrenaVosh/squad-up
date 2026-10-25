// Package team turns a preset's agents into a squad: a main agent that hands
// work down the slots and sends defects back until the gate is satisfied, and
// agents that are *independent*, such as a code reviewer who must judge the
// work without knowing how it was made.
//
// Claude Code starts every subagent with a clean window, so a subagent never
// sees the conversation; what it sees is the task its caller writes for it.
// Independence asks for two things in text: the agent is told to work from its
// task and instructions alone, and whoever calls it is told to give it nothing
// else. Squad adds both to what Claude Code receives; the files stored in a
// preset stay as authored, and Compile produces what is written or passed on.
package team

import (
	"fmt"
	"slices"
	"strings"

	"github.com/EdrenaVosh/squadup/internal/agentfile"
	"github.com/EdrenaVosh/squadup/internal/preset"
)

const (
	// IndependentKey is the frontmatter field that makes an agent independent.
	// Claude Code would ignore it; Compile removes it from what Claude Code gets.
	IndependentKey = "squadIndependent"
	// LegacyShareKey belonged to the shared notes, which are gone. It is read as
	// nothing, never sent to Claude Code, and removed when an agent is saved.
	LegacyShareKey = "squadShare"

	charterOpen, charterClose = "<!-- squad:independent -->", "<!-- /squad:independent -->"
	leadOpen, leadClose       = "<!-- squad:lead -->", "<!-- /squad:lead -->"

	// descriptionNote is added to the description of an independent agent, which
	// is what the caller reads when it decides whom to call and what to say.
	descriptionNote = " Independent: give it only the task and instructions, never your own conclusions or what other agents reasoned or reported."
)

// charter is what an independent agent is told. It is short: every call of the
// agent pays for it, and it fits any kind of agent, not only a reviewer.
const charter = charterOpen + `
## Independence
You are an independent agent. You were given only a task and instructions, on purpose: work from them and from what you can check yourself. Do not rely on, or ask for, the conclusions, reasoning or reports of other agents or of whoever called you. If something you need is missing, say what is missing instead of guessing. Be impartial: weigh the evidence, not who produced it.
` + charterClose

// Independent reports whether the agent is independent. Agents are not unless
// the file says squadIndependent: true.
func Independent(a *agentfile.Agent) bool {
	v, ok := a.Get(IndependentKey)
	if !ok {
		return false
	}
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return slices.Contains([]string{"true", "yes", "on"}, strings.ToLower(x))
	}
	return false
}

// SetIndependent makes the agent independent or returns it to the default. Not
// being independent is the default, so it is never written out.
func SetIndependent(a *agentfile.Agent, independent bool) {
	if independent {
		a.Set(IndependentKey, true)
		return
	}
	a.Delete(IndependentKey)
}

// Compile returns the agent files as Claude Code should get them: without the
// squad-only field and, for an independent agent, with the charter in its prompt
// and a note in its description for whoever calls it. The main agent gets the
// flow brief. Untouched files keep their exact bytes.
func Compile(b preset.Bundle) ([]preset.AgentFile, error) {
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
		independent := Independent(a)
		a.Delete(IndependentKey)
		a.Delete(LegacyShareKey)

		desc := strings.TrimSuffix(a.Description(), descriptionNote)
		if independent {
			desc += descriptionNote
		}
		if desc != a.Description() {
			a.Set("description", desc)
		}

		body := a.Prompt()
		clean := stripBlock(stripBlock(body, charterOpen, charterClose), leadOpen, leadClose)
		add := ""
		if independent {
			add += charter
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

// Brief is what the main agent is told about the squad: the order the agents
// work in and the review loop, when the preset has a flow, and which agents are
// independent and so are given nothing but the task. It is empty when there is
// nothing to tell. The main agent itself is not a step.
func Brief(b preset.Bundle) string {
	var steps, names, independent []string
	for _, f := range b.Agents {
		if b.Preset.Main != nil && f.Name == *b.Preset.Main {
			continue
		}
		names = append(names, f.Name)
		steps = append(steps, fmt.Sprintf("%d %s", len(steps)+1, f.Name))
		if a, err := agentfile.Parse(f.Data); err == nil && Independent(a) {
			independent = append(independent, "`"+f.Name+"`")
		}
	}
	flow := b.Preset.Flow
	if (flow == nil || len(steps) < 2) && len(independent) == 0 {
		return ""
	}

	var s strings.Builder
	s.WriteString(leadOpen + "\n## Squad\n")
	if flow != nil && len(steps) >= 2 {
		s.WriteString("Delegate in this order, one agent per step, giving each its task:\n" + strings.Join(steps, " → ") + "\n")
	}
	if len(independent) > 0 {
		s.WriteString("Independent: " + strings.Join(independent, ", ") + ". Give them only the original task and the instructions, with the material they need to work on (files, a diff). Never your own conclusions, what other agents reasoned or reported, or what you expect them to find. Pass what they report on exactly as it is.\n")
	}
	if flow != nil && len(steps) >= 2 {
		if flow.Loops() {
			s.WriteString(loopText(flow, names))
		} else {
			s.WriteString("When the last step is done, finish with a summary of at most 5 lines.\n")
		}
	}
	s.WriteString(leadClose)
	return s.String()
}

func loopText(f *preset.Flow, names []string) string {
	clean := "With no defects, finish with a summary of at most 5 lines."
	if i := slices.Index(names, f.Gate); i >= 0 && i < len(names)-1 {
		clean = fmt.Sprintf("With no defects, go on to the next step, `%s`; when the last step is done, finish with a summary of at most 5 lines.", names[i+1])
	}
	return fmt.Sprintf("Loop: after `%s` finishes, if it reports defects, give `%s` only those defects and run it, then run `%s` again. %s After %d rounds with defects left, stop and show the user what is still open.\n",
		f.Gate, f.Fix, f.Gate, clean, f.MaxRounds())
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

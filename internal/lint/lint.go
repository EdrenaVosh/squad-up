// Package lint checks agents and presets for mistakes Claude Code would not
// report: it silently ignores unknown fields, loads one of two agents with the
// same name, and delegates badly to agents whose description does not say when
// to call them.
package lint

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/EdrenaVosh/squadup/internal/agentfile"
	"github.com/EdrenaVosh/squadup/internal/models"
	"github.com/EdrenaVosh/squadup/internal/preset"
	"github.com/EdrenaVosh/squadup/internal/team"
)

// Level is the severity of a finding.
type Level string

// Severities.
const (
	LevelError Level = "error" // squad or Claude Code will misbehave
	LevelWarn  Level = "warn"  // probably a mistake
	LevelInfo  Level = "info"  // worth knowing
)

// minDescription is the length below which a description is too thin to
// delegate by.
const minDescription = 40

// similarity is the word overlap above which two descriptions compete.
const similarity = 0.8

// Finding is one problem.
type Finding struct {
	Level   Level  `json:"level"`
	Rule    string `json:"rule"`
	File    string `json:"file,omitempty"`
	Message string `json:"message"`
	Fix     string `json:"fix,omitempty"`
}

// Agent is an agent file to check; File is how it is shown to the user.
type Agent struct {
	File string
	Data []byte
}

// Input is what to check: agents, and for a preset its preset.yaml.
type Input struct {
	Name   string
	Preset *preset.Preset // nil for Initial, whose files can be anywhere
	Agents []Agent
}

// AgentCost is the estimated size of one agent in tokens.
type AgentCost struct {
	Name              string `json:"name"`
	DescriptionTokens int    `json:"descriptionTokens"` // loaded into every session
	PromptTokens      int    `json:"promptTokens"`      // loaded when the agent is called
}

// Cost estimates what a preset costs in context. Tokens are characters ÷ 4,
// so the numbers are approximate.
type Cost struct {
	SessionTokens int         `json:"sessionTokens"` // names and descriptions of all agents
	Agents        []AgentCost `json:"agents"`
}

// Report is the result of checking one input.
type Report struct {
	Name     string    `json:"name"`
	Findings []Finding `json:"findings"`
	Cost     Cost      `json:"cost"`
}

// Count returns how many findings have the level.
func (r Report) Count(level Level) int {
	n := 0
	for _, f := range r.Findings {
		if f.Level == level {
			n++
		}
	}
	return n
}

type loaded struct {
	file  string
	agent *agentfile.Agent
}

// Run checks the input.
func Run(in Input) Report {
	r := Report{Name: in.Name, Findings: []Finding{}}
	add := func(level Level, rule, file, message, fix string) {
		r.Findings = append(r.Findings, Finding{Level: level, Rule: rule, File: file, Message: message, Fix: fix})
	}

	var files []loaded
	for _, f := range in.Agents {
		a, err := agentfile.Parse(f.Data)
		if err != nil {
			add(LevelError, "unreadable", f.File, fmt.Sprintf("%s cannot be read as an agent: %v", f.File, err),
				"an agent file starts with a --- line, has name and description in YAML, then ends the frontmatter with another ---")
			continue
		}
		files = append(files, loaded{file: f.File, agent: a})
	}

	if in.Preset != nil {
		presetRules(in, files, add)
	}
	for _, f := range files {
		agentRules(f, in.Preset != nil, add)
	}
	crossRules(files, add)
	r.Cost = cost(files)
	return r
}

type adder func(level Level, rule, file, message, fix string)

func presetRules(in Input, files []loaded, add adder) {
	p := in.Preset
	if err := p.Validate(); err != nil {
		for _, line := range strings.Split(err.Error(), "\n") {
			add(LevelError, "invalid-preset", "preset.yaml", line, "")
		}
	}
	if len(p.Agents) == 0 {
		add(LevelInfo, "no-agents", "preset.yaml", "the preset has no agents; it only sets settings", "")
	}
	if p.Main != nil && !slices.Contains(p.Agents, *p.Main) {
		add(LevelWarn, "missing-main", "preset.yaml",
			fmt.Sprintf("main agent %q is not one of the preset's agents; that works only if Claude Code finds it elsewhere (a plugin, your own agents)", *p.Main),
			"add the agent to the preset or leave `main` empty")
	}

	teamRules(p, files, add)

	have := map[string]bool{}
	for _, in := range in.Agents {
		have[strings.TrimSuffix(in.File, ".md")] = true
	}
	for _, name := range p.Agents {
		if !have[name] {
			add(LevelError, "missing-agent-file", "agents/"+name+".md", fmt.Sprintf("agent %q is listed in preset.yaml but agents/%s.md does not exist", name, name),
				"restore the file or remove the agent from the `agents` list")
		}
	}
	for _, f := range files {
		stem := strings.TrimSuffix(f.file, ".md")
		if name := f.agent.Name(); name != "" && name != stem {
			add(LevelError, "name-mismatch", f.file, fmt.Sprintf("%s defines the agent %q, but the preset lists it as %q", f.file, name, stem),
				"make the name in the file match the file name")
		}
	}

	if perms := p.Settings.Permissions; perms != nil {
		if perms.DefaultMode == "bypassPermissions" {
			add(LevelWarn, "wide-permissions", "preset.yaml", "defaultMode is bypassPermissions: every tool call runs without asking", "use acceptEdits, or list the commands you trust in `allow`")
		}
		for _, rule := range perms.Allow {
			switch rule {
			case "Bash", "Bash(*)", "WebFetch", "WebFetch(*)":
				add(LevelWarn, "broad-allow", "preset.yaml", fmt.Sprintf("allow rule %q lets Claude Code run anything of that kind without asking", rule), "narrow it, for example Bash(npm run *)")
			}
		}
	}
}

// teamRules checks what the flow asks of the agents' tools.
func teamRules(p *preset.Preset, files []loaded, add adder) {
	if p.Flow == nil || p.Main == nil {
		return
	}
	for _, f := range files {
		if strings.TrimSuffix(f.file, ".md") == *p.Main && !team.CanDelegate(f.agent) {
			add(LevelWarn, "lead-cannot-delegate", f.file,
				fmt.Sprintf("the main agent %q runs the flow, but its tools do not include Agent, so it cannot hand work to the others", *p.Main),
				"add Agent to its tools")
		}
	}
}

var triggerWords = regexp.MustCompile(`(?i)\b(use|when|after|before)\b`)

func agentRules(f loaded, inPreset bool, add adder) {
	a := f.agent
	name, desc := a.Name(), a.Description()

	switch {
	case name == "":
		add(LevelError, "missing-name", f.file, f.file+" has no name", "add `name:` to the frontmatter")
	case !preset.ValidName(name):
		add(LevelError, "invalid-name", f.file, fmt.Sprintf("agent name %q is not valid: use lowercase letters, digits and dashes, up to 64 characters", name), "rename the agent")
	}

	switch desc {
	case "":
		add(LevelError, "missing-description", f.file, f.file+" has no description", "add `description:` saying what the agent does and when to call it")
	default:
		if utf8.RuneCountInString(desc) < minDescription {
			add(LevelWarn, "short-description", f.file, fmt.Sprintf("the description of %q is shorter than %d characters; Claude delegates by description", label(name, f.file), minDescription), "say what the agent does and when to use it")
		}
		if !triggerWords.MatchString(desc) {
			add(LevelWarn, "no-trigger", f.file, fmt.Sprintf("the description of %q does not say when to call the agent", label(name, f.file)), "add a sentence like “Use when …” or “Use after …”")
		}
	}

	if strings.TrimSpace(a.Prompt()) == "" {
		add(LevelWarn, "empty-prompt", f.file, fmt.Sprintf("%q has an empty prompt", label(name, f.file)), "write the instructions the agent should follow")
	}

	for _, key := range a.Keys() {
		if !slices.Contains(agentfile.KnownKeys, key) && key != team.IndependentKey && key != team.LegacyShareKey {
			msg := fmt.Sprintf("unknown field %q: Claude Code ignores it", key)
			if s := suggest(key); s != "" {
				msg += fmt.Sprintf("; did you mean %q?", s)
			}
			add(LevelWarn, "unknown-field", f.file, msg, "")
		}
	}
	fieldRules(f, add)

	if _, ok := a.Get("tools"); !ok {
		add(LevelInfo, "inherits-all-tools", f.file, fmt.Sprintf("%q has no `tools` list, so it can use every tool", label(name, f.file)), "list only the tools it needs")
	}
}

func fieldRules(f loaded, add adder) {
	a := f.agent
	bad := func(level Level, rule, key, want string) {
		v, _ := a.Get(key)
		add(level, rule, f.file, fmt.Sprintf("%s is %v: %s", key, v, want), "")
	}
	str := func(key string) (string, bool) {
		v, ok := a.Get(key)
		if !ok {
			return "", false
		}
		return fmt.Sprint(v), true
	}

	if v, ok := str("model"); ok && !models.ValidAgentModel(v) {
		bad(LevelError, "bad-model", "model", "use inherit, fable, opus, sonnet, haiku or a full model ID such as claude-sonnet-5-5")
	}
	if v, ok := str("effort"); ok && !slices.Contains(preset.EffortLevels, v) {
		bad(LevelError, "bad-effort", "effort", fmt.Sprintf("use one of %s", strings.Join(preset.EffortLevels, ", ")))
	}
	if v, ok := str("permissionMode"); ok {
		switch {
		case !slices.Contains(append([]string{"manual"}, preset.PermissionModes...), v):
			bad(LevelError, "bad-permission-mode", "permissionMode", fmt.Sprintf("use one of %s", strings.Join(preset.PermissionModes, ", ")))
		case v == "bypassPermissions":
			add(LevelWarn, "wide-permissions", f.file, "permissionMode bypassPermissions: the agent runs every tool call without asking", "use acceptEdits or default")
		case v == "auto" || v == "dontAsk":
			add(LevelInfo, "wide-permissions", f.file, fmt.Sprintf("permissionMode %s changes how the agent is asked for permission", v), "")
		}
	}
	if v, ok := str("memory"); ok && !slices.Contains([]string{"user", "project", "local"}, v) {
		bad(LevelError, "bad-memory", "memory", "use user, project or local")
	}
	if v, ok := str("isolation"); ok && v != "worktree" {
		bad(LevelError, "bad-isolation", "isolation", "the only value is worktree")
	}
	if v, ok := str("color"); ok && !slices.Contains(preset.Accents, v) {
		bad(LevelWarn, "bad-color", "color", fmt.Sprintf("use one of %s", strings.Join(preset.Accents, ", ")))
	}
	if raw, ok := a.Get("maxTurns"); ok {
		if n, isInt := asInt(raw); !isInt || n < 1 {
			bad(LevelError, "bad-max-turns", "maxTurns", "use a whole number of 1 or more")
		}
	}
	if raw, ok := a.Get("autoCompactWindow"); ok {
		if n, isInt := asInt(raw); !isInt || n < 100_000 || n > 1_000_000 {
			bad(LevelError, "bad-auto-compact-window", "autoCompactWindow", "use a token count from 100000 to 1000000")
		}
	}
	for _, key := range []string{"tools", "disallowedTools"} {
		raw, ok := a.Get(key)
		if !ok {
			continue
		}
		switch list := raw.(type) {
		case string:
		case []any:
			for _, item := range list {
				if _, isString := item.(string); !isString {
					bad(LevelError, "bad-tools", key, "use a comma-separated string or a list of tool names")
					break
				}
			}
		default:
			bad(LevelError, "bad-tools", key, "use a comma-separated string or a list of tool names")
		}
	}
}

func asInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case uint64:
		return int(n), true
	case float64:
		if n == float64(int(n)) {
			return int(n), true
		}
	}
	return 0, false
}

// crossRules checks agents against each other.
func crossRules(files []loaded, add adder) {
	firstFile := map[string]string{}
	for _, f := range files {
		name := f.agent.Name()
		if name == "" {
			continue
		}
		if first, dup := firstFile[name]; dup {
			add(LevelError, "duplicate-name", f.file, fmt.Sprintf("agent name %q is also defined in %s; Claude Code would load only one of them", name, first), "rename one of them")
			continue
		}
		firstFile[name] = f.file
	}

	for i := range files {
		for j := i + 1; j < len(files); j++ {
			a, b := files[i].agent, files[j].agent
			if a.Description() == "" || b.Description() == "" {
				continue
			}
			if jaccard(words(a.Description()), words(b.Description())) >= similarity {
				add(LevelWarn, "similar-description", files[j].file,
					fmt.Sprintf("the descriptions of %q and %q are almost the same; Claude cannot tell when to pick which", label(a.Name(), files[i].file), label(b.Name(), files[j].file)),
					"make each description say what only that agent does")
			}
		}
	}
}

func cost(files []loaded) Cost {
	c := Cost{Agents: []AgentCost{}}
	for _, f := range files {
		name := f.agent.Name()
		if name == "" {
			continue
		}
		ac := AgentCost{
			Name:              name,
			DescriptionTokens: tokens(name + f.agent.Description()),
			PromptTokens:      tokens(f.agent.Prompt()),
		}
		c.SessionTokens += ac.DescriptionTokens
		c.Agents = append(c.Agents, ac)
	}
	return c
}

// tokens estimates the token count of s as characters ÷ 4, rounded up.
func tokens(s string) int { return (utf8.RuneCountInString(s) + 3) / 4 }

func label(name, file string) string {
	if name != "" {
		return name
	}
	return file
}

func words(s string) map[string]bool {
	set := map[string]bool{}
	for _, w := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		set[w] = true
	}
	return set
}

func jaccard(a, b map[string]bool) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	inter := 0
	for w := range a {
		if b[w] {
			inter++
		}
	}
	return float64(inter) / float64(len(a)+len(b)-inter)
}

// suggest returns the known field closest to key, or "" when none is close.
func suggest(key string) string {
	n := normalize(key)
	best, bestDist := "", 3
	for _, known := range agentfile.KnownKeys {
		k := normalize(known)
		if k == n {
			return known
		}
		if d := distance(n, k); d < bestDist {
			best, bestDist = known, d
		}
	}
	return best
}

func normalize(s string) string {
	return strings.NewReplacer("_", "", "-", "").Replace(strings.ToLower(s))
}

// distance is the Levenshtein distance between two strings.
func distance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur := make([]int, len(rb)+1)
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(rb)]
}

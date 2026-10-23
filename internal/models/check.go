package models

import (
	"fmt"
	"slices"
	"strings"
)

// Verdict is what the policy says about one model.
type Verdict struct {
	Blocked   bool   // the policy forbids the model
	Listed    bool   // the allowlist names it explicitly
	Uncertain bool   // the answer depends on a version that is not known
	Reason    string // why, for Blocked and Uncertain
}

var aliasFamilies = []string{"fable", "opus", "sonnet", "haiku"}

// Check applies the policy to a model. name is an alias or a full ID; id is
// the full ID the name resolves to, or "" when that is not known (a probe
// finds it out).
func (p Policy) Check(name, id string) Verdict {
	base := strings.TrimSuffix(name, "[1m]")
	switch base {
	case "inherit", "default":
		return Verdict{}
	case "opusplan":
		return p.checkAll("opusplan", "opus", "sonnet")
	case "best":
		return p.checkAny("best", "fable", "opus")
	}
	if id == "" && !slices.Contains(aliasFamilies, base) {
		id = base
	}
	family := base
	if !slices.Contains(aliasFamilies, base) {
		family = familyOf(id)
	}

	v := Verdict{}
	for _, denied := range p.Denied {
		d := strings.TrimSuffix(denied, "[1m]")
		switch {
		case slices.Contains(aliasFamilies, d):
			if d == family {
				return Verdict{Blocked: true, Reason: fmt.Sprintf("deniedModels lists %q", denied)}
			}
		case id != "":
			if p.matchSpecific(d, id) {
				return Verdict{Blocked: true, Reason: fmt.Sprintf("deniedModels lists %q", denied)}
			}
		case familyOf(d) == family:
			v.Uncertain = true
			v.Reason = fmt.Sprintf("deniedModels blocks some versions of %s (%q)", family, denied)
		}
	}

	if !p.HasAvailable {
		return v
	}
	if len(p.Available) == 0 {
		return Verdict{Blocked: true, Reason: "availableModels is empty: no named model may be selected"}
	}

	pinned := map[string]bool{} // families that have a version-specific entry
	for _, e := range p.Available {
		if e = strings.TrimSuffix(e, "[1m]"); !slices.Contains(aliasFamilies, e) {
			pinned[familyOf(e)] = true
		}
	}
	allowed := false
	for _, e := range p.Available {
		e = strings.TrimSuffix(e, "[1m]")
		if slices.Contains(aliasFamilies, e) {
			// A version-specific entry in the family disables its wildcard.
			if e == family && !pinned[e] {
				allowed = true
			}
		} else if id != "" && p.matchSpecific(e, id) {
			allowed = true
		}
	}

	switch {
	case allowed:
		v.Listed = true
	case id == "" && pinned[family]:
		v.Uncertain = true
		v.Reason = fmt.Sprintf("availableModels allows only some versions of %s", family)
	default:
		return Verdict{Blocked: true, Reason: fmt.Sprintf("not allowed by availableModels (%s)", strings.Join(p.Available, ", "))}
	}
	return v
}

// checkAll is the verdict for a model that needs all of the parts.
func (p Policy) checkAll(name string, parts ...string) Verdict {
	out := Verdict{Listed: true}
	for _, part := range parts {
		v := p.Check(part, "")
		if v.Blocked {
			return Verdict{Blocked: true, Reason: fmt.Sprintf("%s needs %s: %s", name, part, v.Reason)}
		}
		out.Uncertain = out.Uncertain || v.Uncertain
		out.Listed = out.Listed && v.Listed
	}
	return out
}

// checkAny is the verdict for a model that resolves to one of the parts.
func (p Policy) checkAny(name string, parts ...string) Verdict {
	blocked := 0
	out := Verdict{}
	for _, part := range parts {
		v := p.Check(part, "")
		if v.Blocked {
			blocked++
			continue
		}
		out.Listed = out.Listed || v.Listed
		out.Uncertain = out.Uncertain || v.Uncertain
	}
	if blocked == len(parts) {
		return Verdict{Blocked: true, Reason: fmt.Sprintf("%s resolves to %s, none of which is allowed", name, strings.Join(parts, " or "))}
	}
	return out
}

// matchSpecific matches a version prefix or full ID against an ID. A prefix
// matches later versions that extend it at a "-" boundary, unless the policy
// asks for exact matching.
func (p Policy) matchSpecific(entry, id string) bool {
	if p.Exact {
		return entry == id
	}
	return id == entry || strings.HasPrefix(id, entry+"-")
}

// familyOf returns the family of a full ID or version prefix such as
// "claude-sonnet-4-5": the word after "claude-".
func familyOf(id string) string {
	parts := strings.Split(id, "-")
	if len(parts) >= 2 && parts[0] == "claude" {
		return parts[1]
	}
	return ""
}

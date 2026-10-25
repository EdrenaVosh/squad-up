package models

import (
	"fmt"
	"slices"
	"strings"
)

// Status says how much is known about a model.
type Status string

// Statuses of a model.
const (
	StatusVerified    Status = "verified"          // a probe succeeded, or the name always works
	StatusAllowed     Status = "allowed by policy" // the allowlist names it, but it was not probed
	StatusUnknown     Status = "unknown"           // nothing says either way
	StatusUnavailable Status = "unavailable"       // the policy or a probe says no
)

// Result is the status of one catalogue entry.
type Result struct {
	Name       string
	Agents     bool
	Settings   bool
	Status     Status
	ResolvedTo string // the full model ID, once probed
	Reason     string
}

// Evaluate combines the catalogue, the policy and the probe cache. The policy
// can only take a model away; a probe can confirm or refute it.
func Evaluate(catalog []Entry, policy Policy, cache *Cache) []Result {
	base := map[string]Result{}
	for _, alias := range ProbeAliases {
		base[alias] = evalAlias(alias, policy, cache)
	}

	out := make([]Result, 0, len(catalog))
	for _, e := range catalog {
		r := Result{Name: e.Name, Agents: e.Agents, Settings: e.Settings}
		switch {
		case e.Name == "inherit" || e.Name == "default":
			r.Status, r.Reason = StatusVerified, e.Note
		case e.Name == "opusplan":
			r = derived(r, policy, "opusplan", true, base["opus"], base["sonnet"])
		case e.Name == "best":
			r = derived(r, policy, "best", false, base["fable"], base["opus"])
		case strings.HasSuffix(e.Name, "[1m]"):
			b := base[strings.TrimSuffix(e.Name, "[1m]")]
			r.Status, r.Reason = b.Status, b.Reason
			if b.Status == StatusVerified {
				r.Status, r.Reason = StatusUnknown, "the 1M context window is not probed; it needs a plan that includes it"
			}
		case e.Version:
			b := evalVersion(e.Name, policy, cache)
			r.Status, r.ResolvedTo, r.Reason = b.Status, b.ResolvedTo, b.Reason
		default:
			b := base[e.Name]
			r.Status, r.ResolvedTo, r.Reason = b.Status, b.ResolvedTo, b.Reason
		}
		out = append(out, r)
	}
	return out
}

func evalAlias(alias string, policy Policy, cache *Cache) Result {
	r := Result{Name: alias}
	var probe ProbeResult
	probed := false
	if cache != nil {
		probe, probed = cache.Results[alias]
	}

	id := ""
	if probed && probe.OK {
		id = probe.ID
	}
	verdict := policy.Check(alias, id)

	switch {
	case verdict.Blocked:
		r.Status, r.Reason = StatusUnavailable, "blocked by policy: "+verdict.Reason
	case probed && !probe.OK:
		r.Status, r.Reason = StatusUnavailable, probe.Error
	case probed:
		r.Status, r.ResolvedTo = StatusVerified, probe.ID
	case verdict.Listed:
		r.Status, r.Reason = StatusAllowed, "named in availableModels"
	default:
		r.Status = StatusUnknown
		if verdict.Uncertain {
			r.Reason = verdict.Reason
		}
	}
	return r
}

// evalVersion is the status of a release named by its full ID. A probe of the
// ID says it; so does an alias that resolved to this very ID, without a
// request of its own.
func evalVersion(id string, policy Policy, cache *Cache) Result {
	r := evalAlias(id, policy, cache)
	if r.Status != StatusUnknown && r.Status != StatusAllowed || cache == nil {
		return r
	}
	for _, alias := range ProbeAliases {
		if res, ok := cache.Results[alias]; ok && res.OK && res.ID == id {
			return Result{Name: id, Status: StatusVerified, ResolvedTo: id, Reason: "what " + alias + " resolves to"}
		}
	}
	return r
}

// derived is the status of a name built from other models. needAll means all
// of the parts are needed (opusplan), otherwise one is enough (best).
func derived(r Result, policy Policy, name string, needAll bool, parts ...Result) Result {
	var good, bad []Result
	for _, p := range parts {
		switch p.Status {
		case StatusUnavailable:
			bad = append(bad, p)
		case StatusVerified:
			good = append(good, p)
		}
	}

	switch {
	case needAll && len(bad) > 0:
		r.Status, r.Reason = StatusUnavailable, fmt.Sprintf("%s needs %s, which is unavailable: %s", name, bad[0].Name, bad[0].Reason)
	case !needAll && len(bad) == len(parts):
		r.Status, r.Reason = StatusUnavailable, fmt.Sprintf("%s resolves to %s, none of which is available", name, joinNames(parts))
	case len(good) == len(parts) || (!needAll && len(good) > 0):
		r.Status = StatusVerified
	default:
		r.Status = StatusUnknown
	}
	if v := policy.Check(name, ""); v.Blocked {
		r.Status, r.Reason = StatusUnavailable, "blocked by policy: "+v.Reason
	}
	return r
}

func joinNames(parts []Result) string {
	names := make([]string, len(parts))
	for i, p := range parts {
		names[i] = p.Name
	}
	slices.Sort(names)
	return strings.Join(names, " or ")
}

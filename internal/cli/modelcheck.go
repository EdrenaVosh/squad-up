package cli

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/EdrenaVosh/squadup/internal/agentfile"
	"github.com/EdrenaVosh/squadup/internal/claude"
	"github.com/EdrenaVosh/squadup/internal/models"
	"github.com/EdrenaVosh/squadup/internal/preset"
)

// policy loads the model allowlist from the user's and the managed settings.
func (a *app) policy() (models.Policy, []string) {
	p, notes, _ := models.LoadPolicy(a.paths.ClaudeDir, models.ManagedDirs())
	return p, notes
}

// cachedProbe returns the saved probe when it can still be trusted, judging by
// the claude version only (it is cheap to ask for; the account type is not).
func (a *app) cachedProbe(ctx context.Context) *models.Cache {
	cache, err := models.LoadCache(a.paths)
	if err != nil || cache == nil {
		return nil
	}
	bin, err := claude.Find()
	if err != nil {
		return nil
	}
	version, err := claude.Client{Bin: bin}.Version(ctx)
	if err != nil || !cache.Valid(version, "", a.now()) {
		return nil
	}
	return cache
}

// modelWarnings lists the models a preset uses that the policy or a saved
// probe says the account cannot use.
func (a *app) modelWarnings(ctx context.Context, b *preset.Bundle) []string {
	policy, _ := a.policy()
	results := models.Evaluate(models.Catalog(), policy, a.cachedProbe(ctx))

	unavailable := func(name string) (string, bool) {
		for _, r := range results {
			if r.Name == name {
				return r.Reason, r.Status == models.StatusUnavailable
			}
		}
		// A full model ID: only the policy can say anything about it.
		if v := policy.Check(name, name); v.Blocked {
			return "blocked by policy: " + v.Reason, true
		}
		return "", false
	}

	var out []string
	if m := b.Preset.Settings.Model; m != "" {
		if reason, bad := unavailable(m); bad {
			out = append(out, fmt.Sprintf("the preset's default model %q is unavailable: %s", m, reason))
		}
	}
	for _, file := range b.Agents {
		agent, err := agentfile.Parse(file.Data)
		if err != nil {
			continue
		}
		raw, ok := agent.Get("model")
		if !ok {
			continue
		}
		model := fmt.Sprint(raw)
		if reason, bad := unavailable(model); bad {
			out = append(out, fmt.Sprintf("agent %q uses the model %q, which is unavailable: %s; edit the agent or set `model: inherit`", file.Name, model, reason))
		}
	}
	return out
}

// verifiedAndUnavailable lists the probed aliases by outcome, alphabetically.
func verifiedAndUnavailable(results []models.Result) (verified, unavailable []string) {
	verified, unavailable = []string{}, []string{}
	for _, r := range results {
		if !slices.Contains(models.ProbeAliases, r.Name) {
			continue
		}
		switch r.Status {
		case models.StatusVerified:
			verified = append(verified, r.Name)
		case models.StatusUnavailable:
			unavailable = append(unavailable, r.Name)
		}
	}
	slices.Sort(verified)
	slices.Sort(unavailable)
	return verified, unavailable
}

func age(now, then time.Time) string {
	d := now.Sub(then).Round(time.Minute)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	}
	return fmt.Sprintf("%d h ago", int(d.Hours()))
}

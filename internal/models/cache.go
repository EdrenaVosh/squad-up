package models

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/EdrenaVosh/squadup/internal/claude"
	"github.com/EdrenaVosh/squadup/internal/fsx"
	"github.com/EdrenaVosh/squadup/internal/paths"
)

// cacheTTL is how long probe results stay valid.
const cacheTTL = 24 * time.Hour

// ProbeResult is the outcome of probing one alias.
type ProbeResult struct {
	OK    bool   `json:"ok"`
	ID    string `json:"id,omitempty"`    // the full model ID the alias resolved to
	Error string `json:"error,omitempty"` // why the model cannot be used
}

// Cache holds the results of a probe. It is keyed by the Claude Code version
// and the kind of account, because both change what is available.
type Cache struct {
	ClaudeVersion string                 `json:"claudeVersion"`
	AuthMethod    string                 `json:"authMethod"`
	ProbedAt      time.Time              `json:"probedAt"`
	Results       map[string]ProbeResult `json:"results"`
}

func cachePath(p paths.Paths) string { return filepath.Join(p.CacheDir(), "models.json") }

// LoadCache reads the cache. A missing or damaged cache is simply absent.
func LoadCache(p paths.Paths) (*Cache, error) {
	data, err := os.ReadFile(cachePath(p))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", cachePath(p), err)
	}
	var c Cache
	if json.Unmarshal(data, &c) != nil {
		return nil, nil
	}
	return &c, nil
}

// SaveCache writes the cache.
func SaveCache(p paths.Paths, c *Cache) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return fsx.WriteFileAtomic(cachePath(p), append(data, '\n'), 0o600)
}

// Valid reports whether the cache can still be trusted. An empty authMethod
// skips the account comparison, for callers that do not know it.
func (c *Cache) Valid(claudeVersion, authMethod string, now time.Time) bool {
	return c != nil &&
		c.ClaudeVersion == claudeVersion &&
		(authMethod == "" || c.AuthMethod == authMethod) &&
		now.Sub(c.ProbedAt) < cacheTTL
}

// Probe tests the aliases in parallel, each with its own timeout.
func Probe(ctx context.Context, c claude.Client, aliases []string, timeout time.Duration) map[string]ProbeResult {
	return ProbeEach(ctx, c, aliases, timeout, nil)
}

// ProbeEach is Probe that also tells each result as soon as it is known, from
// the goroutine that found it, so that a screen can show the progress.
func ProbeEach(ctx context.Context, c claude.Client, aliases []string, timeout time.Duration, each func(alias string, res ProbeResult)) map[string]ProbeResult {
	results := make(map[string]ProbeResult, len(aliases))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, alias := range aliases {
		wg.Add(1)
		go func() {
			defer wg.Done()
			probeCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			id, err := c.Probe(probeCtx, alias)
			res := ProbeResult{OK: err == nil, ID: id}
			if err != nil {
				res.Error = err.Error()
			}
			mu.Lock()
			results[alias] = res
			mu.Unlock()
			if each != nil {
				each(alias, res)
			}
		}()
	}
	wg.Wait()
	return results
}

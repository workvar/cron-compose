// Package updates watches upstream agent releases and exposes update status to the UI.
package updates

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/croncompose/croncompose/control-plane/internal/agentgw"
)

// DefaultGitHubURLTemplate is the download URL pattern for official GitHub releases.
const DefaultGitHubURLTemplate = "https://github.com/{repo}/releases/download/{version}/croncompose-agent-{target}"

// Release is one pinned agent build the control plane can offer to agents.
type Release struct {
	Version     string            `json:"version"`
	Tag         string            `json:"tag"`
	ReleaseURL  string            `json:"release_url"`
	PublishedAt time.Time         `json:"published_at"`
	Checksums   map[string]string `json:"checksums"` // os/arch -> sha256 hex
}

type Catalog struct {
	log      *slog.Logger
	repo     string
	restart  bool
	interval time.Duration

	mu      sync.RWMutex
	latest  *Release
	lastErr string
}

// Config tunes how the catalog polls GitHub.
type Config struct {
	Repo     string
	Restart  bool
	Interval time.Duration
}

// NewCatalog returns a catalog that must be started with Run.
func NewCatalog(log *slog.Logger, cfg Config) *Catalog {
	if cfg.Interval <= 0 {
		cfg.Interval = 15 * time.Minute
	}
	return &Catalog{
		log:      log,
		repo:     cfg.Repo,
		restart:  cfg.Restart,
		interval: cfg.Interval,
	}
}

// Run polls GitHub until ctx is cancelled.
func (c *Catalog) Run(ctx context.Context) {
	c.refresh(ctx)
	tick := time.NewTicker(c.interval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			c.refresh(ctx)
		}
	}
}

func (c *Catalog) refresh(ctx context.Context) {
	rel, err := fetchLatestRelease(ctx, c.repo)
	c.mu.Lock()
	defer c.mu.Unlock()
	if err != nil {
		c.lastErr = err.Error()
		c.log.Debug("github release check failed", "repo", c.repo, "err", err)
		return
	}
	c.lastErr = ""
	c.latest = rel
	c.log.Info("latest agent release", "version", rel.Version, "tag", rel.Tag)
}

// Snapshot returns the cached latest release, if any.
func (c *Catalog) Snapshot() (Release, bool, string) {
	if c == nil {
		return Release{}, false, ""
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.latest == nil {
		return Release{}, false, c.lastErr
	}
	return *c.latest, true, c.lastErr
}

// Policy builds an UpdatePolicy from the cached release.
func (c *Catalog) Policy() agentgw.UpdatePolicy {
	if c == nil {
		return agentgw.UpdatePolicy{}
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.latest == nil {
		return agentgw.UpdatePolicy{}
	}
	return agentgw.UpdatePolicy{
		Version:     c.latest.Version,
		URLTemplate: DefaultGitHubURLTemplate,
		Repo:        c.repo,
		Checksums:   c.latest.Checksums,
		Restart:     c.restart,
	}
}

// NeedsUpdate reports whether current is behind the cached latest release.
func (c *Catalog) NeedsUpdate(current string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.latest == nil || current == "" {
		return false
	}
	return agentgw.VersionNewer(current, c.latest.Version)
}

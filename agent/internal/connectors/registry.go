package connectors

import (
	"context"
	"log/slog"
	"time"
)

// Discovered bundles one instance with its resources for the discovery push.
type Discovered struct {
	Instance  Instance
	Resources []Resource
}

// Registry holds the enabled providers.
type Registry struct {
	log       *slog.Logger
	providers []Provider
}

// NewRegistry returns the default Phase A provider set. Order is cosmetic. Additional
// connectors (apache, caddy, traefik, haproxy, cron, ufw) plug in here as they land.
func NewRegistry(log *slog.Logger) *Registry {
	return &Registry{
		log: log,
		providers: []Provider{
			&nginxProvider{},
			&systemdProvider{},
			&dockerProvider{},
			&pm2Provider{},
		},
	}
}

// Discover probes every provider and returns what is installed. Each provider gets a
// bounded context so one hung command cannot stall the whole sweep.
func (r *Registry) Discover(ctx context.Context) []Discovered {
	var out []Discovered
	for _, p := range r.providers {
		pctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		for _, inst := range p.Detect(pctx) {
			out = append(out, Discovered{Instance: inst, Resources: p.Resources(pctx, inst)})
		}
		cancel()
	}
	return out
}

// Resolve finds the provider for a kind and the specific instance a command targets.
//
// The instance is re-detected rather than served from a cache on purpose: a command
// acts on the box as it is right now, and capabilities (is nginx.conf still writable?
// is the agent still in the docker group?) are exactly the kind of thing that changes
// between a discovery sweep and an operator clicking a button. An empty wanted
// instance matches the first one, which is the singleton case.
func (r *Registry) Resolve(ctx context.Context, kind, wanted string) (Provider, Instance, bool) {
	for _, p := range r.providers {
		if p.Kind() != kind {
			continue
		}
		for _, inst := range p.Detect(ctx) {
			if wanted == "" || inst.Instance == wanted {
				return p, inst, true
			}
		}
		return nil, Instance{}, false
	}
	return nil, Instance{}, false
}

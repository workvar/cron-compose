// Package connectors discovers (and, in later phases, manages) the service managers
// running on the host: nginx, systemd, docker, pm2, and so on. Phase A is read-only:
// each provider reports what is installed, its status, config paths, and the inventory
// of config files and lifecycle objects. The runtime ships this up the gRPC stream as a
// ConnectorEvent. See docs/connectors.md.
package connectors

import "context"

// Capabilities reports what the agent can actually do for a connector on THIS host,
// given installed binaries and privileges. The UI only offers enabled actions; in Phase
// A these flags are informational (read-only).
type Capabilities struct {
	ManagesConfig  bool
	ManagesObjects bool
	CanValidate    bool
	CanReload      bool
	CanLifecycle   bool
	CanEdit        bool
}

// Instance is one discovered connector instance (for example the nginx install, or pm2
// for a given user).
type Instance struct {
	Kind        string
	Instance    string // e.g. pm2 user; empty for singletons
	Version     string
	Status      string // running|stopped|degraded|unknown
	Manageable  bool
	ConfigPaths []string
	ObjectCount int
	Caps        Capabilities
	Detail      map[string]string
}

// Resource is a config file or lifecycle object within a connector.
type Resource struct {
	Type       string // config_file|object
	Ref        string
	Name       string
	State      string
	Checksum   string
	SizeBytes  int64
	Attributes map[string]string
}

// Provider is the read-only contract for Phase A. Lifecycle and config-write methods are
// added in later phases. Providers are best-effort: a missing tool means "not installed",
// returned as a nil slice rather than an error.
type Provider interface {
	Kind() string
	// Detect returns installed instances with version, status, paths, and capabilities.
	Detect(ctx context.Context) []Instance
	// Resources returns the config files and/or objects for one instance.
	Resources(ctx context.Context, inst Instance) []Resource
}

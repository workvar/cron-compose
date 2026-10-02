// Package config loads agent settings from environment + a small config file.
package config

import (
	"fmt"
	"os"
	"strings"
)

// buildVersion is set at link time by the release workflow (-ldflags -X ...).
var buildVersion = "0.1.0-dev"

// Config is everything the agent needs at runtime.
type Config struct {
	ControlPlaneAddr     string // host:port of the gRPC endpoint
	ControlPlaneHTTPBase string // base URL for REST calls (enrollment)
	ControlPlaneSNI      string // server name to verify against in TLS
	// GRPCAddrSet is true when the operator set AGENT_GRPC_ADDR (or the legacy name).
	// An explicit address beats the one the control plane advertised at enrollment.
	GRPCAddrSet bool
	// GRPCMode is AGENT_GRPC_MODE as set (empty when unset); see transport.ResolveMode.
	GRPCMode     string
	Warnings     []string // conflicting endpoint settings, logged by the caller
	DataDir      string   // where the local store, cert, and key live
	AgentVersion string   // injected at build time or hard-coded
	// SelfUpdate lets the control plane replace this agent's binary. On by default:
	// the install paths all run the agent under a supervisor that restarts it. Set
	// AGENT_SELF_UPDATE=0 on a hand-managed box where nothing would bring it back.
	SelfUpdate bool
}

// Load reads env vars with dev-friendly defaults.
func Load() (Config, error) {
	addr, w1 := endpointEnv(envGRPCAddr, legacyGRPCAddr, "localhost:9090")
	httpBase, w2 := endpointEnv(envEnrollHTTP, legacyEnrollHTTP, "http://localhost:8080/api/v1")
	sni, w3 := endpointEnv(envGRPCSNI, legacyGRPCSNI, "localhost")
	c := Config{
		ControlPlaneAddr:     addr,
		GRPCAddrSet:          os.Getenv(envGRPCAddr) != "" || os.Getenv(legacyGRPCAddr) != "",
		GRPCMode:             os.Getenv("AGENT_GRPC_MODE"),
		ControlPlaneHTTPBase: httpBase,
		ControlPlaneSNI:      sni,
		DataDir:              env("DATA_DIR", defaultDataDir),
		AgentVersion:         resolveAgentVersion(),
		SelfUpdate:           envBool("AGENT_SELF_UPDATE", true),
	}
	for _, w := range []string{w1, w2, w3} {
		if w != "" {
			c.Warnings = append(c.Warnings, w)
		}
	}
	if c.ControlPlaneAddr == "" {
		return c, fmt.Errorf("%s is required", envGRPCAddr)
	}
	return c, nil
}

// resolveAgentVersion prefers the version linked into the binary. Installers used
// to pin Environment=AGENT_VERSION in the systemd unit, which made Hello keep
// reporting the install-time tag after a successful self-update. AGENT_VERSION
// still wins for local/dev binaries whose linked version is the placeholder.
func resolveAgentVersion() string {
	linked := strings.TrimSpace(buildVersion)
	if linked != "" && linked != "0.1.0-dev" {
		return linked
	}
	return env("AGENT_VERSION", buildVersion)
}

// envBool reads a boolean env var. Anything that is plainly a "no" turns it off; an
// unset or unrecognised value keeps the default, because a typo should not silently
// disable a safety-relevant setting in either direction.
func envBool(key string, def bool) bool {
	switch os.Getenv(key) {
	case "":
		return def
	case "0", "false", "no", "off":
		return false
	case "1", "true", "yes", "on":
		return true
	}
	return def
}

// Version returns the agent version baked in at link time.
func Version() string {
	return buildVersion
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

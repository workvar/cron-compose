// Package config loads agent settings from environment + a small config file.
package config

import (
	"fmt"
	"os"
)

// Config is everything the agent needs at runtime.
type Config struct {
	ControlPlaneAddr     string // host:port of the gRPC endpoint
	ControlPlaneHTTPBase string // base URL for REST calls (enrollment)
	ControlPlaneSNI      string // server name to verify against in TLS
	DataDir              string // where the local store, cert, and key live
	AgentVersion         string // injected at build time or hard-coded
	// SelfUpdate lets the control plane replace this agent's binary. On by default:
	// the install paths all run the agent under a supervisor that restarts it. Set
	// AGENT_SELF_UPDATE=0 on a hand-managed box where nothing would bring it back.
	SelfUpdate bool
}

// Load reads env vars with dev-friendly defaults.
func Load() (Config, error) {
	c := Config{
		ControlPlaneAddr:     env("CONTROL_PLANE_ADDR", "localhost:9090"),
		ControlPlaneHTTPBase: env("CONTROL_PLANE_HTTP", "http://localhost:8080/api/v1"),
		ControlPlaneSNI:      env("CONTROL_PLANE_SNI", "localhost"),
		DataDir:              env("DATA_DIR", defaultDataDir),
		AgentVersion:         env("AGENT_VERSION", "0.1.0-dev"),
		SelfUpdate:           envBool("AGENT_SELF_UPDATE", true),
	}
	if c.ControlPlaneAddr == "" {
		return c, fmt.Errorf("CONTROL_PLANE_ADDR is required")
	}
	return c, nil
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

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

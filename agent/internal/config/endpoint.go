package config

import (
	"fmt"
	"os"
)

// The agent's endpoint settings have their own names so they can never be confused
// with the control plane's (GRPC_ADDR, PUBLIC_GRPC_ADDR, ...) when both live in one
// .env on the same host. The old CONTROL_PLANE_* names still work as a fallback.
const (
	envGRPCAddr   = "AGENT_GRPC_ADDR"
	envGRPCSNI    = "AGENT_GRPC_SNI"
	envEnrollHTTP = "AGENT_ENROLL_HTTP"

	legacyGRPCAddr   = "CONTROL_PLANE_ADDR"
	legacyGRPCSNI    = "CONTROL_PLANE_SNI"
	legacyEnrollHTTP = "CONTROL_PLANE_HTTP"
)

// endpointEnv returns the value of name, else of the legacy variable, else def. A
// warning is returned when both are set to different values: the new name wins, so a
// stale legacy override (an old systemd drop-in, say) would otherwise be ignored
// without a trace.
func endpointEnv(name, legacy, def string) (string, string) {
	v, old := os.Getenv(name), os.Getenv(legacy)
	switch {
	case v != "" && old != "" && v != old:
		return v, fmt.Sprintf("%s=%q is used and %s=%q is ignored; remove the stale one", name, v, legacy, old)
	case v != "":
		return v, ""
	case old != "":
		return old, ""
	}
	return def, ""
}

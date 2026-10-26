//go:build !windows

package config

// defaultEnvFile is empty on Unix: systemd and launchd inject the environment
// themselves. AGENT_ENV_FILE still works when set explicitly.
var defaultEnvFile = ""

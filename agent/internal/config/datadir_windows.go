//go:build windows

package config

import (
	"os"
	"path/filepath"
)

// defaultDataDir is %ProgramData%\CronCompose, the machine-wide state folder for a
// service. The installer creates it and restricts it to SYSTEM and Administrators.
var defaultDataDir = filepath.Join(programData(), "CronCompose")

// defaultEnvFile is the Windows equivalent of the systemd EnvironmentFile: a service
// has no per-service environment of its own, so settings live beside the state.
var defaultEnvFile = filepath.Join(defaultDataDir, "agent.env")

func programData() string {
	if d := os.Getenv("ProgramData"); d != "" {
		return d
	}
	return `C:\ProgramData`
}

//go:build !windows

package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/croncompose/croncompose/agent/internal/config"
)

// runningAsService is always false off Windows: systemd and launchd run `agent run`
// as an ordinary foreground process.
func runningAsService() bool { return false }

func runService(config.Config) int { return 1 }

// bootHooks has nothing to do on Unix.
func bootHooks(context.Context, *slog.Logger) {}

func serviceCommand([]string) int {
	fmt.Fprintln(os.Stderr, "agent service is for Windows; on Linux use systemd, on macOS launchd")
	return 2
}

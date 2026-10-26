//go:build windows

package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"golang.org/x/sys/windows/svc"

	"github.com/croncompose/croncompose/agent/internal/config"
)

// serviceName is the Windows service the installer registers.
const serviceName = "croncompose-agent"

// runningAsService reports whether the Service Control Manager started us.
func runningAsService() bool {
	ok, err := svc.IsWindowsService()
	return err == nil && ok
}

// runService hands control to the SCM. Logs go to a file because a service has no
// console.
func runService(cfg config.Config) int {
	log, closeLog := fileLogger(cfg.DataDir)
	defer closeLog()
	h := &agentService{cfg: cfg, log: log}
	if err := svc.Run(serviceName, h); err != nil {
		log.Error("service dispatcher failed", "err", err)
		return 1
	}
	return int(h.exitCode)
}

type agentService struct {
	cfg      config.Config
	log      *slog.Logger
	exitCode uint32
}

// Execute runs the agent until the SCM asks it to stop. If the agent loop ends on
// its own with an error, a non-zero service exit code lets the SCM's recovery
// actions (set by `agent service install`) restart it. A self-update exits the
// process directly, which the SCM also treats as a failure and restarts, now on
// the new binary.
func (a *agentService) Execute(_ []string, req <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	status <- svc.Status{State: svc.StartPending}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan int, 1)
	go func() { done <- runAgent(ctx, a.cfg, a.log) }()
	status <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	a.log.Info("service running", "name", serviceName)

	for {
		select {
		case c := <-req:
			switch c.Cmd {
			case svc.Interrogate:
				status <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				a.log.Info("service stop requested")
				status <- svc.Status{State: svc.StopPending, WaitHint: 20000}
				cancel()
				select {
				case <-done:
				case <-time.After(20 * time.Second):
					a.log.Warn("agent did not stop within 20s; exiting anyway")
				}
				return false, 0
			}
		case code := <-done:
			if code != 0 {
				a.exitCode = uint32(code)
				return true, a.exitCode
			}
			return false, 0
		}
	}
}

func serviceCommand(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: agent service [install|uninstall|start|stop]")
		return 2
	}
	var err error
	switch args[0] {
	case "install":
		err = installService()
	case "uninstall":
		err = uninstallService()
	case "start":
		err = startService()
	case "stop":
		err = stopService()
	default:
		fmt.Fprintln(os.Stderr, "unknown service verb:", args[0])
		return 2
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println("ok:", serviceName, args[0])
	return 0
}

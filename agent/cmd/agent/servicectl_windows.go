//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// recoveryActions restart the agent after any exit the SCM did not ask for: a
// crash, a fatal error, or the deliberate exit that finishes a self-update. The
// last action repeats for every later failure, so updates never exhaust it.
var recoveryActions = []mgr.RecoveryAction{
	{Type: mgr.ServiceRestart, Delay: 2 * time.Second},
	{Type: mgr.ServiceRestart, Delay: 5 * time.Second},
	{Type: mgr.ServiceRestart, Delay: 15 * time.Second},
}

// installService registers (or re-registers) the agent as an automatic,
// LocalSystem service pointing at this executable. Idempotent: rerunning it after
// moving the binary updates the existing service in place.
func installService() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, _ = filepath.Abs(exe)
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to service manager (run as Administrator): %w", err)
	}
	defer m.Disconnect()

	cfg := mgr.Config{
		DisplayName:  "CronCompose Agent",
		Description:  "Runs CronCompose jobs and deploys on this server and keeps it connected to the control plane.",
		StartType:    mgr.StartAutomatic,
		ErrorControl: mgr.ErrorNormal,
	}
	s, err := m.OpenService(serviceName)
	if err == nil {
		cfg.BinaryPathName = fmt.Sprintf("%s run", windows.EscapeArg(exe))
		if err := s.UpdateConfig(cfg); err != nil {
			s.Close()
			return fmt.Errorf("update service: %w", err)
		}
	} else {
		s, err = m.CreateService(serviceName, exe, cfg, "run")
		if err != nil {
			return fmt.Errorf("create service: %w", err)
		}
	}
	defer s.Close()
	if err := s.SetRecoveryActions(recoveryActions, uint32((24 * time.Hour).Seconds())); err != nil {
		return fmt.Errorf("set recovery actions: %w", err)
	}
	if err := s.SetRecoveryActionsOnNonCrashFailures(true); err != nil {
		return fmt.Errorf("set recovery on non-crash failures: %w", err)
	}
	return nil
}

func uninstallService() error {
	m, s, err := openService()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	defer s.Close()
	_ = stopAndWait(s)
	return s.Delete()
}

func startService() error {
	m, s, err := openService()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	defer s.Close()
	return s.Start()
}

func stopService() error {
	m, s, err := openService()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	defer s.Close()
	return stopAndWait(s)
}

func openService() (*mgr.Mgr, *mgr.Service, error) {
	m, err := mgr.Connect()
	if err != nil {
		return nil, nil, fmt.Errorf("connect to service manager (run as Administrator): %w", err)
	}
	s, err := m.OpenService(serviceName)
	if err != nil {
		m.Disconnect()
		return nil, nil, fmt.Errorf("open service %s: %w", serviceName, err)
	}
	return m, s, nil
}

func stopAndWait(s *mgr.Service) error {
	st, err := s.Control(svc.Stop)
	if err != nil {
		if errors.Is(err, windows.ERROR_SERVICE_NOT_ACTIVE) {
			return nil
		}
		return err
	}
	deadline := time.Now().Add(30 * time.Second)
	for st.State != svc.Stopped && time.Now().Before(deadline) {
		time.Sleep(300 * time.Millisecond)
		if st, err = s.Query(); err != nil {
			return err
		}
	}
	if st.State != svc.Stopped {
		return errors.New("service did not stop within 30s")
	}
	return nil
}

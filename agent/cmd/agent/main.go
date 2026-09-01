// Command agent is the per-server CronCompose agent.
//
// Subcommands:
//
//	agent enroll --token=<token>   Generates a keypair + CSR, POSTs to the control
//	                               plane's REST enroll endpoint, persists the signed
//	                               cert + CA + identity, then exits.
//	agent run                      Starts the long-lived loop: mTLS dial, sync jobs,
//	                               schedule them locally, execute, stream logs back.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/croncompose/croncompose/agent/internal/config"
	"github.com/croncompose/croncompose/agent/internal/enroll"
	"github.com/croncompose/croncompose/agent/internal/identity"
	"github.com/croncompose/croncompose/agent/internal/mtls"
	"github.com/croncompose/croncompose/agent/internal/outbox"
	agentruntime "github.com/croncompose/croncompose/agent/internal/runtime"
	"github.com/croncompose/croncompose/agent/internal/store"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "enroll":
		os.Exit(cmdEnroll(os.Args[2:]))
	case "run":
		os.Exit(cmdRun(os.Args[2:]))
	case "version":
		fmt.Println("croncompose-agent", config.Version())
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: agent [enroll|run|version] [args...]")
}

func cmdEnroll(args []string) int {
	fs := flag.NewFlagSet("enroll", flag.ExitOnError)
	token := fs.String("token", "", "one-time enrollment token from the control plane UI")
	_ = fs.Parse(args)
	if *token == "" {
		fmt.Fprintln(os.Stderr, "--token is required")
		return 2
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	hostname, _ := os.Hostname()
	csr, err := mtls.GenerateCSR(hostname)
	if err != nil {
		log.Error("csr generate failed", "err", err)
		return 1
	}

	resp, err := enroll.Post(cfg.ControlPlaneHTTPBase, enroll.Request{
		Token:        *token,
		Hostname:     hostname,
		OS:           "linux",
		Arch:         runtime.GOARCH,
		AgentVersion: cfg.AgentVersion,
		CSRPEM:       string(csr.CSRPEM),
	})
	if err != nil {
		log.Error("enroll failed", "err", err)
		return 1
	}

	if err := csr.SaveBundle(cfg.DataDir, []byte(resp.ClientCertPEM), []byte(resp.ServerCAPEM)); err != nil {
		log.Error("save tls bundle failed", "err", err)
		return 1
	}
	id := identity.Identity{
		ServerID:             resp.ServerID,
		ControlPlaneGRPCAddr: resp.ControlPlaneGRPCAddr,
	}
	if err := identity.Save(cfg.DataDir, id); err != nil {
		log.Error("save identity failed", "err", err)
		return 1
	}
	log.Info("enrolled", "server_id", id.ServerID, "data_dir", cfg.DataDir)
	return 0
}

func cmdRun(_ []string) int {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	id, err := identity.Load(cfg.DataDir)
	if err != nil {
		log.Error("not enrolled", "hint", "run `agent enroll --token=...` first", "err", err)
		return 1
	}

	st, err := store.New(cfg.DataDir)
	if err != nil {
		log.Error("store init failed", "err", err)
		return 1
	}

	tlsCfg, err := mtls.LoadConfig(cfg.DataDir, cfg.ControlPlaneSNI)
	if err != nil {
		log.Error("load tls failed", "err", err)
		return 1
	}

	ob, err := outbox.Open(cfg.DataDir)
	if err != nil {
		log.Error("outbox open failed", "err", err)
		return 1
	}
	defer ob.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Info("shutdown signal received")
		cancel()
	}()

	rt := agentruntime.New(cfg, log, st, id, tlsCfg, ob)
	if err := rt.Run(ctx); err != nil && ctx.Err() == nil {
		log.Error("runtime exited", "err", err)
		return 1
	}
	return 0
}

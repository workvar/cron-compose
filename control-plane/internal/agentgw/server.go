// Package agentgw is the gRPC gateway agents connect to. Long-lived AgentStream over
// mutual TLS, plus the per-server connection registry and the live-log broker.
package agentgw

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"net"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/croncompose/croncompose/control-plane/internal/pki"
	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// FailedRunHook is invoked from onRunFinished whenever a run terminates with a
// non-success status. Implemented by *notify.Notifier; injected to keep agentgw from
// depending on the notify package directly.
type FailedRunHook interface {
	FireRunFailed(serverID, jobID, runID, status string, exitCode, durationMs int32, errMsg string)
}

// Gateway owns the gRPC server, the per-server connection registry, the log broker,
// and the TLS listener.
type Gateway struct {
	addr     string
	log      *slog.Logger
	pool     *pgxpool.Pool
	bundle   *pki.Bundle
	registry  *Registry
	broker    *LogBroker
	terminals *TerminalBus
	resolver  SecretResolver
	onFailed  FailedRunHook
	grpc      *grpc.Server
	lis       net.Listener
}

// New constructs a Gateway.
func New(addr string, log *slog.Logger, pool *pgxpool.Pool, bundle *pki.Bundle, resolver SecretResolver) *Gateway {
	return &Gateway{
		addr:     addr,
		log:      log,
		pool:     pool,
		bundle:   bundle,
		registry:  NewRegistry(),
		broker:    NewLogBroker(),
		terminals: NewTerminalBus(),
		resolver:  resolver,
	}
}

// SetFailedRunHook installs a hook invoked when a run ends with a non-success status.
// Set this once, before Start, so the stream handler picks it up.
func (g *Gateway) SetFailedRunHook(h FailedRunHook) { g.onFailed = h }

// Registry exposes the per-server connection registry so the REST API can push
// RunNow / CancelRun / SyncJobs over the active stream.
func (g *Gateway) Registry() *Registry { return g.registry }

// Broker exposes the log broker so the REST SSE endpoint can subscribe.
func (g *Gateway) Broker() *LogBroker { return g.broker }

// Terminals exposes the terminal bus so the REST WebSocket endpoint can route a
// session's output back to the browser.
func (g *Gateway) Terminals() *TerminalBus { return g.terminals }

// Start binds and serves over mTLS.
func (g *Gateway) Start(_ context.Context) error {
	if g.bundle == nil {
		return errors.New("agentgw: nil PKI bundle")
	}

	clientCAs := x509.NewCertPool()
	if !clientCAs.AppendCertsFromPEM(g.bundle.CACertPEM) {
		return errors.New("agentgw: append CA cert failed")
	}

	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{g.bundle.ServerTLS},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    clientCAs,
		MinVersion:   tls.VersionTLS12,
	}

	lis, err := net.Listen("tcp", g.addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", g.addr, err)
	}
	g.lis = lis

	creds := credentials.NewTLS(tlsCfg)
	g.grpc = grpc.NewServer(grpc.Creds(creds))
	agentv1.RegisterAgentServiceServer(g.grpc, newService(g.log, g.pool, g.registry, g.broker, g.terminals, g.resolver, g.onFailed))

	go func() {
		g.log.Info("grpc listening (mTLS)", "addr", g.addr)
		if err := g.grpc.Serve(lis); err != nil {
			g.log.Error("grpc serve stopped", "err", err)
		}
	}()
	return nil
}

// Stop attempts a graceful shutdown.
func (g *Gateway) Stop() {
	if g.grpc != nil {
		g.grpc.GracefulStop()
	}
}

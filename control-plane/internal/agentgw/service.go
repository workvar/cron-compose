package agentgw

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// service implements agentv1.AgentServiceServer.
//
// Authentication is per-RPC and uses the peer's client certificate (mTLS). The cert
// fingerprint resolves to a server row. Enroll is intentionally NOT implemented here;
// it lives on the REST API (POST /api/v1/agents/enroll) because it has to happen
// before the agent has a cert. UnimplementedAgentServiceServer returns Unimplemented
// to any agent that still calls the gRPC Enroll RPC.
type service struct {
	agentv1.UnimplementedAgentServiceServer
	log       *slog.Logger
	pool      *pgxpool.Pool
	registry  *Registry
	broker    *LogBroker
	terminals *TerminalBus
	resolver  SecretResolver
	onFailed  FailedRunHook
}

func newService(log *slog.Logger, pool *pgxpool.Pool, reg *Registry, broker *LogBroker, terminals *TerminalBus, resolver SecretResolver, onFailed FailedRunHook) *service {
	return &service{log: log, pool: pool, registry: reg, broker: broker, terminals: terminals, resolver: resolver, onFailed: onFailed}
}

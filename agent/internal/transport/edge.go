package transport

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// Mode is how the agent authenticates to the control plane.
const (
	ModeMTLS = "mtls" // client certificate, checked by the control plane (default)
	ModeEdge = "edge" // public TLS to an edge that ends it, plus a per-server secret
)

// Metadata keys the control plane's edge listener reads. They must match
// control-plane/internal/agentgw/edge_auth.go.
const (
	metaServerID = "x-cc-server-id"
	metaSecret   = "x-cc-agent-secret"
)

// ResolveMode picks the connection mode: a value set in the environment wins over the
// one the control plane saved at enrollment, and anything unrecognised means mTLS so a
// typo cannot switch the agent to the secret-authenticated path.
func ResolveMode(env, enrolled string) string {
	for _, m := range []string{env, enrolled} {
		switch strings.ToLower(strings.TrimSpace(m)) {
		case ModeEdge:
			return ModeEdge
		case ModeMTLS:
			return ModeMTLS
		}
	}
	return ModeMTLS
}

// EdgeOptions are the inputs for DialEdge.
type EdgeOptions struct {
	ServerID string
	Secret   string
	// RootCAs overrides the system roots. Nil in production: the edge presents a
	// certificate from a public CA. Tests pass their own.
	RootCAs *x509.CertPool
}

// DialEdge connects over ordinary TLS, verifying the edge's public certificate for the
// address's own host, and authenticates every call with the agent's secret instead of a
// client certificate. Use it only when mutual TLS cannot pass through, as the edge can
// read this traffic.
func DialEdge(ctx context.Context, addr string, o EdgeOptions) (*Client, error) {
	if o.ServerID == "" || o.Secret == "" {
		return nil, errors.New("edge mode needs the agent secret saved at enrollment: re-enroll this agent with a new token")
	}
	addr = NormalizeAddr(addr)
	host, _, err := net.SplitHostPort(addr)
	if err != nil || host == "" {
		return nil, fmt.Errorf("edge address %q has no host", addr)
	}
	tlsCfg := &tls.Config{ServerName: host, RootCAs: o.RootCAs, MinVersion: tls.VersionTLS12}
	return dial(ctx, addr, grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)),
		grpc.WithPerRPCCredentials(secretAuth{id: o.ServerID, secret: o.Secret}))
}

// secretAuth adds the agent's id and secret to every call. It refuses to run over a
// connection without transport security, so the secret is never sent in the clear.
type secretAuth struct{ id, secret string }

func (a secretAuth) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{metaServerID: a.id, metaSecret: a.secret}, nil
}

func (secretAuth) RequireTransportSecurity() bool { return true }

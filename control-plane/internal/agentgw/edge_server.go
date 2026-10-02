package agentgw

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc"

	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// SetEdgeAddr enables the edge listener on addr (empty disables it). Set before Start.
// The edge listener speaks plain HTTP/2 gRPC and authenticates agents by a per-server
// secret instead of a client certificate, for hosts reached through an edge that ends
// TLS, such as a Cloudflare proxy or tunnel. It is the only authentication it has, so
// it must only be reachable from that edge: Start refuses a non-loopback address.
func (g *Gateway) SetEdgeAddr(addr string) { g.edgeAddr = addr }

// requireLoopback fails for any address that is not bound to a loopback interface.
func requireLoopback(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("edge listener address %q: %w", addr, err)
	}
	if host == "localhost" {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return nil
	}
	return fmt.Errorf("edge listener address %q is not loopback: it has no TLS, so bind 127.0.0.1 and let the tunnel connect to it", addr)
}

func (g *Gateway) startEdge(svc agentv1.AgentServiceServer) error {
	if g.edgeAddr == "" {
		return nil
	}
	if err := requireLoopback(g.edgeAddr); err != nil {
		return err
	}
	lis, err := net.Listen("tcp", g.edgeAddr)
	if err != nil {
		return fmt.Errorf("listen %s (edge): %w", g.edgeAddr, err)
	}
	auth := newEdgeAuth(g.edgeSecretHash, g.log)
	g.edgeGRPC = grpc.NewServer(
		grpc.StreamInterceptor(auth.streamInterceptor),
		grpc.UnaryInterceptor(auth.unaryInterceptor),
	)
	agentv1.RegisterAgentServiceServer(g.edgeGRPC, svc)
	go func() {
		g.log.Info("grpc listening (edge, secret auth, plain h2c)", "addr", g.edgeAddr)
		if err := g.edgeGRPC.Serve(lis); err != nil {
			g.log.Error("edge grpc serve stopped", "err", err)
		}
	}()
	return nil
}

func (g *Gateway) edgeSecretHash(ctx context.Context, serverID string) (string, error) {
	var hash *string
	err := g.pool.QueryRow(ctx, `select agent_secret_hash from servers where id = $1`, serverID).Scan(&hash)
	if errors.Is(err, pgx.ErrNoRows) || hash == nil {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return *hash, nil
}

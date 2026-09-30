// Package transport wraps the gRPC client to the control plane (mTLS).
package transport

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// Client is a thin wrapper around the generated AgentService client.
type Client struct {
	addr string
	conn *grpc.ClientConn
	svc  agentv1.AgentServiceClient
}

// Dial opens a mTLS connection to the control plane.
func Dial(ctx context.Context, addr string, tlsCfg *tls.Config) (*Client, error) {
	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(dialCtx, addr,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)),
		grpc.WithBlock(),
		// Without this a refused port, a TLS mismatch and a timeout all surface as
		// "context deadline exceeded", which hides a wrong address or port clash.
		grpc.WithReturnConnectionError(),
	)
	if err != nil {
		if hint := dialHint(err); hint != "" {
			return nil, fmt.Errorf("dial %s: %w (%s)", addr, err, hint)
		}
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}
	return &Client{
		addr: addr,
		conn: conn,
		svc:  agentv1.NewAgentServiceClient(conn),
	}, nil
}

// dialHint names the likely cause for errors an operator can act on.
func dialHint(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "first record does not look like a TLS handshake"),
		strings.Contains(msg, "certificate signed by unknown authority"),
		strings.Contains(msg, "certificate is valid for"),
		strings.Contains(msg, "http2: frame too large"),
		strings.Contains(msg, "error reading server preface"):
		return "something other than this CronCompose control plane answered on that port: " +
			"check the address in identity.json / AGENT_GRPC_ADDR and whether another service uses the port"
	case strings.Contains(msg, "connection refused"):
		return "nothing is listening there: is the control plane running and is GRPC_ADDR on that port?"
	case strings.Contains(msg, "no such host"):
		return "the host does not resolve"
	}
	return ""
}

// Close tears down the connection.
func (c *Client) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

// OpenStream attaches to AgentStream. Authentication is the mTLS client cert.
func (c *Client) OpenStream(ctx context.Context) (agentv1.AgentService_AgentStreamClient, error) {
	return c.svc.AgentStream(ctx)
}

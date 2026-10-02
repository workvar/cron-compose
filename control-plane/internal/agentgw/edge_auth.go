package agentgw

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/croncompose/croncompose/control-plane/internal/agentsecret"
)

// Metadata keys an agent sends on the edge listener.
const (
	MetaServerID = "x-cc-server-id"
	MetaSecret   = "x-cc-agent-secret"
)

// edgeFailDelay slows a rejected login so the listener cannot be used to hammer the
// database with guesses. The secret is 256 random bits, so this is only a courtesy.
const edgeFailDelay = 300 * time.Millisecond

type edgeCtxKey struct{}

// edgeServerID returns the server an edge-authenticated stream belongs to. Only the
// edge listener's interceptor sets it; the mTLS listener never does, so a header sent
// to that listener cannot name a server.
func edgeServerID(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(edgeCtxKey{}).(string)
	return id, ok && id != ""
}

// secretLookup returns the stored hash of a server's secret, or "" when it has none.
type secretLookup func(ctx context.Context, serverID string) (string, error)

type edgeAuth struct {
	lookup secretLookup
	log    *slog.Logger
	delay  time.Duration
}

func newEdgeAuth(lookup secretLookup, log *slog.Logger) *edgeAuth {
	return &edgeAuth{lookup: lookup, log: log, delay: edgeFailDelay}
}

// authenticate checks the id and secret in the stream's metadata. Every failure looks
// the same to the caller, so it cannot tell a wrong id from a wrong secret.
func (a *edgeAuth) authenticate(ctx context.Context) (string, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	id, secret := first(md, MetaServerID), first(md, MetaSecret)
	if id != "" && secret != "" {
		stored, err := a.lookup(ctx, id)
		if err == nil && agentsecret.Matches(secret, stored) {
			return id, nil
		}
		if err != nil && !errors.Is(err, context.Canceled) {
			a.log.Warn("edge auth lookup failed", "err", err)
		}
	}
	time.Sleep(a.delay)
	return "", status.Error(codes.Unauthenticated, "unauthenticated")
}

// streamInterceptor authenticates every stream and hands the handler a context that
// carries the resolved server id.
func (a *edgeAuth) streamInterceptor(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, h grpc.StreamHandler) error {
	id, err := a.authenticate(ss.Context())
	if err != nil {
		return err
	}
	return h(srv, &edgeStream{ServerStream: ss, ctx: context.WithValue(ss.Context(), edgeCtxKey{}, id)})
}

// unaryInterceptor rejects unary calls: the edge listener only serves the agent stream.
func (a *edgeAuth) unaryInterceptor(context.Context, any, *grpc.UnaryServerInfo, grpc.UnaryHandler) (any, error) {
	return nil, status.Error(codes.Unimplemented, "not available on the edge listener")
}

type edgeStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *edgeStream) Context() context.Context { return s.ctx }

func first(md metadata.MD, key string) string {
	if v := md.Get(key); len(v) > 0 {
		return v[0]
	}
	return ""
}

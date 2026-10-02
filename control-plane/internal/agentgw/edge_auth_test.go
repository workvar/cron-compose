package agentgw

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/croncompose/croncompose/control-plane/internal/agentsecret"
)

func testEdgeAuth(t *testing.T, secrets map[string]string) *edgeAuth {
	t.Helper()
	a := newEdgeAuth(func(_ context.Context, id string) (string, error) {
		h, ok := secrets[id]
		if !ok {
			return "", nil
		}
		if h == "error" {
			return "", errors.New("db down")
		}
		return h, nil
	}, slog.Default())
	a.delay = 0
	return a
}

func ctxWith(id, secret string) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs(MetaServerID, id, MetaSecret, secret))
}

func TestEdgeAuthAcceptsTheRightSecret(t *testing.T) {
	plain, hash, _ := agentsecret.Generate()
	a := testEdgeAuth(t, map[string]string{"srv-1": hash})
	id, err := a.authenticate(ctxWith("srv-1", plain))
	if err != nil || id != "srv-1" {
		t.Fatalf("got %q, %v", id, err)
	}
}

// Production change that would fail this test: accepting a server with no stored
// secret, or telling the caller why a login failed.
func TestEdgeAuthRejectsEverythingElseTheSameWay(t *testing.T) {
	plain, hash, _ := agentsecret.Generate()
	a := testEdgeAuth(t, map[string]string{"srv-1": hash, "no-secret": "", "broken": "error"})
	cases := map[string]context.Context{
		"wrong secret":     ctxWith("srv-1", plain+"x"),
		"unknown server":   ctxWith("srv-9", plain),
		"server no secret": ctxWith("no-secret", plain),
		"empty secret":     ctxWith("srv-1", ""),
		"lookup error":     ctxWith("broken", plain),
		"no metadata":      context.Background(),
	}
	for name, ctx := range cases {
		_, err := a.authenticate(ctx)
		if status.Code(err) != codes.Unauthenticated || status.Convert(err).Message() != "unauthenticated" {
			t.Errorf("%s: got %v", name, err)
		}
	}
}

// A header sent to the mTLS listener must never name a server. Only the edge
// interceptor puts an id in the context. Production change that would fail this
// test: reading the id straight from metadata inside service.authenticate.
func TestMetadataAloneDoesNotAuthenticateOnTheMTLSPath(t *testing.T) {
	s := &service{}
	_, err := s.authenticate(ctxWith("srv-1", "anything"))
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("got %v", err)
	}
}

func TestRequireLoopback(t *testing.T) {
	for addr, ok := range map[string]bool{
		"127.0.0.1:9078": true, "localhost:9078": true, "[::1]:9078": true,
		"0.0.0.0:9078": false, ":9078": false, "192.168.1.5:9078": false, "grpc.example.com:9078": false, "nonsense": false,
	} {
		if err := requireLoopback(addr); (err == nil) != ok {
			t.Errorf("requireLoopback(%q) = %v, want ok=%v", addr, err, ok)
		}
	}
}

func TestEdgeStreamCarriesTheResolvedServerID(t *testing.T) {
	plain, hash, _ := agentsecret.Generate()
	a := testEdgeAuth(t, map[string]string{"srv-1": hash})
	got := make(chan string, 1)
	srv := startEdgeTestServer(t, a, got)
	conn := dialEdgeTest(t, srv)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stream, err := conn.stream(metadata.AppendToOutgoingContext(ctx, MetaServerID, "srv-1", MetaSecret, plain))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stream.Recv(); err != nil && err != io.EOF {
		t.Fatal(err)
	}
	if id := <-got; id != "srv-1" {
		t.Fatalf("handler saw %q", id)
	}

	bad, err := conn.stream(metadata.AppendToOutgoingContext(ctx, MetaServerID, "srv-1", MetaSecret, "nope"))
	if err == nil {
		_, err = bad.Recv()
	}
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("wrong secret over the edge: %v", err)
	}
}

// Production change that would fail this test: the real service ignoring the id the
// edge interceptor resolved, which would reject every edge stream as having no peer.
func TestServiceAuthenticateTrustsTheEdgeInterceptorsID(t *testing.T) {
	s := &service{}
	ctx := context.WithValue(context.Background(), edgeCtxKey{}, "srv-1")
	if id, err := s.authenticate(ctx); err != nil || id != "srv-1" {
		t.Fatalf("got %q, %v", id, err)
	}
}

package agentgw

import (
	"log/slog"
	"testing"

	"github.com/croncompose/croncompose/control-plane/internal/agentsecret"
)

// End to end against a real database: the stored hash is what the edge listener
// checks. Production change that would fail this test: reading the wrong column, or
// treating a server with no stored secret as having one.
func TestEdgeSecretHashFromDatabase(t *testing.T) {
	ctx, pool, serverID := seedRootServer(t, false)
	g := &Gateway{pool: pool, log: slog.Default()}
	a := newEdgeAuth(g.edgeSecretHash, g.log)
	a.delay = 0

	plain, hash, _ := agentsecret.Generate()
	if _, err := a.authenticate(ctxWith(serverID, plain)); err == nil {
		t.Fatal("a server with no stored secret must reject every login")
	}
	if _, err := pool.Exec(ctx, `update servers set agent_secret_hash = $2 where id = $1`, serverID, hash); err != nil {
		t.Fatal(err)
	}
	if id, err := a.authenticate(ctxWith(serverID, plain)); err != nil || id != serverID {
		t.Fatalf("got %q, %v", id, err)
	}
	if _, err := a.authenticate(ctxWith(serverID, plain+"x")); err == nil {
		t.Fatal("wrong secret must be rejected")
	}
}

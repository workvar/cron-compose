package transport

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"

	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

func TestResolveMode(t *testing.T) {
	cases := []struct{ env, enrolled, want string }{
		{"", "", ModeMTLS},
		{"", "edge", ModeEdge},
		{"mtls", "edge", ModeMTLS}, // the environment wins
		{"edge", "mtls", ModeEdge},
		{"EDGE", "", ModeEdge},
		{"typo", "", ModeMTLS},
		{"typo", "edge", ModeEdge}, // an unrecognised env value falls through
	}
	for _, c := range cases {
		if got := ResolveMode(c.env, c.enrolled); got != c.want {
			t.Errorf("ResolveMode(%q,%q)=%q want %q", c.env, c.enrolled, got, c.want)
		}
	}
}

func TestDialEdgeNeedsASecret(t *testing.T) {
	_, err := DialEdge(context.Background(), "grpc.example.com", EdgeOptions{ServerID: "s"})
	if err == nil || !strings.Contains(err.Error(), "re-enroll") {
		t.Fatalf("got %v", err)
	}
}

// Production change that would fail this test: allowing the secret over a connection
// without transport security.
func TestSecretIsNeverSentWithoutTLS(t *testing.T) {
	if !(secretAuth{}).RequireTransportSecurity() {
		t.Fatal("secretAuth must require transport security")
	}
}

type metaCheckSvc struct {
	agentv1.UnimplementedAgentServiceServer
	seen chan metadata.MD
}

func (s *metaCheckSvc) AgentStream(stream agentv1.AgentService_AgentStreamServer) error {
	md, _ := metadata.FromIncomingContext(stream.Context())
	s.seen <- md
	return nil
}

// The agent must verify the edge's certificate for the address's host, and send its id
// and secret as call metadata.
func TestDialEdgeSendsSecretOverVerifiedTLS(t *testing.T) {
	cert, pool := localhostCert(t)
	svc := &metaCheckSvc{seen: make(chan metadata.MD, 1)}
	gs := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{Certificates: []tls.Certificate{cert}})))
	agentv1.RegisterAgentServiceServer(gs, svc)
	plain, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = gs.Serve(plain) }()
	t.Cleanup(gs.Stop)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := DialEdge(ctx, "localhost:"+portOf(plain.Addr()), EdgeOptions{ServerID: "srv-1", Secret: "s3cret", RootCAs: pool})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	stream, err := c.OpenStream(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = stream.Recv()
	md := <-svc.seen
	if md.Get(metaServerID)[0] != "srv-1" || md.Get(metaSecret)[0] != "s3cret" {
		t.Fatalf("metadata not sent: %v", md)
	}

	// A certificate for a different host must be rejected.
	short, stop := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer stop()
	if _, err := DialEdge(short, "127.0.0.2:"+portOf(plain.Addr()), EdgeOptions{ServerID: "srv-1", Secret: "s3cret", RootCAs: pool}); err == nil {
		t.Fatal("a certificate for the wrong host must not be accepted")
	}
}

func portOf(a net.Addr) string { _, p, _ := net.SplitHostPort(a.String()); return p }

func localhostCert(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "localhost"},
		DNSNames:  []string{"localhost"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true, IsCA: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	leaf, _ := x509.ParseCertificate(der)
	pool := x509.NewCertPool()
	pool.AddCert(leaf)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, pool
}

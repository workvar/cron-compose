package agentgw

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log/slog"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/croncompose/croncompose/control-plane/internal/pki"
)

// A host we cannot shell into keeps presenting the cert this control plane signed.
// When the stored fingerprint no longer matches, authenticate must bind that cert
// to the server id in the CommonName and let the next stream in.
func TestAuthenticateRebindsCertByCommonName(t *testing.T) {
	ctx, pool, serverID := seedRootServer(t, false)
	if _, err := pool.Exec(ctx, `update servers set cert_fingerprint = $2 where id = $1`, serverID, "stale-fingerprint"); err != nil {
		t.Fatal(err)
	}
	cert := signClientCert(t, serverID)
	s := &service{log: slog.Default(), pool: pool}

	got, err := s.authenticate(peerCtx(cert))
	if err != nil || got != serverID {
		t.Fatalf("rebind: got %q, %v", got, err)
	}
	var stored string
	if err := pool.QueryRow(ctx, `select cert_fingerprint from servers where id = $1`, serverID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != pki.FingerprintDER(cert.Raw) {
		t.Fatalf("stored fingerprint %q, want %q", stored, pki.FingerprintDER(cert.Raw))
	}

	got, err = s.authenticate(peerCtx(cert))
	if err != nil || got != serverID {
		t.Fatalf("second connect: got %q, %v", got, err)
	}
}

func TestAuthenticateRejectsCertForUnknownServer(t *testing.T) {
	_, pool, _ := seedRootServer(t, false)
	cert := signClientCert(t, "does-not-exist")
	s := &service{log: slog.Default(), pool: pool}

	_, err := s.authenticate(peerCtx(cert))
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("got %v, want Unauthenticated", err)
	}
}

func signClientCert(t *testing.T, serverID string) *x509.Certificate {
	t.Helper()
	bundle, err := pki.LoadOrCreate(t.TempDir(), []string{"localhost"})
	if err != nil {
		t.Fatal(err)
	}
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader,
		&x509.CertificateRequest{Subject: pkix.Name{CommonName: "ignored"}}, priv)
	if err != nil {
		t.Fatal(err)
	}
	certPEM, _, err := bundle.SignAgentCSR(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER}), serverID)
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(certPEM)
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	return cert
}

func peerCtx(cert *x509.Certificate) context.Context {
	return peer.NewContext(context.Background(), &peer.Peer{
		AuthInfo: credentials.TLSInfo{State: tls.ConnectionState{PeerCertificates: []*x509.Certificate{cert}}},
	})
}

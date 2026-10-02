package agentgw

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"io"
	"math/big"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// edgeTestSvc stands in for the real service: it records which server the edge
// interceptor resolved, then ends the stream.
type edgeTestSvc struct {
	agentv1.UnimplementedAgentServiceServer
	got chan string
}

func (s *edgeTestSvc) AgentStream(stream agentv1.AgentService_AgentStreamServer) error {
	id, _ := edgeServerID(stream.Context())
	s.got <- id
	return nil
}

// startEdgeTestServer runs the edge gRPC server on loopback, behind a TLS-terminating
// TCP proxy. That is what a Cloudflare tunnel does: the agent sees a normal TLS server
// certificate, the origin sees plain HTTP/2.
func startEdgeTestServer(t *testing.T, a *edgeAuth, got chan string) *edgeTestServer {
	t.Helper()
	origin, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	gs := grpc.NewServer(grpc.StreamInterceptor(a.streamInterceptor), grpc.UnaryInterceptor(a.unaryInterceptor))
	agentv1.RegisterAgentServiceServer(gs, &edgeTestSvc{got: got})
	go func() { _ = gs.Serve(origin) }()
	t.Cleanup(gs.Stop)

	cert, pool := selfSignedForLocalhost(t)
	front, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}, NextProtos: []string{"h2"}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = front.Close() })
	go func() {
		for {
			c, err := front.Accept()
			if err != nil {
				return
			}
			go func() {
				defer c.Close()
				up, err := net.Dial("tcp", origin.Addr().String())
				if err != nil {
					return
				}
				defer up.Close()
				go func() { _, _ = io.Copy(up, c) }()
				_, _ = io.Copy(c, up)
			}()
		}
	}()
	return &edgeTestServer{addr: front.Addr().String(), pool: pool}
}

type edgeTestServer struct {
	addr string
	pool *x509.CertPool
}

type edgeTestConn struct{ cc *grpc.ClientConn }

func dialEdgeTest(t *testing.T, s *edgeTestServer) *edgeTestConn {
	t.Helper()
	cc, err := grpc.NewClient(s.addr, grpc.WithTransportCredentials(
		credentials.NewTLS(&tls.Config{RootCAs: s.pool, ServerName: "localhost", MinVersion: tls.VersionTLS12})))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cc.Close() })
	return &edgeTestConn{cc: cc}
}

func (c *edgeTestConn) stream(ctx context.Context) (agentv1.AgentService_AgentStreamClient, error) {
	return agentv1.NewAgentServiceClient(c.cc).AgentStream(ctx)
}

func selfSignedForLocalhost(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "localhost"},
		DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
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

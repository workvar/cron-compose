package transport

import (
	"context"
	"crypto/tls"
	"strings"
	"testing"
	"time"
)

func TestNormalizeAddr(t *testing.T) {
	cases := map[string]string{
		"grpc.example.com":      "grpc.example.com:443",
		"grpc.example.com:9077": "grpc.example.com:9077",
		"127.0.0.1:9077":        "127.0.0.1:9077",
		"127.0.0.1":             "127.0.0.1:443",
		"[::1]":                 "[::1]:443",
		"::1":                   "[::1]:443",
		"[::1]:9077":            "[::1]:9077",
		"  grpc.example.com  ":  "grpc.example.com:443",
		":9077":                 ":9077",
		"":                      "",
	}
	for in, want := range cases {
		if got := NormalizeAddr(in); got != want {
			t.Errorf("NormalizeAddr(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestHasHost(t *testing.T) {
	for addr, want := range map[string]bool{
		"grpc.example.com": true, "grpc.example.com:9077": true, ":9077": false, "": false,
	} {
		if got := HasHost(addr); got != want {
			t.Errorf("HasHost(%q) = %v, want %v", addr, got, want)
		}
	}
}

// Production change that would fail this test: Dial passing a bare hostname straight
// to gRPC, whose default dialer fails with "missing port in address".
func TestDialAddsDefaultPortToBareHost(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_, err := Dial(ctx, "localhost", &tls.Config{})
	if err == nil {
		t.Fatal("expected a dial error, nothing listens on :443 here")
	}
	if strings.Contains(err.Error(), "missing port") || !strings.Contains(err.Error(), "localhost:443") {
		t.Fatalf("bare host was not given the default port: %v", err)
	}
}

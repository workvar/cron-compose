package config

import (
	"os"
	"testing"
)

// Setting PUBLIC_BASE_URL should derive the REST URL, the gRPC dial address (host of
// the base URL + the gRPC listen port), and add the host to the TLS SANs.
func TestPublicBaseURLDerivesEverything(t *testing.T) {
	t.Setenv("PUBLIC_BASE_URL", "https://cron.example.com")
	t.Setenv("GRPC_ADDR", ":9443")
	t.Setenv("PUBLIC_HTTP_URL", "")
	t.Setenv("PUBLIC_GRPC_ADDR", "")
	t.Setenv("TLS_HOSTS", "localhost,127.0.0.1")
	t.Setenv("OIDC_ISSUER_URL", "https://idp.example.com")
	t.Setenv("OIDC_REDIRECT_URL", "")

	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got, want := c.PublicHTTPURL, "https://cron.example.com/api/v1"; got != want {
		t.Errorf("PublicHTTPURL = %q, want %q", got, want)
	}
	if got, want := c.PublicGRPCAddr, "cron.example.com:9443"; got != want {
		t.Errorf("PublicGRPCAddr = %q, want %q", got, want)
	}
	if got, want := c.OIDCRedirectURL, "https://cron.example.com/api/v1/auth/oidc/callback"; got != want {
		t.Errorf("OIDCRedirectURL = %q, want %q", got, want)
	}
	if !contains(c.TLSHosts, "cron.example.com") {
		t.Errorf("TLSHosts %v missing derived host", c.TLSHosts)
	}
}

// A base URL that includes a port (the typical local Pi case) keeps the host:port for
// the REST URL and uses the gRPC port for the agent address.
func TestPublicBaseURLWithPort(t *testing.T) {
	t.Setenv("PUBLIC_BASE_URL", "http://raspberrypi.local:8080")
	t.Setenv("GRPC_ADDR", ":9090")
	t.Setenv("PUBLIC_HTTP_URL", "")
	t.Setenv("PUBLIC_GRPC_ADDR", "")

	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got, want := c.PublicHTTPURL, "http://raspberrypi.local:8080/api/v1"; got != want {
		t.Errorf("PublicHTTPURL = %q, want %q", got, want)
	}
	if got, want := c.PublicGRPCAddr, "raspberrypi.local:9090"; got != want {
		t.Errorf("PublicGRPCAddr = %q, want %q", got, want)
	}
}

// An explicitly-set value still wins over the derived one (advanced setups).
func TestExplicitOverrideWins(t *testing.T) {
	t.Setenv("PUBLIC_BASE_URL", "https://cron.example.com")
	t.Setenv("PUBLIC_GRPC_ADDR", "grpc.example.com:5000")

	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got, want := c.PublicGRPCAddr, "grpc.example.com:5000"; got != want {
		t.Errorf("PublicGRPCAddr = %q, want explicit %q", got, want)
	}
}

// Without PUBLIC_BASE_URL the original defaults are untouched.
func TestNoBaseURLKeepsDefaults(t *testing.T) {
	t.Setenv("PUBLIC_BASE_URL", "")
	t.Setenv("PUBLIC_HTTP_URL", "")
	t.Setenv("PUBLIC_GRPC_ADDR", "")

	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got, want := c.PublicHTTPURL, "http://localhost:8080/api/v1"; got != want {
		t.Errorf("PublicHTTPURL = %q, want default %q", got, want)
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func TestParseDotEnvQuotedSpecials(t *testing.T) {
	got, err := parseDotEnv("SEED_ADMIN_PASSWORD=\"p#ass$word\"\nSEED_ADMIN_EMAIL=admin@example.com\n")
	if err != nil {
		t.Fatal(err)
	}
	if got["SEED_ADMIN_PASSWORD"] != "p#ass$word" {
		t.Errorf("password=%q, want quoted value with # and $ intact", got["SEED_ADMIN_PASSWORD"])
	}
	if got["SEED_ADMIN_EMAIL"] != "admin@example.com" {
		t.Errorf("email=%q", got["SEED_ADMIN_EMAIL"])
	}
}

func TestParseDotEnvSkipsComments(t *testing.T) {
	got, err := parseDotEnv("# comment\nFOO=bar\n\n")
	if err != nil {
		t.Fatal(err)
	}
	if got["FOO"] != "bar" {
		t.Errorf("FOO=%q", got["FOO"])
	}
}

func TestApplyDotEnvDoesNotOverride(t *testing.T) {
	t.Setenv("KEEP_ME", "from-process")
	dir := t.TempDir()
	path := dir + "/.env"
	if err := os.WriteFile(path, []byte("KEEP_ME=from-file\nNEW_ME=from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := applyDotEnvFile(path); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("KEEP_ME") != "from-process" {
		t.Errorf("KEEP_ME overridden to %q", os.Getenv("KEEP_ME"))
	}
	if os.Getenv("NEW_ME") != "from-file" {
		t.Errorf("NEW_ME=%q, want from-file", os.Getenv("NEW_ME"))
	}
	t.Cleanup(func() { os.Unsetenv("NEW_ME") })
}

package deploy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPreferEcosystemName(t *testing.T) {
	dir := t.TempDir()
	proxy := filepath.Join(dir, "proxy")
	if err := os.MkdirAll(proxy, 0o755); err != nil {
		t.Fatal(err)
	}
	eco := filepath.Join(dir, "ecosystem.config.js")
	body := `module.exports = { apps: [
  { name: 'peepal-proxy', script: './proxy', cwd: 'proxy' },
  { name: 'peepal-api', script: './api', cwd: 'api' },
]}`
	if err := os.WriteFile(eco, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got := preferEcosystemName("peepal-rp-proxy", proxy, eco)
	if got != "peepal-proxy" {
		t.Fatalf("got %q", got)
	}
	found := findEcosystem(proxy)
	if found != eco {
		t.Fatalf("findEcosystem = %q want %q", found, eco)
	}
}

func TestPreferEcosystemNameLocalFile(t *testing.T) {
	dir := t.TempDir()
	eco := filepath.Join(dir, "ecosystem.config.js")
	if err := os.WriteFile(eco, []byte(`module.exports = { apps: [{ name: 'peepal-proxy', script: './go' }] }`), 0o644); err != nil {
		t.Fatal(err)
	}
	got := preferEcosystemName("peepal-rp-proxy", dir, eco)
	if got != "peepal-proxy" {
		t.Fatalf("got %q", got)
	}
}

func TestPickPM2Aliases(t *testing.T) {
	procs := parsePM2Jlist(`[{"name":"peepal-proxy","pm2_env":{"status":"online","pm_cwd":"/opt/apps/x/current/proxy"}}]`)
	got, ok := pickPM2(procs, "peepal-rp-proxy", "/opt/apps/x/current/proxy", "peepal-proxy")
	if !ok || got.Name != "peepal-proxy" {
		t.Fatalf("got %+v ok=%v", got, ok)
	}
}

package updates

import (
	"log/slog"
	"testing"
)

func TestPolicyUsesConfiguredRepo(t *testing.T) {
	c := NewCatalog(slog.Default(), Config{Repo: "workvar/cron-compose"})
	c.latest = &Release{
		Version:   "v0.0.2",
		Checksums: map[string]string{"linux/amd64": "deadbeef"},
	}
	p := c.Policy()
	if p.Repo != "workvar/cron-compose" {
		t.Errorf("Repo = %q", p.Repo)
	}
	if p.Version != "v0.0.2" {
		t.Errorf("Version = %q", p.Version)
	}
	if !p.Active() {
		t.Fatal("policy should be active")
	}
}

func TestCatalogRepo(t *testing.T) {
	c := NewCatalog(slog.Default(), Config{Repo: "workvar/cron-compose"})
	if got := c.Repo(); got != "workvar/cron-compose" {
		t.Errorf("Repo() = %q", got)
	}
}

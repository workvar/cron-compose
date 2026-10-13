package deploy

import "testing"

func TestPickPM2EcosystemName(t *testing.T) {
	raw := `[PM2] Spawning PM2 daemon
[{"name":"proxy","pm2_env":{"status":"online","pm_cwd":"/opt/apps/node/peepal-rp/current/proxy"}},{"name":"other","pm2_env":{"status":"online","pm_cwd":"/somewhere/else"}}]`
	procs := parsePM2Jlist(raw)
	if len(procs) != 2 {
		t.Fatalf("parsed %d", len(procs))
	}
	got, ok := pickPM2(procs, "peepal-rp-proxy", "/opt/apps/node/peepal-rp/current/proxy")
	if !ok || got.Name != "proxy" {
		t.Fatalf("got %+v ok=%v", got, ok)
	}
}

func TestPickPM2QualifiedName(t *testing.T) {
	procs := parsePM2Jlist(`[{"name":"peepal-rp-proxy","pm2_env":{"status":"launching","pm_cwd":"/opt/apps/proxy"}}]`)
	got, ok := pickPM2(procs, "peepal-rp-proxy", "/opt/apps/proxy")
	if !ok || got.Pm2Env.Status != "launching" {
		t.Fatalf("got %+v ok=%v", got, ok)
	}
}

func TestPortFromEnv(t *testing.T) {
	if got := portFrom(0, map[string]string{"PORT": "4010"}); got != 4010 {
		t.Fatalf("env port: %d", got)
	}
	if got := portFrom(8080, map[string]string{"PORT": "4010"}); got != 8080 {
		t.Fatalf("explicit port wins: %d", got)
	}
}

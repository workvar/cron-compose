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

func TestPickPM2PeepalProxyAlias(t *testing.T) {
	// Real host: ecosystem names the process peepal-proxy; we wait for peepal-rp-proxy.
	// cwd is the release folder while work is …/current/proxy.
	procs := parsePM2Jlist(`[{"name":"peepal-proxy","pm2_env":{"status":"online","pm_cwd":"/opt/apps/node/peepal-rp/releases/20261013T104257Z-6d65186/proxy"}}]`)
	got, ok := pickPM2(procs, "peepal-rp-proxy", "/opt/apps/node/peepal-rp/current/proxy")
	if !ok || got.Name != "peepal-proxy" {
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

func TestPm2NameMatch(t *testing.T) {
	cases := []struct {
		got, want, base string
		ok              bool
	}{
		{"peepal-proxy", "peepal-rp-proxy", "proxy", true},
		{"peepal-rp-proxy", "peepal-rp-proxy", "proxy", true},
		{"proxy", "peepal-rp-proxy", "proxy", true},
		{"backend", "peepal-rp-proxy", "proxy", false},
		{"peepal-backend", "peepal-rp-proxy", "proxy", false},
	}
	for _, c := range cases {
		if got := pm2NameMatch(c.got, c.want, c.base); got != c.ok {
			t.Fatalf("%s vs %s base=%s: got %v want %v", c.got, c.want, c.base, got, c.ok)
		}
	}
}

func TestPm2IsReady(t *testing.T) {
	online := pm2Listed{Name: "peepal-proxy", Pid: 12}
	online.Pm2Env.Status = "online"
	if !pm2IsReady(online) {
		t.Fatal("online should be ready")
	}
	pidOnly := pm2Listed{Name: "peepal-proxy", Pid: 99}
	if !pm2IsReady(pidOnly) {
		t.Fatal("pid without status should be ready")
	}
	stopped := pm2Listed{Name: "peepal-proxy", Pid: 1}
	stopped.Pm2Env.Status = "errored"
	if pm2IsReady(stopped) {
		t.Fatal("errored should not be ready")
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

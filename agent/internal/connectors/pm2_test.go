package connectors

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestPm2DumpPathUsesPm2Home(t *testing.T) {
	t.Setenv("PM2_HOME", "/tmp/custom-pm2")
	got := pm2DumpPath()
	want := filepath.Join("/tmp/custom-pm2", "dump.pm2")
	if got != want {
		t.Fatalf("dump path: got %q want %q", got, want)
	}
}

func TestPm2DumpPathFallsBackToHome(t *testing.T) {
	t.Setenv("PM2_HOME", "")
	t.Setenv("HOME", "/Users/deploy")
	got := pm2DumpPath()
	want := filepath.Join("/Users/deploy", ".pm2", "dump.pm2")
	if got != want {
		t.Fatalf("dump path: got %q want %q", got, want)
	}
}

func TestParsePm2ListSkipsBannerAndFillsAttributes(t *testing.T) {
	raw := `[PM2] Spawning PM2 daemon
[{"name":"api","pm_id":3,"pm2_env":{"status":"online","restart_time":2,"exec_mode":"fork_mode","pm_exec_path":"/app/server.js","pm_cwd":"/app"}}]`
	procs := parsePm2List(raw)
	if len(procs) != 1 {
		t.Fatalf("len: got %d want 1", len(procs))
	}
	p := procs[0]
	if p.Name != "api" || p.PmID != 3 {
		t.Fatalf("identity: %+v", p)
	}
	if p.Pm2Env.Status != "online" {
		t.Fatalf("status: %q", p.Pm2Env.Status)
	}
	res := pm2Object(p)
	if res.Ref != "3" || res.Name != "api" || res.State != "online" {
		t.Fatalf("resource: %+v", res)
	}
	if res.Attributes["exec"] != "/app/server.js" {
		t.Fatalf("exec: %q", res.Attributes["exec"])
	}
	if res.Attributes["mode"] != "fork_mode" {
		t.Fatalf("mode: %q", res.Attributes["mode"])
	}
	if res.Attributes["restarts"] != "2" {
		t.Fatalf("restarts: %q", res.Attributes["restarts"])
	}
	if res.Attributes["cwd"] != "/app" {
		t.Fatalf("cwd: %q", res.Attributes["cwd"])
	}
}

func TestParsePm2ListEmptyOrJunk(t *testing.T) {
	if parsePm2List("") != nil {
		t.Fatal("empty should be nil")
	}
	if parsePm2List("not json") != nil {
		t.Fatal("junk should be nil")
	}
}

func TestPm2OwnsOnlyTheDumpFile(t *testing.T) {
	dump := "/home/deploy/.pm2/dump.pm2"
	if !pm2Owns(dump, dump) {
		t.Fatal("dump itself should be owned")
	}
	if pm2Owns("/etc/passwd", dump) {
		t.Fatal("must not own /etc/passwd")
	}
	if pm2Owns(filepath.Join(dump, "..", "dump.pm2.bak"), dump) {
		t.Fatal("must not own a sibling via ..")
	}
	if pm2Owns("/home/deploy/.pm2/dump.pm2.exe", dump) {
		t.Fatal("prefix match is not enough")
	}
	if pm2Owns(dump, "") {
		t.Fatal("empty dump path owns nothing")
	}
}

func TestPm2ReadConfigRejectsPathOutsideDump(t *testing.T) {
	p := &pm2Provider{}
	inst := Instance{ConfigPaths: []string{"/tmp/dump.pm2"}}
	r := p.ReadConfig(context.Background(), inst, "/etc/passwd")
	if r.Status != StatusUnauthorized {
		t.Fatalf("status: got %q want %s (%s)", r.Status, StatusUnauthorized, r.Message)
	}
}

func TestPm2ValidateRejectsInvalidJSON(t *testing.T) {
	p := &pm2Provider{}
	dump := "/tmp/dump.pm2"
	inst := Instance{ConfigPaths: []string{dump}}
	r := p.Validate(context.Background(), inst, dump, []byte("not-json"))
	if r.Status != StatusInvalid {
		t.Fatalf("status: got %q want %s (%s)", r.Status, StatusInvalid, r.Message)
	}
}

func TestPm2ValidateAcceptsJSON(t *testing.T) {
	p := &pm2Provider{}
	dump := "/tmp/dump.pm2"
	inst := Instance{ConfigPaths: []string{dump}}
	r := p.Validate(context.Background(), inst, dump, []byte(`[{"name":"api"}]`))
	if r.Failed() {
		t.Fatalf("valid JSON rejected: %s %s", r.Status, r.Message)
	}
}

func TestPm2ReadConfigReturnsBytes(t *testing.T) {
	dir := t.TempDir()
	dump := filepath.Join(dir, "dump.pm2")
	if err := os.WriteFile(dump, []byte(`[]`), 0o644); err != nil {
		t.Fatal(err)
	}
	p := &pm2Provider{}
	inst := Instance{ConfigPaths: []string{dump}}
	r := p.ReadConfig(context.Background(), inst, dump)
	if r.Failed() {
		t.Fatalf("read: %s %s", r.Status, r.Message)
	}
	if string(r.Content) != "[]" {
		t.Fatalf("content: %q", r.Content)
	}
	if r.Checksum == "" {
		t.Fatal("expected checksum")
	}
}

func TestUsesSystemdHealth(t *testing.T) {
	if usesSystemdHealth("nginx") != true {
		t.Fatal("nginx is a systemd unit we can health-check")
	}
	for _, kind := range []string{"pm2", "docker", "systemd"} {
		if usesSystemdHealth(kind) {
			t.Fatalf("%s should skip the systemd health gate", kind)
		}
	}
}

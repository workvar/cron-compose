package deploy

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/croncompose/croncompose/agent/internal/osuser"
)

func TestStripEnvKeys(t *testing.T) {
	in := []string{
		"PATH=/usr/bin",
		"HOME=/root",
		"GOCACHE=/root/.cache/go-build",
		"GOMODCACHE=/root/go/pkg/mod",
		"KEEP=1",
	}
	out := stripEnvKeys(in, "HOME", "GOCACHE", "GOMODCACHE")
	got := strings.Join(out, "\n")
	if strings.Contains(got, "HOME=") || strings.Contains(got, "GOCACHE=") || strings.Contains(got, "GOMODCACHE=") {
		t.Fatalf("stripped keys still present: %v", out)
	}
	if !strings.Contains(got, "PATH=/usr/bin") || !strings.Contains(got, "KEEP=1") {
		t.Fatalf("kept keys missing: %v", out)
	}
}

func TestCredEnvRewritesRootGoCacheForDeployUser(t *testing.T) {
	// Simulate a root agent unit: GOCACHE under /root must not reach run_as=pi.
	t.Setenv("HOME", "/root")
	t.Setenv("GOCACHE", "/root/.cache/go-build")
	t.Setenv("GOMODCACHE", "/root/go/pkg/mod")
	t.Setenv("GOPATH", "/root/go")

	cred := &osuser.Credential{
		Username: "pi",
		UID:      1000,
		GID:      1000,
		Home:     "/home/pi",
		Shell:    "/bin/bash",
	}
	env := credEnv(cred, "/home/pi/tmp", nil)
	got := map[string]string{}
	for _, e := range env {
		k, v, ok := strings.Cut(e, "=")
		if !ok {
			continue
		}
		got[k] = v // last wins, same as os/exec
	}
	if got["HOME"] != "/home/pi" {
		t.Fatalf("HOME=%q", got["HOME"])
	}
	wantCache := filepath.Join("/home/pi", ".cache", "go-build")
	if got["GOCACHE"] != wantCache {
		t.Fatalf("GOCACHE=%q, want %q (root agent cache must not leak)", got["GOCACHE"], wantCache)
	}
	if got["TMPDIR"] != "/home/pi/tmp" {
		t.Fatalf("TMPDIR=%q", got["TMPDIR"])
	}
}

func TestRunTimeoutDefault(t *testing.T) {
	if got := runTimeout(0); got != defaultRunTimeout {
		t.Fatalf("runTimeout(0)=%v, want %v", got, defaultRunTimeout)
	}
	if defaultRunTimeout < 60*time.Minute {
		t.Fatalf("defaultRunTimeout=%v; Pi multi-app builds need at least an hour", defaultRunTimeout)
	}
}

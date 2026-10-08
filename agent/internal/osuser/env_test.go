package osuser

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCredentialEnvPointsGoCacheAtUserHome(t *testing.T) {
	c := &Credential{Username: "pi", Home: "/home/pi", Shell: "/bin/bash"}
	got := map[string]string{}
	for _, e := range c.Env() {
		k, v, ok := strings.Cut(e, "=")
		if !ok {
			t.Fatalf("bad pair %q", e)
		}
		got[k] = v
	}
	if got["HOME"] != "/home/pi" || got["USER"] != "pi" {
		t.Fatalf("identity: %+v", got)
	}
	if got["GOCACHE"] != filepath.Join("/home/pi", ".cache", "go-build") {
		t.Fatalf("GOCACHE=%q", got["GOCACHE"])
	}
	if got["GOPATH"] != filepath.Join("/home/pi", "go") {
		t.Fatalf("GOPATH=%q", got["GOPATH"])
	}
	if got["GOMODCACHE"] != filepath.Join("/home/pi", "go", "pkg", "mod") {
		t.Fatalf("GOMODCACHE=%q", got["GOMODCACHE"])
	}
}

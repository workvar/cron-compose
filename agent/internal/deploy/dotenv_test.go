package deploy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseDotEnvBasic(t *testing.T) {
	got := parseDotEnv(`
# comment
FRONTEND_PORT=3002
export BACKEND_PORT=3003
PROXY_PORT="80"
EMPTY=
`)
	if got["FRONTEND_PORT"] != "3002" || got["BACKEND_PORT"] != "3003" || got["PROXY_PORT"] != "80" {
		t.Fatalf("%+v", got)
	}
	if _, ok := got["EMPTY"]; !ok || got["EMPTY"] != "" {
		t.Fatalf("EMPTY: %+v", got)
	}
}

func TestMergeDotEnvAroundPrefersExisting(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "proxy")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("PROXY_PORT=80\nFRONTEND_PORT=3002\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	env := mergeDotEnvAround(sub, map[string]string{"PROXY_PORT": "3004"})
	if env["PROXY_PORT"] != "3004" {
		t.Fatalf("deploy overlay lost: %+v", env)
	}
	if env["FRONTEND_PORT"] != "3002" {
		t.Fatalf("dotenv not merged: %+v", env)
	}
}

func TestApplyListenPortSyncsAliases(t *testing.T) {
	env := map[string]string{
		"PROXY_PORT": "80",
	}
	applyListenPort(env, 3004)
	if env["PORT"] != "3004" || env["PROXY_PORT"] != "3004" {
		t.Fatalf("single-listen alias should sync: %+v", env)
	}
}

func TestApplyListenPortKeepsMonorepoPorts(t *testing.T) {
	env := map[string]string{
		"PROXY_PORT":    "80",
		"FRONTEND_PORT": "3002",
	}
	applyListenPort(env, 3002)
	if env["PORT"] != "3002" {
		t.Fatalf("PORT: %+v", env)
	}
	if env["PROXY_PORT"] != "80" || env["FRONTEND_PORT"] != "3002" {
		t.Fatalf("monorepo ports must stay authored: %+v", env)
	}
}

func TestWriteDotEnvFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := writeDotEnvFile(path, map[string]string{
		"FRONTEND_PORT": "3002",
		"PROXY_PORT":    "80",
		"NOTE":          "hello world",
	}); err != nil {
		t.Fatal(err)
	}
	got := readDotEnvFile(path)
	if got["FRONTEND_PORT"] != "3002" || got["PROXY_PORT"] != "80" || got["NOTE"] != "hello world" {
		t.Fatalf("%+v", got)
	}
}

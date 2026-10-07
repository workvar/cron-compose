package deploy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindCompose(t *testing.T) {
	dir := t.TempDir()
	if findCompose(dir) != "" {
		t.Fatal("expected empty")
	}
	p := filepath.Join(dir, "docker-compose.yml")
	if err := os.WriteFile(p, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := findCompose(dir); got != p {
		t.Errorf("got %q", got)
	}
}

func TestSystemdUnit(t *testing.T) {
	u := systemdUnit("web", "/opt/apps/node/web", "/usr/bin/npm start", map[string]string{"PORT": "3000"})
	for _, want := range []string{
		"WorkingDirectory=/opt/apps/node/web",
		"ExecStart=/usr/bin/npm start",
		"Environment=PORT=3000",
		"WantedBy=default.target",
	} {
		if !strings.Contains(u, want) {
			t.Errorf("missing %q in\n%s", want, u)
		}
	}
}

func TestStartCommandNode(t *testing.T) {
	bin, args := startCommand("node", "", "web")
	if bin != "npm" || strings.Join(args, " ") != "start" {
		t.Errorf("got %s %v", bin, args)
	}
	// Framework ids share the node runtime fallback.
	bin, args = startCommand("nextjs", "", "web")
	if bin != "npm" || strings.Join(args, " ") != "start" {
		t.Errorf("nextjs fallback got %s %v", bin, args)
	}
}

func TestStartCommandGo(t *testing.T) {
	bin, args := startCommand("go", "", "api")
	if bin != "./app" || len(args) != 0 {
		t.Errorf("got %s %v", bin, args)
	}
}

func TestStartCommandRunScript(t *testing.T) {
	bin, args := startCommand("go", "./server", "api")
	if bin != "./server" || len(args) != 0 {
		t.Errorf("got %s %v", bin, args)
	}
}

func TestPm2ArgsWithoutEcosystem(t *testing.T) {
	args := pm2StartArgs("web", "node", "", "")
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--name") || !strings.Contains(joined, "web") {
		t.Errorf("got %v", args)
	}
}

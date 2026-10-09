package deploy

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// Multi-app installs must overlap instead of waiting end-to-end. Two ~400ms
// sleeps in parallel finish near 400ms; sequentially they would take ~800ms.
func TestInstallAppsRunsInParallel(t *testing.T) {
	release := t.TempDir()
	webDir := filepath.Join(release, "apps", "web")
	apiDir := filepath.Join(release, "apps", "api")
	if err := os.MkdirAll(webDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(apiDir, 0o755); err != nil {
		t.Fatal(err)
	}

	var (
		mu   sync.Mutex
		logs []string
	)
	m := NewManager(slog.New(slog.DiscardHandler), func(ev *agentv1.DeployEvent) {
		if ev.GetKind() != "log" || len(ev.GetData()) == 0 {
			return
		}
		mu.Lock()
		logs = append(logs, string(ev.GetData()))
		mu.Unlock()
	})
	m.mu.Lock()
	m.seq["run"] = &atomic.Int32{}
	m.logged["run"] = &atomic.Int64{}
	m.mu.Unlock()

	cmd := &agentv1.DeployCommand{
		RunId: "run",
		Apps: []*agentv1.DeployApp{
			{Name: "web", RootDirectory: "apps/web", InstallScript: "sleep 0.4"},
			{Name: "api", RootDirectory: "apps/api", InstallScript: "sleep 0.4"},
		},
	}

	start := time.Now()
	if err := m.installApps(context.Background(), cmd, release, nil, ""); err != nil {
		t.Fatalf("installApps: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed > 700*time.Millisecond {
		t.Fatalf("installs took %v; expected parallel overlap under ~700ms", elapsed)
	}

	mu.Lock()
	joined := strings.Join(logs, "")
	mu.Unlock()
	if !strings.Contains(joined, "done (in "+webDir+")") {
		t.Fatalf("missing web done marker in logs:\n%s", joined)
	}
	if !strings.Contains(joined, "done (in "+apiDir+")") {
		t.Fatalf("missing api done marker in logs:\n%s", joined)
	}
	if !strings.Contains(joined, "sleep 0.4 (in "+webDir+")") {
		t.Fatalf("missing web install phase line:\n%s", joined)
	}
	if !strings.Contains(joined, "sleep 0.4 (in "+apiDir+")") {
		t.Fatalf("missing api install phase line:\n%s", joined)
	}
}

package identity

import (
	"os"
	"path/filepath"
	"testing"
)

// Production change that would fail this test: widening the file mode, which would
// expose the agent secret to other local users.
func TestSaveKeepsTheSecretAndIsPrivate(t *testing.T) {
	dir := t.TempDir()
	in := Identity{ServerID: "srv-1", ControlPlaneGRPCAddr: "grpc.example.com", AgentSecret: "sek", GRPCMode: "edge"}
	if err := Save(dir, in); err != nil {
		t.Fatal(err)
	}
	out, err := Load(dir)
	if err != nil || out != in {
		t.Fatalf("got %+v, %v", out, err)
	}
	fi, err := os.Stat(filepath.Join(dir, fileName))
	if err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v, %v", fi.Mode().Perm(), err)
	}
}

// Agents enrolled before edge mode have neither field and must still load.
func TestLoadOldIdentityWithoutEdgeFields(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, fileName), []byte(`{"server_id":"srv-1"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := Load(dir)
	if err != nil || out.ServerID != "srv-1" || out.AgentSecret != "" || out.GRPCMode != "" {
		t.Fatalf("got %+v, %v", out, err)
	}
}

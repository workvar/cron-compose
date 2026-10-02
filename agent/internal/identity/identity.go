// Package identity persists the agent's server_id and bearer token after Enroll. The
// file lives in the data dir with root-only perms.
package identity

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Identity is the persisted result of Enroll.
//
// After the mTLS upgrade, only ServerID is strictly needed at runtime: the real
// authenticator is the client cert on disk under DATA_DIR/tls/. We still record the
// gRPC address the control plane told us to dial.
type Identity struct {
	ServerID             string `json:"server_id"`
	ControlPlaneGRPCAddr string `json:"control_plane_grpc_addr,omitempty"`
	// AgentSecret and GRPCMode are only used in edge mode, where the connection passes
	// through an edge that ends TLS and a client certificate cannot be presented. The
	// file is written 0600. Empty for agents enrolled before edge mode existed.
	AgentSecret string `json:"agent_secret,omitempty"`
	GRPCMode    string `json:"grpc_mode,omitempty"`
}

const fileName = "identity.json"

// Load returns the saved identity, or os.ErrNotExist if not yet enrolled.
func Load(dataDir string) (Identity, error) {
	b, err := os.ReadFile(filepath.Join(dataDir, fileName))
	if errors.Is(err, os.ErrNotExist) {
		return Identity{}, os.ErrNotExist
	}
	if err != nil {
		return Identity{}, err
	}
	var id Identity
	if err := json.Unmarshal(b, &id); err != nil {
		return Identity{}, err
	}
	return id, nil
}

// Save writes the identity atomically with 0o600 perms.
func Save(dataDir string, id Identity) error {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(id, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(dataDir, fileName+".tmp")
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dataDir, fileName))
}

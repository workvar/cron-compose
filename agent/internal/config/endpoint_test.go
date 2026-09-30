package config

import "testing"

func TestLoadPrefersAgentNamesOverLegacy(t *testing.T) {
	t.Setenv("AGENT_GRPC_ADDR", "127.0.0.1:9077")
	t.Setenv("AGENT_GRPC_SNI", "grpc.example.com")
	t.Setenv("AGENT_ENROLL_HTTP", "http://127.0.0.1:8383/api/v1")
	t.Setenv("CONTROL_PLANE_ADDR", "grpc.example.com:9077")

	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.ControlPlaneAddr != "127.0.0.1:9077" || c.ControlPlaneSNI != "grpc.example.com" ||
		c.ControlPlaneHTTPBase != "http://127.0.0.1:8383/api/v1" {
		t.Fatalf("got %+v", c)
	}
	if len(c.Warnings) != 1 {
		t.Fatalf("a stale legacy value must produce one warning, got %v", c.Warnings)
	}
}

// Production change that would fail this test: dropping the legacy fallback, which
// would strand every agent installed with the old unit file after a self-update.
func TestLoadFallsBackToLegacyNames(t *testing.T) {
	t.Setenv("CONTROL_PLANE_ADDR", "cc.example.com:9090")
	t.Setenv("CONTROL_PLANE_SNI", "cc.example.com")
	t.Setenv("CONTROL_PLANE_HTTP", "https://cc.example.com/api")

	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.ControlPlaneAddr != "cc.example.com:9090" || c.ControlPlaneSNI != "cc.example.com" ||
		c.ControlPlaneHTTPBase != "https://cc.example.com/api" || len(c.Warnings) != 0 {
		t.Fatalf("got %+v", c)
	}
}

func TestLoadSameValueInBothNamesIsQuiet(t *testing.T) {
	t.Setenv("AGENT_GRPC_ADDR", "a:1")
	t.Setenv("CONTROL_PLANE_ADDR", "a:1")
	c, _ := Load()
	if len(c.Warnings) != 0 {
		t.Fatalf("got %v", c.Warnings)
	}
}

func TestLoadDefaults(t *testing.T) {
	c, _ := Load()
	if c.ControlPlaneAddr != "localhost:9090" || c.ControlPlaneSNI != "localhost" {
		t.Fatalf("got %+v", c)
	}
}

func TestGRPCAddrSetOnlyWhenOperatorSetIt(t *testing.T) {
	if c, _ := Load(); c.GRPCAddrSet {
		t.Fatal("defaults must not count as explicit")
	}
	t.Setenv("AGENT_GRPC_ADDR", "127.0.0.1:9077")
	if c, _ := Load(); !c.GRPCAddrSet {
		t.Fatal("AGENT_GRPC_ADDR must count as explicit")
	}
}

func TestGRPCAddrSetByLegacyName(t *testing.T) {
	t.Setenv("CONTROL_PLANE_ADDR", "cc.example.com:9090")
	if c, _ := Load(); !c.GRPCAddrSet {
		t.Fatal("legacy name must count as explicit")
	}
}

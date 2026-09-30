package runtime

import "testing"

// Production change that would fail this test: trusting an enrolled address with
// no host (":9077" from older control planes), so the agent dials itself.
func TestDialAddr(t *testing.T) {
	cfg := "cron.example.com:9077"
	cases := map[string]string{
		"":                        cfg,
		":9077":                   cfg,
		"not a host":              cfg,
		"grpc.example.com":        "grpc.example.com",
		"agents.example.com:5152": "agents.example.com:5152",
	}
	for enrolled, want := range cases {
		if got := dialAddr(cfg, false, enrolled); got != want {
			t.Errorf("dialAddr(%q)=%q want %q", enrolled, got, want)
		}
	}
}

// The reported case: enrolled against the public hostname, but this host reaches the
// control plane on loopback. Production change that would fail this test: preferring
// the enrolled address over an explicit AGENT_GRPC_ADDR, which silently ignores the
// override and leaves the agent dialing an address it cannot reach.
func TestDialAddrExplicitConfigBeatsEnrolled(t *testing.T) {
	got := dialAddr("127.0.0.1:9077", true, "grpc.workvar.com:9077")
	if got != "127.0.0.1:9077" {
		t.Fatalf("got %q", got)
	}
}

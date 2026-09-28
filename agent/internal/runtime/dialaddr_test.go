package runtime

import "testing"

// Production change that would fail this test: trusting an enrolled address with
// no host (":9077" from older control planes), so the agent dials itself.
func TestDialAddr(t *testing.T) {
	cfg := "cron.example.com:9077"
	cases := map[string]string{
		"":                        cfg,
		":9077":                   cfg,
		"garbage":                 cfg,
		"agents.example.com:5152": "agents.example.com:5152",
	}
	for enrolled, want := range cases {
		if got := dialAddr(cfg, enrolled); got != want {
			t.Errorf("dialAddr(%q)=%q want %q", enrolled, got, want)
		}
	}
}

package agentenroll

import "testing"

// Production change that would fail this test: advertising a listen address
// (":9077") at enrollment, which makes remote agents dial their own machine.
func TestAdvertisedGRPCAddr(t *testing.T) {
	cases := map[string]string{
		":9077":                   "",
		"":                        "",
		"garbage":                 "",
		"agents.example.com:5152": "agents.example.com:5152",
		"127.0.0.1:9090":          "127.0.0.1:9090",
	}
	for in, want := range cases {
		if got := advertisedGRPCAddr(in); got != want {
			t.Errorf("advertisedGRPCAddr(%q)=%q want %q", in, got, want)
		}
	}
}

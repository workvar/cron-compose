package agentenroll

import "testing"

func TestNormalizeMode(t *testing.T) {
	for in, want := range map[string]string{"edge": "edge", "mtls": "mtls", "": "mtls", "EDGE": "mtls", "typo": "mtls"} {
		if got := normalizeMode(in); got != want {
			t.Errorf("normalizeMode(%q)=%q want %q", in, got, want)
		}
	}
}

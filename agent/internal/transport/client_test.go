package transport

import (
	"errors"
	"strings"
	"testing"
)

// Production change that would fail this test: dropping the hint, so a port
// answered by an unrelated service reads as a bare TLS error.
func TestDialHint(t *testing.T) {
	cases := map[string]string{
		"tls: first record does not look like a TLS handshake": "something other than",
		"x509: certificate signed by unknown authority":        "something other than",
		"connect: connection refused":                          "nothing is listening",
		"lookup x: no such host":                               "does not resolve",
		"context deadline exceeded":                            "",
	}
	for msg, want := range cases {
		got := dialHint(errors.New(msg))
		if want == "" && got != "" || want != "" && !strings.Contains(got, want) {
			t.Errorf("dialHint(%q)=%q want contains %q", msg, got, want)
		}
	}
}

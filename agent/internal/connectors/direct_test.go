package connectors

import "testing"

func TestSplitRunAs(t *testing.T) {
	user, rest := splitRunAs([]byte("run_as:deploy"))
	if user != "deploy" || rest != nil {
		t.Fatalf("got %q %q", user, rest)
	}
	user, rest = splitRunAs([]byte("not-a-prefix"))
	if user != "" || string(rest) != "not-a-prefix" {
		t.Fatalf("plain content: %q %q", user, rest)
	}
	user, _ = splitRunAs([]byte("run_as:bad user"))
	if user != "" {
		t.Fatalf("rejected username came back as %q", user)
	}
}

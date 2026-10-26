//go:build windows

package connectors

import "testing"

func TestParseNetstatListen(t *testing.T) {
	raw := `
Active Connections

  Proto  Local Address          Foreign Address        State           PID
  TCP    0.0.0.0:135            0.0.0.0:0              LISTENING       1012
  TCP    127.0.0.1:3000         0.0.0.0:0              LISTENING       4242
  TCP    10.0.0.5:52044         140.82.112.3:443       ESTABLISHED     5555
  TCP    [::]:3000              [::]:0                 LISTENING       4242
`
	got := parseNetstatListen(raw)
	if len(got) != 3 {
		t.Fatalf("got %d sockets: %+v", len(got), got)
	}
	if got[1].Port != 3000 || got[1].PID != 4242 || got[1].Address != "127.0.0.1" {
		t.Fatalf("row 2 = %+v", got[1])
	}
	if got[2].Address != "[::]" {
		t.Fatalf("ipv6 address = %q", got[2].Address)
	}
}

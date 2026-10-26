//go:build windows

package connectors

import "testing"

func TestParseWinServicesArrayAndSingle(t *testing.T) {
	many, err := parseWinServices(`[{"Name":"a","State":"Running","ProcessId":4},{"Name":"b","State":"Stopped"}]`)
	if err != nil || len(many) != 2 || many[0].ProcessID != 4 {
		t.Fatalf("many = %+v, %v", many, err)
	}
	one, err := parseWinServices(`{"Name":"only","State":"Paused"}`)
	if err != nil || len(one) != 1 || winServiceState(one[0].State) != "paused" {
		t.Fatalf("one = %+v, %v", one, err)
	}
}

func TestValidServiceName(t *testing.T) {
	for _, bad := range []string{"", "a'b", "x; rm", `c:\x`} {
		if validServiceName(bad) {
			t.Errorf("%q should be rejected", bad)
		}
	}
	for _, good := range []string{"Spooler", "MSSQL$SQLEXPRESS", "croncompose-agent"} {
		if !validServiceName(good) {
			t.Errorf("%q should be accepted", good)
		}
	}
}

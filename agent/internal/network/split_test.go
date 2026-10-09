package network

import "testing"

func TestSplitNM(t *testing.T) {
	got := splitNM(`My\:SSID:80:WPA2:aa\:bb:yes`, 5)
	if len(got) != 5 {
		t.Fatalf("len=%d want 5: %#v", len(got), got)
	}
	if got[0] != "My:SSID" {
		t.Fatalf("ssid=%q", got[0])
	}
	if got[1] != "80" || got[2] != "WPA2" {
		t.Fatalf("mid=%v", got[1:3])
	}
	if got[3] != "aa:bb" {
		t.Fatalf("bssid=%q", got[3])
	}
}

func TestSplitNMNoLimit(t *testing.T) {
	got := splitNM(`a:b:c`, 0)
	if len(got) != 3 || got[0] != "a" || got[2] != "c" {
		t.Fatalf("%v", got)
	}
}

func TestParseArgs(t *testing.T) {
	a, err := parseArgs(`{"ssid":"home","alongside":true,"prefix":24}`)
	if err != nil {
		t.Fatal(err)
	}
	if a.SSID != "home" || !a.Alongside || a.Prefix != 24 {
		t.Fatalf("%+v", a)
	}
}

func TestDetectBackendUnavailable(t *testing.T) {
	// Without nmcli/netplan in a minimal env this may still find host tools;
	// just ensure it returns a known constant.
	b := DetectBackend(t.Context())
	switch b {
	case BackendNetworkManager, BackendNetplan, BackendUnavailable:
	default:
		t.Fatalf("unexpected backend %q", b)
	}
}

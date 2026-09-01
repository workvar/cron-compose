package agentgw

import "testing"

func TestVersionsEqual(t *testing.T) {
	if !VersionsEqual("v1.2.0", "1.2.0") {
		t.Fatal("expected equal")
	}
	if VersionsEqual("1.2.0", "1.3.0") {
		t.Fatal("expected different")
	}
}

func TestVersionNewer(t *testing.T) {
	cases := []struct {
		current, latest string
		want            bool
	}{
		{"0.1.0-dev", "v1.0.0", true},
		{"v1.0.0", "v1.0.0", false},
		{"v1.1.0", "v1.0.0", false},
		{"1.0.0", "1.0.1", true},
	}
	for _, tc := range cases {
		if got := VersionNewer(tc.current, tc.latest); got != tc.want {
			t.Fatalf("VersionNewer(%q, %q) = %v, want %v", tc.current, tc.latest, got, tc.want)
		}
	}
}

func TestPlatformTarget(t *testing.T) {
	if got := PlatformTarget("linux", "amd64"); got != "linux-amd64" {
		t.Fatalf("got %q", got)
	}
}

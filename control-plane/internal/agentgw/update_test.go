package agentgw

import "testing"

func TestSourcePolicyForwardsRepoWithoutChecksums(t *testing.T) {
	p := UpdatePolicy{Version: "v0.0.4", Repo: "workvar/cron-compose", Restart: true}
	if !p.Active() {
		t.Fatal("source policy should be active with version + repo")
	}
	up := p.For("v0.0.3", "linux", "arm64")
	if up == nil {
		t.Fatal("expected an update offer")
	}
	if up.GetTargetVersion() != "v0.0.4" {
		t.Errorf("target = %q", up.GetTargetVersion())
	}
	if up.GetDownloadUrl() != "https://github.com/workvar/cron-compose" {
		t.Errorf("url = %q", up.GetDownloadUrl())
	}
	if up.GetSha256() != "" {
		t.Errorf("sha256 = %q, want empty for source updates", up.GetSha256())
	}
	if !up.GetRestart() {
		t.Error("want restart")
	}
	if p.For("v0.0.4", "linux", "amd64") != nil {
		t.Fatal("already-current agent should not be offered an update")
	}
}

func TestBinaryPolicyStillRequiresChecksum(t *testing.T) {
	p := ParseUpdatePolicy("v1.0.0", "https://example.test/{version}", "", true)
	if p.Active() {
		t.Fatal("binary policy without checksums must stay inert")
	}
}

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

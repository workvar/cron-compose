package sourceupdate

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAssetName(t *testing.T) {
	if got := AssetName("windows", "amd64"); got != "croncompose-agent-windows-amd64.exe" {
		t.Fatalf("got %q", got)
	}
	if got := AssetName("linux", "arm64"); got != "croncompose-agent-linux-arm64" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveReleasePinsChecksum(t *testing.T) {
	digest := strings.Repeat("a", 64)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/acme/cc/releases/download/v1.2.3/checksums.json" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"windows/amd64":"` + digest + `"}`))
	}))
	defer srv.Close()
	old := releaseBase
	releaseBase = srv.URL
	defer func() { releaseBase = old }()

	req, err := resolveRelease(context.Background(), "acme/cc", "v1.2.3", "windows", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if req.SHA256 != digest || !strings.HasSuffix(req.DownloadURL, "/v1.2.3/croncompose-agent-windows-amd64.exe") {
		t.Fatalf("req = %+v", req)
	}
	if _, err := resolveRelease(context.Background(), "acme/cc", "v1.2.3", "windows", "arm64"); err == nil {
		t.Fatal("expected an error for a platform the release does not publish")
	}
}

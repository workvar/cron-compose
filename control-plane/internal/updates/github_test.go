package updates

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchLatestReleaseReadsChecksums(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/checksums.json", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"linux/amd64": "abc123"})
	})
	var srv *httptest.Server
	mux.HandleFunc("/repos/workvar/cron-compose/releases/latest", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(ghRelease{
			TagName: "v0.0.2",
			HTMLURL: "https://github.com/workvar/cron-compose/releases/tag/v0.0.2",
			Assets: []ghAsset{{
				Name:               "checksums.json",
				BrowserDownloadURL: srv.URL + "/checksums.json",
			}},
		})
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	prev := githubAPI
	githubAPI = srv.URL
	t.Cleanup(func() { githubAPI = prev })

	rel, err := fetchLatestRelease(context.Background(), "workvar/cron-compose")
	if err != nil {
		t.Fatalf("fetchLatestRelease: %v", err)
	}
	if rel.Version != "v0.0.2" {
		t.Errorf("Version = %q, want v0.0.2", rel.Version)
	}
	if rel.Checksums["linux/amd64"] != "abc123" {
		t.Errorf("checksums = %#v", rel.Checksums)
	}
}

func TestFetchLatestReleaseNotesOnly(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/workvar/cron-compose/releases/latest", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(ghRelease{
			TagName: "v0.0.4",
			HTMLURL: "https://github.com/workvar/cron-compose/releases/tag/v0.0.4",
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	prev := githubAPI
	githubAPI = srv.URL
	t.Cleanup(func() { githubAPI = prev })

	rel, err := fetchLatestRelease(context.Background(), "workvar/cron-compose")
	if err != nil {
		t.Fatalf("fetchLatestRelease: %v", err)
	}
	if rel.Version != "v0.0.4" {
		t.Errorf("Version = %q, want v0.0.4", rel.Version)
	}
	if len(rel.Checksums) != 0 {
		t.Errorf("checksums = %#v, want none for a notes-only release", rel.Checksums)
	}
}

func TestFetchLatestReleaseMissingRepo(t *testing.T) {
	_, err := fetchLatestRelease(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty repo")
	}
}

func TestTargetToPlatform(t *testing.T) {
	if got := targetToPlatform("linux-amd64"); got != "linux/amd64" {
		t.Errorf("got %q", got)
	}
}

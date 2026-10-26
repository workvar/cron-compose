package sourceupdate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/croncompose/croncompose/agent/internal/selfupdate"
)

// releaseBase is where GitHub serves release assets. A var so tests can point it
// at a local server.
var releaseBase = "https://github.com"

// AssetName is the release asset for one platform, matching the release workflow:
// croncompose-agent-linux-amd64, croncompose-agent-windows-amd64.exe, ...
func AssetName(goos, goarch string) string {
	name := "croncompose-agent-" + goos + "-" + goarch
	if goos == "windows" {
		name += ".exe"
	}
	return name
}

// ApplyReleaseBinary answers a source-update offer by installing the prebuilt binary
// from the same GitHub release instead of compiling. Windows uses this: it needs no
// Go toolchain or git on the server, and the download is pinned to the sha256 the
// release publishes in checksums.json, so it is as verified as a binary offer.
func ApplyReleaseBinary(ctx context.Context, log *slog.Logger, downloadURL, tag, current string, report Reporter) (string, error) {
	repo := repoFromGitHubURL(downloadURL)
	if repo == "" {
		return "", fmt.Errorf("not a github source url: %s", downloadURL)
	}
	report.report("fetching", "Reading release checksums", 10)
	req, err := resolveRelease(ctx, repo, tag, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", err
	}
	log.Info("release binary update", "url", req.DownloadURL, "sha256", req.SHA256)
	report.report("downloading", "Downloading the new agent binary", 30)
	path, err := selfupdate.Apply(ctx, req, current)
	if err != nil {
		return "", err
	}
	report.report("installing", "Installed the new agent binary", 80)
	return path, nil
}

// resolveRelease builds a pinned selfupdate.Request for this platform's asset.
func resolveRelease(ctx context.Context, repo, tag, goos, goarch string) (selfupdate.Request, error) {
	base := fmt.Sprintf("%s/%s/releases/download/%s/", releaseBase, repo, tag)
	sums, err := fetchChecksums(ctx, base+"checksums.json")
	if err != nil {
		return selfupdate.Request{}, err
	}
	key := goos + "/" + goarch
	sum := strings.TrimSpace(sums[key])
	if sum == "" {
		return selfupdate.Request{}, fmt.Errorf("release %s publishes no binary for %s", tag, key)
	}
	return selfupdate.Request{
		TargetVersion: tag,
		DownloadURL:   base + AssetName(goos, goarch),
		SHA256:        sum,
	}, nil
}

func fetchChecksums(ctx context.Context, url string) (map[string]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "croncompose-agent-selfupdate/1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("checksums.json: HTTP %d", resp.StatusCode)
	}
	var m map[string]string
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&m); err != nil {
		return nil, fmt.Errorf("checksums.json: %w", err)
	}
	return m, nil
}

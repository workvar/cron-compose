package updates

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var githubAPI = "https://api.github.com"

type ghRelease struct {
	TagName     string    `json:"tag_name"`
	HTMLURL     string    `json:"html_url"`
	PublishedAt time.Time `json:"published_at"`
	Assets      []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func fetchLatestRelease(ctx context.Context, repo string) (*Release, error) {
	if repo == "" {
		return nil, fmt.Errorf("github repo not configured")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		githubAPI+"/repos/"+repo+"/releases/latest", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "croncompose-control-plane")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("no releases published yet")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("github api: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var raw ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	tag := strings.TrimSpace(raw.TagName)
	if tag == "" {
		return nil, fmt.Errorf("release has no tag")
	}

	checksums, err := loadChecksums(ctx, raw.Assets)
	if err != nil {
		return nil, err
	}

	return &Release{
		Version:     tag,
		Tag:         tag,
		ReleaseURL:  raw.HTMLURL,
		PublishedAt: raw.PublishedAt,
		Checksums:   checksums,
	}, nil
}

func loadChecksums(ctx context.Context, assets []ghAsset) (map[string]string, error) {
	out := map[string]string{}

	for _, a := range assets {
		switch a.Name {
		case "checksums.json":
			m, err := fetchChecksumsJSON(ctx, a.BrowserDownloadURL)
			if err != nil {
				return nil, err
			}
			for k, v := range m {
				out[k] = v
			}
		}
	}

	// Map bare agent binaries to os/arch keys when checksums.json is absent.
	for _, a := range assets {
		const prefix = "croncompose-agent-"
		if !strings.HasPrefix(a.Name, prefix) {
			continue
		}
		target := strings.TrimPrefix(a.Name, prefix)
		key := targetToPlatform(target)
		if key == "" {
			continue
		}
		if _, ok := out[key]; ok {
			continue
		}
		sum, err := hashRemoteFile(ctx, a.BrowserDownloadURL)
		if err != nil {
			return nil, fmt.Errorf("hash %s: %w", a.Name, err)
		}
		out[key] = sum
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("release has no agent binaries or checksums.json")
	}
	return out, nil
}

func fetchChecksumsJSON(ctx context.Context, url string) (map[string]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("checksums.json: HTTP %d", resp.StatusCode)
	}
	var m map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, err
	}
	return m, nil
}

func hashRemoteFile(ctx context.Context, url string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download: HTTP %d", resp.StatusCode)
	}
	h := sha256.New()
	if _, err := io.Copy(h, io.LimitReader(resp.Body, 256<<20)); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func targetToPlatform(target string) string {
	switch target {
	case "linux-amd64":
		return "linux/amd64"
	case "linux-arm64":
		return "linux/arm64"
	case "linux-armv7":
		return "linux/arm"
	case "darwin-amd64":
		return "darwin/amd64"
	case "darwin-arm64":
		return "darwin/arm64"
	default:
		if i := strings.Index(target, "-"); i > 0 {
			return target[:i] + "/" + target[i+1:]
		}
		return ""
	}
}

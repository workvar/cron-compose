// Package autoupdate asks GitHub for the latest agent release so an agent can keep
// itself current without an operator pressing "Update" in the UI.
//
// It only discovers the tag. Installing goes through the same path as an update the
// control plane offers (runtime.handleUpdate), so a discovered release is verified
// against the sha256 the release publishes before the binary is swapped.
package autoupdate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// apiBase is the GitHub REST endpoint. A var so tests can point it elsewhere.
var apiBase = "https://api.github.com"

// Latest returns the tag of repo's latest published (non-draft, non-prerelease)
// release.
func Latest(ctx context.Context, repo string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBase+"/repos/"+repo+"/releases/latest", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "croncompose-agent-autoupdate/1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github latest release: HTTP %d", resp.StatusCode)
	}
	var body struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body); err != nil {
		return "", err
	}
	if strings.TrimSpace(body.TagName) == "" {
		return "", fmt.Errorf("latest release has no tag")
	}
	return strings.TrimSpace(body.TagName), nil
}

// Newer reports whether latest is strictly newer than current, comparing dotted
// numeric segments after a leading "v" and ignoring pre-release suffixes. It
// mirrors the control plane's agentgw.VersionNewer so both sides agree.
func Newer(current, latest string) bool {
	cur, lat := parts(current), parts(latest)
	for i := 0; i < len(cur) || i < len(lat); i++ {
		var c, l int
		if i < len(cur) {
			c = cur[i]
		}
		if i < len(lat) {
			l = lat[i]
		}
		if l != c {
			return l > c
		}
	}
	return false
}

func parts(v string) []int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	if v == "" {
		return nil
	}
	var out []int
	for _, p := range strings.Split(v, ".") {
		n := 0
		for _, ch := range p {
			if ch < '0' || ch > '9' {
				break
			}
			n = n*10 + int(ch-'0')
		}
		out = append(out, n)
	}
	return out
}

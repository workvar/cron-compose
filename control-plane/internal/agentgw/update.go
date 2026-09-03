package agentgw

import (
	"encoding/json"
	"strings"

	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// UpdatePolicy describes the release this control plane can offer to agents.
//
// Source updates (Version + Repo) tell the agent to git-checkout that tag and build
// locally. Binary updates (Version + URLTemplate + checksums) stay available as a
// manual AGENT_UPDATE_* override.
type UpdatePolicy struct {
	// Version is the target agent version string, matched against Hello.agent_version.
	Version string
	// URLTemplate builds a binary download URL. {version}, {os}, {arch}, {target} and
	// {repo} are replaced. Unused for source updates.
	URLTemplate string
	// Repo is the GitHub owner/repo agents clone for a source update
	// (e.g. workvar/cron-compose). Also substituted for {repo} in URLTemplate.
	Repo string
	// Checksums maps "os/arch" to a sha256 hex digest. Required only for binary updates.
	Checksums map[string]string
	// Restart tells the agent to exit after swapping the binary so its supervisor
	// brings it back on the new one. Off means the swap takes effect on next restart.
	Restart bool
}

// ParseUpdatePolicy builds a policy from the three configuration values. An empty
// version yields a zero policy, and a malformed checksum map is treated as absent
// rather than fatal: a bad value here must not stop the control plane from starting.
func ParseUpdatePolicy(version, urlTemplate, checksumsJSON string, restart bool) UpdatePolicy {
	p := UpdatePolicy{
		Version:     strings.TrimSpace(version),
		URLTemplate: strings.TrimSpace(urlTemplate),
		Restart:     restart,
		Checksums:   map[string]string{},
	}
	if s := strings.TrimSpace(checksumsJSON); s != "" {
		if strings.HasPrefix(s, "{") {
			_ = json.Unmarshal([]byte(s), &p.Checksums)
		} else {
			// A bare digest is the common single-platform case.
			p.Checksums["*"] = s
		}
	}
	return p
}

// Active reports whether the policy is complete enough to act on.
func (p UpdatePolicy) Active() bool {
	if strings.TrimSpace(p.Version) == "" {
		return false
	}
	if strings.TrimSpace(p.Repo) != "" {
		return true
	}
	return p.URLTemplate != "" && len(p.Checksums) > 0
}

func (p UpdatePolicy) source() bool {
	return strings.TrimSpace(p.Repo) != "" && (p.URLTemplate == "" || len(p.Checksums) == 0)
}

// For builds the update message for one agent, or nil if there is nothing to send.
func (p UpdatePolicy) For(agentVersion, os, arch string) *agentv1.UpdateAgent {
	if !p.Active() || VersionsEqual(agentVersion, p.Version) {
		return nil
	}
	if p.source() {
		return &agentv1.UpdateAgent{
			TargetVersion: p.Version,
			DownloadUrl:   "https://github.com/" + strings.TrimSpace(p.Repo),
			Restart:       p.Restart,
		}
	}
	sum := p.checksumFor(os, arch)
	if sum == "" {
		return nil
	}
	return &agentv1.UpdateAgent{
		TargetVersion: p.Version,
		DownloadUrl:   p.urlFor(os, arch),
		Sha256:        sum,
		Restart:       p.Restart,
	}
}

func (p UpdatePolicy) checksumFor(os, arch string) string {
	if s, ok := p.Checksums[os+"/"+arch]; ok {
		return s
	}
	return p.Checksums["*"]
}

func (p UpdatePolicy) urlFor(os, arch string) string {
	r := strings.NewReplacer(
		"{version}", p.Version,
		"{os}", os,
		"{arch}", arch,
		"{target}", PlatformTarget(os, arch),
		"{repo}", p.Repo,
	)
	return r.Replace(p.URLTemplate)
}

// PlatformTarget maps runtime os/arch to the release asset suffix used by the
// installer and GitHub release artifacts.
func PlatformTarget(os, arch string) string {
	switch {
	case os == "linux" && (arch == "amd64" || arch == "386"):
		return "linux-amd64"
	case os == "linux" && arch == "arm64":
		return "linux-arm64"
	case os == "linux" && (arch == "arm" || arch == "armv7"):
		return "linux-armv7"
	case os == "darwin" && arch == "arm64":
		return "darwin-arm64"
	case os == "darwin" && arch == "amd64":
		return "darwin-amd64"
	default:
		return os + "-" + arch
	}
}

// NormalizeVersion strips a leading "v" so v1.2.0 and 1.2.0 compare equal.
func NormalizeVersion(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

// VersionsEqual reports whether two agent version strings refer to the same release.
func VersionsEqual(a, b string) bool {
	return NormalizeVersion(a) == NormalizeVersion(b)
}

// VersionNewer reports whether latest is strictly newer than current. Both strings
// are compared as dot-separated numeric segments after normalizing the leading "v".
func VersionNewer(current, latest string) bool {
	cur := parseVersionParts(NormalizeVersion(current))
	lat := parseVersionParts(NormalizeVersion(latest))
	for i := 0; i < len(cur) || i < len(lat); i++ {
		var c, l int
		if i < len(cur) {
			c = cur[i]
		}
		if i < len(lat) {
			l = lat[i]
		}
		if l > c {
			return true
		}
		if l < c {
			return false
		}
	}
	return false
}

func parseVersionParts(v string) []int {
	if v == "" {
		return nil
	}
	// Drop pre-release suffix: 1.2.0-dev -> 1.2.0
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
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

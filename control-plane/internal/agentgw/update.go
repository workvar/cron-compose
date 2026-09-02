package agentgw

import (
	"encoding/json"
	"strings"

	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// UpdatePolicy describes the agent build this control plane wants its fleet to run.
//
// It is deliberately configuration rather than a service: a self-hosted control plane
// should not need to reach an upstream release feed to keep its own agents current,
// and the operator who builds the agents is the one who knows where they live.
//
// A policy with no version is inert, which is the default. Nothing updates until an
// operator says what to update to.
type UpdatePolicy struct {
	// Version is the target agent version string, matched against Hello.agent_version.
	Version string
	// URLTemplate builds the download URL. {version}, {os}, {arch}, {target} and {repo}
	// are replaced. Example:
	// https://github.com/{repo}/releases/download/{version}/croncompose-agent-{target}
	URLTemplate string
	// Repo is substituted for {repo} in URLTemplate (e.g. workvar/cron-compose).
	Repo string
	// Checksums maps "os/arch" to a sha256 hex digest. A single-entry map keyed "*"
	// covers a homogeneous fleet.
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
	return p.Version != "" && p.URLTemplate != "" && len(p.Checksums) > 0
}

// For builds the update message for one agent, or nil if there is nothing to send.
//
// Nothing is sent when the agent already runs the target version, when the platform
// has no pinned checksum, or when the policy is incomplete. Sending an update without
// a checksum is not a degraded mode we offer.
func (p UpdatePolicy) For(agentVersion, os, arch string) *agentv1.UpdateAgent {
	if !p.Active() || VersionsEqual(agentVersion, p.Version) {
		return nil
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

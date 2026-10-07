package deploys

import "strings"

// NormalizeRedeployOn keeps only known modes and defaults to branch push.
func NormalizeRedeployOn(modes []string) []string {
	allowed := map[string]bool{"branch": true, "tag": true, "release": true}
	out := make([]string, 0, len(modes))
	seen := map[string]bool{}
	for _, m := range modes {
		if !allowed[m] || seen[m] {
			continue
		}
		seen[m] = true
		out = append(out, m)
	}
	if len(out) == 0 {
		return []string{"branch"}
	}
	return out
}

// WantsRedeploy reports whether the project is configured for this trigger mode.
func WantsRedeploy(p Project, mode string) bool {
	modes := p.RedeployOn
	if len(modes) == 0 {
		modes = []string{"branch"}
	}
	for _, m := range modes {
		if m == mode {
			return true
		}
	}
	return false
}

// TagFromRef extracts a tag name from refs/tags/<name>. Empty for other refs.
func TagFromRef(ref string) string {
	const prefix = "refs/tags/"
	if strings.HasPrefix(ref, prefix) {
		return strings.TrimPrefix(ref, prefix)
	}
	return ""
}

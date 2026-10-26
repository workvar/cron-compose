package deploys

import (
	"fmt"
	"path"
	"strings"
)

// runScope limits a deploy to one app and, optionally, one step.
// Empty apps means every app. Empty onlyPhase means that app's full chain.
type runScope struct {
	apps      []string
	onlyPhase string
}

func normalizePhase(p string) (string, error) {
	p = strings.ToLower(strings.TrimSpace(strings.ReplaceAll(p, "-", "_")))
	p = strings.ReplaceAll(p, " ", "_")
	if p == "" || p == "all" {
		return "", nil
	}
	switch p {
	case "install", "build", "cleanup", "pre_start", "run", "start", "health":
		return p, nil
	default:
		return "", fmt.Errorf("unknown phase %q", p)
	}
}

func allowSet(names []string) map[string]bool {
	allow := map[string]bool{}
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name != "" {
			allow[name] = true
		}
	}
	return allow
}

// appAllowed reports whether name (or its folder) is in the restart set.
// An empty allow list means every app.
func appAllowed(allow map[string]bool, name, root string) bool {
	if len(allow) == 0 {
		return true
	}
	if allow[name] {
		return true
	}
	base := path.Base(strings.Trim(strings.TrimSpace(root), "/"))
	for k := range allow {
		if strings.EqualFold(k, name) {
			return true
		}
		if base != "" && base != "." && strings.EqualFold(k, base) {
			return true
		}
	}
	return false
}

func phaseKept(phase, only string) bool {
	if only == "" {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(phase), only)
}

func postReleasePhase(p string) bool {
	switch strings.ToLower(strings.TrimSpace(p)) {
	case "pre_start", "run", "start", "health":
		return true
	default:
		return false
	}
}

// filterFlow keeps tasks for the allowed apps and, when onlyPhase is set, that step.
func filterFlow(flow [][]FlowTask, allow map[string]bool, onlyPhase string) [][]FlowTask {
	var out [][]FlowTask
	for _, wave := range flow {
		var tasks []FlowTask
		for _, t := range wave {
			if !appAllowed(allow, t.App, "") || !phaseKept(t.Phase, onlyPhase) {
				continue
			}
			tasks = append(tasks, t)
		}
		if len(tasks) > 0 {
			out = append(out, tasks)
		}
	}
	return out
}

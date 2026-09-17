package sourceupdate

import (
	"strings"
	"unicode"
)

// Step is one human-readable stage parsed from update.sh output.
type Step struct {
	Phase   string
	Detail  string
	Percent int
}

type stepMatch struct {
	needle  string
	phase   string
	detail  string
	percent int
}

// Longest / most specific needles first so "Starting / restarting" wins over
// "Restarting services", and "Building images" wins over a generic "Building".
var updateSteps = []stepMatch{
	{"Starting / restarting services", "restarting", "Starting updated containers", 88},
	{"Restarting services", "stopping", "Stopping the server so it can restart", 85},
	{"Applying database migrations", "migrating", "Applying database migrations", 72},
	{"Building web UI", "downloading", "Downloading packages and building the web UI", 55},
	{"Building images", "downloading", "Downloading and building Docker images", 40},
	{"Building Go binaries", "building", "Compiling control plane, agent, and CLI", 35},
	{"Updating source", "fetching", "Fetching the new release from git", 12},
	{"Restoring source", "fetching", "Restoring the source tree", 10},
	{"Mode: compose", "downloading", "Preparing to build Docker images", 18},
	{"Mode: source", "building", "Preparing to rebuild from source", 18},
	{"Done", "restarting", "Restarting so the new version can come up", 92},
}

// ParseUpdateStep maps one update.sh log line onto a UI stage. Lines that are
// not `==>` step headers return ok=false.
func ParseUpdateStep(line string) (Step, bool) {
	line = stripANSI(line)
	line = strings.TrimSpace(line)
	idx := strings.Index(line, "==>")
	if idx < 0 {
		return Step{}, false
	}
	rest := strings.TrimSpace(line[idx+3:])
	if rest == "" {
		return Step{}, false
	}
	for _, m := range updateSteps {
		if strings.Contains(rest, m.needle) {
			return Step{Phase: m.phase, Detail: m.detail, Percent: m.percent}, true
		}
	}
	return Step{}, false
}

func stripANSI(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && !unicode.IsLetter(rune(s[j])) && s[j] != '@' {
				j++
			}
			if j < len(s) {
				i = j
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

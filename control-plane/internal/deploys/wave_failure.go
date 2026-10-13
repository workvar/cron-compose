package deploys

import "strings"

// Wave failure policies for multi-app start waves.
const (
	OnWaveFailureStop     = "stop"
	OnWaveFailureContinue = "continue"
)

// NormalizeOnWaveFailure returns stop or continue. Empty / unknown → stop.
func NormalizeOnWaveFailure(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case OnWaveFailureContinue:
		return OnWaveFailureContinue
	default:
		return OnWaveFailureStop
	}
}

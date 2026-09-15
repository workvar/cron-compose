package notify

// Wording shared by the Slack and email channels for deploy events. A deploy
// notification has one job: tell someone what is running right now and what they have
// to do about it. "Deploy failed" on its own does neither.

// phaseLabel turns the agent's stage into something a human reads at 3am. The health
// stage is the one worth spelling out: it means the code built and installed fine and
// the app still did not come up, which points at runtime config rather than the build.
func phaseLabel(phase string) string {
	switch phase {
	case "preflight":
		return "Preflight (server not ready)"
	case "clone":
		return "Clone"
	case "install":
		return "Install / build"
	case "release":
		return "Release activation"
	case "start":
		return "Process start"
	case "health":
		return "Health check (built fine, app did not come up)"
	default:
		return ""
	}
}

// stateLabel describes what is serving now, which is the part someone paged about a
// deploy actually needs.
func stateLabel(state string) string {
	switch state {
	case "healthy":
		return "healthy"
	case "degraded":
		return "degraded, the failed deploy is what is live"
	case "rolled_back":
		return "running the previous commit after an automatic rollback"
	default:
		return ""
	}
}

// rollbackSuffix renders the automatic-rollback outcome for a headline or subject
// line. Empty for anything that isn't a rollback run's own result.
func rollbackSuffix(ev RunFailedEvent) string {
	switch {
	case ev.RolledBack && ev.Status == "succeeded":
		return " (recovered via rollback)"
	case ev.RolledBack:
		return " (rollback also failed)"
	}
	return ""
}

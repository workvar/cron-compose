package agentenroll

// normalizeMode maps the configured connection mode to one an agent understands.
// Anything but "edge" is the default mutual-TLS mode, so a typo never silently
// switches agents to the secret-authenticated listener.
func normalizeMode(m string) string {
	if m == "edge" {
		return "edge"
	}
	return "mtls"
}

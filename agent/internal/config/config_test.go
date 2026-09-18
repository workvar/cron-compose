package config

import "testing"

func TestResolveAgentVersionPrefersLinkedRelease(t *testing.T) {
	prev := buildVersion
	t.Cleanup(func() { buildVersion = prev })

	buildVersion = "0.0.13"
	t.Setenv("AGENT_VERSION", "v0.0.11")
	if got := resolveAgentVersion(); got != "0.0.13" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveAgentVersionFallsBackForDevPlaceholder(t *testing.T) {
	prev := buildVersion
	t.Cleanup(func() { buildVersion = prev })

	buildVersion = "0.1.0-dev"
	t.Setenv("AGENT_VERSION", "0.9.9-local")
	if got := resolveAgentVersion(); got != "0.9.9-local" {
		t.Fatalf("got %q", got)
	}
}

package servers

import (
	"encoding/json"
	"testing"
)

// Production change that would fail this test: dropping agent_root_enabled or
// agent_euid_root from GET /servers/:id JSON.
func TestServerJSONIncludesRootFlags(t *testing.T) {
	s := Server{
		ID:               "srv-1",
		Name:             "pi",
		Labels:           map[string]string{},
		Status:           "online",
		AgentRootEnabled: true,
		AgentEuidRoot:    true,
	}
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got["agent_root_enabled"] != true {
		t.Fatalf("agent_root_enabled=%v", got["agent_root_enabled"])
	}
	if got["agent_euid_root"] != true {
		t.Fatalf("agent_euid_root=%v", got["agent_euid_root"])
	}
	if _, ok := got["agent_root_error"]; ok {
		t.Fatal("empty agent_root_error should be omitted")
	}
}

package agentgw

import "testing"

func TestRegistryRemoveReportsWhetherStreamWasLive(t *testing.T) {
	r := NewRegistry()
	first := r.Add("srv-1")
	second := r.Add("srv-1") // replaces first; first must not mark the server offline

	if r.Remove(first) {
		t.Fatal("removing a superseded Conn must not report offline")
	}
	if !r.Remove(second) {
		t.Fatal("removing the live Conn must report offline")
	}
	if r.Remove(second) {
		t.Fatal("removing an already-gone Conn must not report offline again")
	}
}

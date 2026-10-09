package agentgw

import (
	"testing"

	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

func TestPendingNetworkPushEventAndResolve(t *testing.T) {
	p := NewPendingNetworkRequests()
	sub := p.Open("req-1")

	p.PushEvent(&agentv1.NetworkEvent{
		RequestId: "req-1",
		Kind:      "pin_required",
		PinDevice: "AA:BB",
		PinMethod: "pin",
	})
	select {
	case ev := <-sub.Events():
		if ev.GetKind() != "pin_required" || ev.GetPinDevice() != "AA:BB" {
			t.Fatalf("%+v", ev)
		}
	default:
		t.Fatal("expected event")
	}

	p.Resolve(&agentv1.NetworkResult{RequestId: "req-1", Status: "succeeded"})
	select {
	case res := <-sub.Result():
		if res.GetStatus() != "succeeded" {
			t.Fatalf("%+v", res)
		}
	default:
		t.Fatal("expected result")
	}

	p.Close("req-1")
}

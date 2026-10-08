package agentgw

import (
	"testing"

	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

func TestPendingToolsPushLogAndResolve(t *testing.T) {
	p := NewPendingToolRequests()
	sub := p.Open("req-1")
	defer p.Close("req-1")

	p.PushLog(&agentv1.HostToolsEvent{RequestId: "req-1", Kind: "log", Data: []byte("hi\n"), Percent: 10, Seq: 1})
	select {
	case ev := <-sub.Logs():
		if string(ev.GetData()) != "hi\n" || ev.GetPercent() != 10 {
			t.Fatalf("log=%v", ev)
		}
	default:
		t.Fatal("expected log event")
	}

	p.Resolve(&agentv1.HostToolsResult{RequestId: "req-1", Status: "succeeded"})
	select {
	case res := <-sub.Result():
		if res.GetStatus() != "succeeded" {
			t.Fatalf("status=%q", res.GetStatus())
		}
	default:
		t.Fatal("expected result")
	}
}

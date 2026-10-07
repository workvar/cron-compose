package runtime

import (
	"context"
	"strings"

	"github.com/croncompose/croncompose/agent/internal/hosttools"
	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

func (r *Runtime) handleHostToolsRequest(req *agentv1.HostToolsRequest) {
	if req == nil {
		return
	}
	res := &agentv1.HostToolsResult{RequestId: req.GetRequestId()}
	ctx := context.Background()
	op := strings.ToLower(strings.TrimSpace(req.GetOp()))
	switch op {
	case "", "detect":
		tools, err := hosttools.Detect(ctx, req.GetRunAs())
		if err != nil {
			res.Error = err.Error()
			res.Status = "failed"
		} else {
			res.Tools = tools
			res.Status = "succeeded"
		}
	case "install":
		tools, log, code, err := hosttools.Install(ctx, req.GetRunAs(), req.GetTool())
		res.Tools = tools
		res.Log = log
		res.ExitCode = int32(code)
		if err != nil {
			res.Error = err.Error()
			res.Status = "failed"
		} else {
			res.Status = "succeeded"
		}
	default:
		res.Error = "unknown op " + op
		res.Status = "failed"
	}
	r.sendDirect(&agentv1.AgentMessage{
		Body: &agentv1.AgentMessage_HostToolsResult{HostToolsResult: res},
	})
}

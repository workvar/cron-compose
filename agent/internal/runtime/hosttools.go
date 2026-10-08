package runtime

import (
	"context"
	"strings"
	"sync/atomic"

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
		var tools []*agentv1.ToolStatus
		var err error
		if t := strings.TrimSpace(req.GetTool()); t != "" {
			tools, err = hosttools.Detect(ctx, req.GetRunAs(), t)
		} else {
			tools, err = hosttools.Detect(ctx, req.GetRunAs())
		}
		if err != nil {
			res.Error = err.Error()
			res.Status = "failed"
		} else {
			res.Tools = tools
			res.Status = "succeeded"
		}
	case "install":
		r.runHostToolsMutate(ctx, req, res, true)
	case "uninstall":
		r.runHostToolsMutate(ctx, req, res, false)
	default:
		res.Error = "unknown op " + op
		res.Status = "failed"
	}
	r.sendDirect(&agentv1.AgentMessage{
		Body: &agentv1.AgentMessage_HostToolsResult{HostToolsResult: res},
	})
}

func (r *Runtime) runHostToolsMutate(ctx context.Context, req *agentv1.HostToolsRequest, res *agentv1.HostToolsResult, install bool) {
	var seq atomic.Int32
	onLog := func(chunk []byte, percent int32) {
		n := seq.Add(1)
		r.sendDirect(&agentv1.AgentMessage{
			Body: &agentv1.AgentMessage_HostToolsEvent{
				HostToolsEvent: &agentv1.HostToolsEvent{
					RequestId: req.GetRequestId(),
					Kind:      "log",
					Data:      chunk,
					Percent:   percent,
					Seq:       n,
				},
			},
		})
	}

	var tools []*agentv1.ToolStatus
	var log string
	var code int
	var err error
	if install {
		tools, log, code, err = hosttools.Install(ctx, req.GetRunAs(), req.GetTool(), onLog)
	} else {
		tools, log, code, err = hosttools.Uninstall(ctx, req.GetRunAs(), req.GetTool(), onLog)
	}
	res.Tools = tools
	res.Log = log
	res.ExitCode = int32(code)
	if err != nil {
		res.Error = err.Error()
		res.Status = "failed"
	} else {
		res.Status = "succeeded"
	}
}

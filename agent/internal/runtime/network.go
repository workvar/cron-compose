package runtime

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"

	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

func (r *Runtime) handleNetworkRequest(req *agentv1.NetworkRequest) {
	if req == nil || r.network == nil {
		return
	}
	op := strings.ToLower(strings.TrimSpace(req.GetOp()))
	// PIN replies unblock an in-flight pair without closing that request's result.
	if op == "bt_pin_reply" {
		targetID := req.GetRequestId()
		var raw struct {
			PIN           string `json:"pin"`
			ForRequestID  string `json:"for_request_id"`
		}
		_ = json.Unmarshal([]byte(req.GetArgsJson()), &raw)
		if raw.ForRequestID != "" {
			targetID = raw.ForRequestID
		}
		ok := r.network.Pins.Deliver(targetID, raw.PIN)
		status := "succeeded"
		errMsg := ""
		if !ok {
			status = "failed"
			errMsg = "no pending pin challenge for this request_id"
		}
		r.sendDirect(&agentv1.AgentMessage{
			Body: &agentv1.AgentMessage_NetworkResult{
				NetworkResult: &agentv1.NetworkResult{
					RequestId:  req.GetRequestId(),
					Status:     status,
					Error:      errMsg,
					ResultJson: `{"accepted":` + boolJSON(ok) + `}`,
				},
			},
		})
		return
	}

	var seq atomic.Int32
	emit := func(kind, pinDevice, pinMethod, pinPrompt string, data []byte) {
		n := seq.Add(1)
		r.sendDirect(&agentv1.AgentMessage{
			Body: &agentv1.AgentMessage_NetworkEvent{
				NetworkEvent: &agentv1.NetworkEvent{
					RequestId: req.GetRequestId(),
					Kind:      kind,
					Data:      data,
					Seq:       n,
					PinDevice: pinDevice,
					PinMethod: pinMethod,
					PinPrompt: pinPrompt,
				},
			},
		})
	}

	res := r.network.Handle(context.Background(), req.GetRequestId(), req.GetOp(), req.GetArgsJson(), emit)
	r.sendDirect(&agentv1.AgentMessage{
		Body: &agentv1.AgentMessage_NetworkResult{
			NetworkResult: &agentv1.NetworkResult{
				RequestId:  req.GetRequestId(),
				Status:     res.Status,
				Error:      res.Error,
				ResultJson: res.ResultJSON,
			},
		},
	})
}

func boolJSON(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

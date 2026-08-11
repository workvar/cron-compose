package runtime

import (
	"context"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/croncompose/croncompose/agent/internal/connectors"
	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// connectorLoop discovers installed service managers on connect and every few minutes
// thereafter, shipping the inventory up the stream as a ConnectorEvent. Read-only in
// Phase A. See docs/connectors.md.
func (r *Runtime) connectorLoop(ctx context.Context) {
	r.discoverConnectors(ctx)
	t := time.NewTicker(5 * time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			r.discoverConnectors(ctx)
		}
	}
}

func (r *Runtime) discoverConnectors(ctx context.Context) {
	found := r.conns.Discover(ctx)
	if len(found) == 0 {
		return
	}
	out := make([]*agentv1.DiscoveredConnector, 0, len(found))
	for _, d := range found {
		out = append(out, discoveredToProto(d))
	}
	r.queue(&agentv1.AgentMessage{
		Body: &agentv1.AgentMessage_ConnectorEvent{ConnectorEvent: &agentv1.ConnectorEvent{
			Ts:         timestamppb.Now(),
			Connectors: out,
		}},
	})
	r.log.Info("connectors discovered", "count", len(out))
}

func discoveredToProto(d connectors.Discovered) *agentv1.DiscoveredConnector {
	inst := d.Instance
	res := make([]*agentv1.ConnectorResource, 0, len(d.Resources))
	for _, rsrc := range d.Resources {
		res = append(res, &agentv1.ConnectorResource{
			Type:       rsrc.Type,
			Ref:        rsrc.Ref,
			Name:       rsrc.Name,
			State:      rsrc.State,
			Checksum:   rsrc.Checksum,
			SizeBytes:  rsrc.SizeBytes,
			Attributes: rsrc.Attributes,
		})
	}
	return &agentv1.DiscoveredConnector{
		Kind:           inst.Kind,
		Instance:       inst.Instance,
		Version:        inst.Version,
		Status:         inst.Status,
		Manageable:     inst.Manageable,
		ConfigPaths:    inst.ConfigPaths,
		ObjectCount:    int32(inst.ObjectCount),
		ManagesConfig:  inst.Caps.ManagesConfig,
		ManagesObjects: inst.Caps.ManagesObjects,
		CanValidate:    inst.Caps.CanValidate,
		CanReload:      inst.Caps.CanReload,
		CanLifecycle:   inst.Caps.CanLifecycle,
		CanEdit:        inst.Caps.CanEdit,
		Detail:         inst.Detail,
		Resources:      res,
	}
}

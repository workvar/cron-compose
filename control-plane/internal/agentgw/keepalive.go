package agentgw

import (
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

// The agent pings every 20s (agent/internal/transport clientKeepalive) so a proxy
// cannot idle-cut the stream. gRPC's default server policy treats any ping closer
// than 5 minutes as abuse and answers the third one with GOAWAY too_many_pings,
// which dropped the agent about once a minute. Deploys kept running on the host,
// but the control plane had no stream, so the server flipped offline and Start/Stop
// failed with "agent offline" until the next dial.
const agentPingInterval = 20 * time.Second

// agentKeepaliveEnforcement is what the server will accept. MinTime must stay
// strictly below agentPingInterval.
var agentKeepaliveEnforcement = keepalive.EnforcementPolicy{
	MinTime:             agentPingInterval / 2,
	PermitWithoutStream: true,
}

func agentKeepaliveOptions() []grpc.ServerOption {
	return []grpc.ServerOption{
		grpc.KeepaliveEnforcementPolicy(agentKeepaliveEnforcement),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    agentPingInterval,
			Timeout: 10 * time.Second,
		}),
	}
}

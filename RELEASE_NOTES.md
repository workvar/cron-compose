# CronCompose v0.0.26

Fixes an agent ignoring `AGENT_GRPC_ADDR`. The agent always dialed the public address the
control plane gave it at enrollment, so a host that reaches the control plane another way,
such as loopback on the control plane host, could not be pointed there. That left an agent
running as root but unable to connect, so the page kept showing "Enable as root".

## Highlights

- **An explicit address wins.** When `AGENT_GRPC_ADDR` (or the old `CONTROL_PLANE_ADDR`)
  is set, the agent dials it instead of the address saved in `identity.json` at
  enrollment. With nothing set, behaviour is unchanged. The agent logs a line at startup
  when it uses the configured address over the enrolled one.
- **Clearer connection warning.** When the installer cannot reach the gRPC address it now
  says that a Cloudflare proxy or Tunnel TCP route does not carry raw gRPC on 443, and
  gives both fixes: loopback plus `AGENT_GRPC_SNI` on the control plane host, or
  `cloudflared access tcp` on other hosts.
- **Corrected docs for tunnels.** A Cloudflare Tunnel application route such as
  `tcp://localhost:9077` is not plain TCP on 443. The docs now say so, and show the
  `cloudflared access tcp` setup for remote agents. The bare-hostname support added in
  v0.0.25 is for plain TCP forwards only.
- **Tests and docs.** New tests for address precedence and the explicit-address flag, and
  a new "Explicit address beats the enrolled address" section in `docs/operations.md`.

## Upgrade

No database migration. Update the agent to this release on any host where you set
`AGENT_GRPC_ADDR` to something other than the public address.

Until then, on older agents, edit `control_plane_grpc_addr` in
`/var/lib/croncompose/identity.json` to the address you want, then run
`sudo systemctl restart croncompose-agent`.

# CronCompose v0.0.25

Lets an agent connect to a gRPC hostname with no port, for setups where the hostname
already maps to the gRPC port at its edge (a tunnel or proxy rule). This is the last
piece for agents on a host like `grpc.example.com` that could never dial an address
with the listener's own port in it.

## Highlights

- **Bare gRPC hostname.** `AGENT_GRPC_ADDR=grpc.example.com` now works and means port
  443. Before, the agent failed with `missing port in address`, so the address always
  had to carry a port that the edge might not expose. `host:port` still works and is
  needed when the endpoint is not on 443.
- **Enrollment keeps a bare host.** The control plane and the agent now accept a bare
  host as the address saved at enrollment, instead of discarding it as having no port.
  A listen address such as `:9077`, or text that is not a host, is still rejected.
- **Installer probes a bare host on 443.** The post-install reachability check no longer
  assumes a port, so it tests the same address the agent will dial.
- **Tests and docs.** New tests for address normalizing, dialing a bare host, the
  enrolled address and the installer probe. `docs/operations.md` has a new section,
  "GRPC hostname without a port".

## Upgrade

No database migration. To use a bare hostname, set `PUBLIC_GRPC_ADDR=grpc.example.com` on
the control plane and restart it, so the install command and the enrolled address carry
the bare host. Then update the agent to this release: agents older than v0.0.25 cannot
dial a bare host, and need the explicit `:443` instead.

Publish this release before upgrading the control plane, because the install command
fetches the installer from the latest release.

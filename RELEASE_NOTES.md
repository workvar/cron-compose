# CronCompose v0.0.27

Adds edge mode, so agents can connect through a Cloudflare proxy or tunnel with nothing
installed on the client. Until now an agent needed mutual TLS, which a TLS-terminating
edge cannot carry, so a proxied hostname failed with "certificate signed by unknown
authority".

## Highlights

- **Edge mode for agents.** The agent dials a hostname such as `grpc.example.com` on 443
  over ordinary TLS, verifies the edge's public certificate, and logs in with a
  per-server secret instead of a client certificate. No `cloudflared`, no forwarder, no
  extra step on the client.
- **Edge listener on the control plane.** Set `EDGE_GRPC_ADDR` to open a second gRPC
  listener for edge agents. It binds loopback only and refuses any other address, because
  it has no TLS of its own. The existing mutual TLS listener is unchanged and stays the
  default.
- **Per-server secrets.** Enrollment now creates a random secret for the server and
  returns it once. Only its SHA-256 is stored, it is compared in constant time, a server
  with no secret cannot log in, and the agent keeps its copy in `identity.json` (mode
  0600). Re-enrolling replaces it.
- **The scripts set it up.** `install.sh` and `update.sh` detect a gRPC hostname behind
  Cloudflare and write `PUBLIC_GRPC_MODE`, `EDGE_GRPC_ADDR` and a bare
  `PUBLIC_GRPC_ADDR` for you, so `.env` is never edited by hand. An existing
  `PUBLIC_GRPC_MODE` is left alone, and `CC_GRPC_MODE=edge|mtls` overrides the detection.
  They print the one Cloudflare step they cannot do.
- **One setting decides the mode.** `PUBLIC_GRPC_MODE=edge` on the control plane tells
  newly enrolled agents to use edge mode, and the agent records it. `AGENT_GRPC_MODE` on
  the agent overrides it. The install command is unchanged apart from the address, so
  nothing can disagree between the command and the control plane.
- **Tests and docs.** New tests cover the secret handling, the detection and `.env` changes in the scripts, the edge login path through a
  TLS-terminating proxy, the agent's TLS verification, mode selection and the database
  lookup. `docs/operations.md` has a new section, "Agents behind Cloudflare (edge mode)",
  and `.env.example` lists the new settings.

## Upgrade

This release adds a database migration (`0020`, one nullable column). It runs on start.

Run `update.sh` as usual. If your gRPC hostname is behind Cloudflare it turns on edge mode
by itself, restarts the stack, and prints the one step left in Cloudflare: turn on
Network, gRPC for the zone, and point the hostname's tunnel route at
`http://localhost:<edge port>` with HTTP2 connection enabled. A hostname has one route, so
this replaces a `tcp://localhost:9077` route on the same name. Then install or reinstall
agents with a new token. Agents enrolled before this release have no secret and must be
reinstalled to use edge mode. Hosts that are not behind Cloudflare are left on mutual TLS.

Docker Compose installs are not detected; see `docs/operations.md`.

What edge mode gives up: Cloudflare can read the traffic between the agent and the control
plane, and a stolen secret logs in as that server until it is replaced by re-enrolling.
Agents on the same host as the control plane should keep using `127.0.0.1:<port>` with mTLS.

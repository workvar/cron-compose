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
- **One setting decides the mode.** `PUBLIC_GRPC_MODE=edge` on the control plane tells
  newly enrolled agents to use edge mode, and the agent records it. `AGENT_GRPC_MODE` on
  the agent overrides it. The install command is unchanged apart from the address, so
  nothing can disagree between the command and the control plane.
- **Tests and docs.** New tests cover the secret handling, the edge login path through a
  TLS-terminating proxy, the agent's TLS verification, mode selection and the database
  lookup. `docs/operations.md` has a new section, "Agents behind Cloudflare (edge mode)",
  and `.env.example` lists the new settings.

## Upgrade

This release adds a database migration (`0020`, one nullable column). It runs on start.

Nothing changes until you opt in. To use edge mode:

1. In the control plane `.env`, set `EDGE_GRPC_ADDR=127.0.0.1:9078`, `PUBLIC_GRPC_MODE=edge`
   and `PUBLIC_GRPC_ADDR=grpc.example.com`, then restart it.
2. In Cloudflare, turn on Network, gRPC for the zone, and point the hostname's tunnel
   route at `http://localhost:9078` with HTTP2 connection enabled. A hostname has one
   route, so this replaces a `tcp://localhost:9077` route on the same name.
3. Install or reinstall agents with a new token. Agents enrolled before this release have
   no secret and must be reinstalled to use edge mode.

What edge mode gives up: Cloudflare can read the traffic between the agent and the control
plane, and a stolen secret logs in as that server until it is replaced by re-enrolling.
Agents on the same host as the control plane should keep using `127.0.0.1:<port>` with mTLS.

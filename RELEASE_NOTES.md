# CronCompose v0.0.28

`install.sh` and `update.sh` now turn on edge mode for a Cloudflare Tunnel whose gRPC
hostname is a TCP route. v0.0.27 only looked at that hostname, and a `tcp://` route does
not speak HTTPS, so the probe failed and the control plane stayed on mutual TLS. Agents
enrolled, showed Online, and never reported root, because the gRPC stream never connected.

## Highlights

- **The public URL is the fallback signal.** When `https://<grpc-host>/` does not answer,
  the scripts probe `PUBLIC_BASE_URL` (then `PUBLIC_HTTP_URL`). If that is Cloudflare,
  they write `PUBLIC_GRPC_MODE=edge`, `EDGE_GRPC_ADDR=127.0.0.1:<free port>` and a bare
  `PUBLIC_GRPC_ADDR`. Nothing in `.env` is edited by hand.
- **A saved `mtls` is corrected.** A previous run that missed Cloudflare left
  `PUBLIC_GRPC_MODE=mtls` or left the key unset. `update.sh` upgrades that when detection
  now says edge. `CC_GRPC_MODE=mtls` keeps mutual TLS for that run.
- **A gRPC host that answers as a normal server stays on mutual TLS,** even if the web UI
  is behind Cloudflare.

## Upgrade

Run `./update.sh` on the control plane. It rewrites `.env`, restarts the stack, and
prints the listener port. Then, in Cloudflare:

1. Turn on gRPC for the zone (Network, gRPC).
2. Edit the gRPC public hostname. Replace `tcp://localhost:<grpc port>` with service
   type HTTP, URL `http://localhost:<edge port>`, and enable HTTP2 connection.
3. Leave the web hostname pointed at the control plane HTTP port.

Reinstall each agent with a new enrollment token. Agents enrolled before edge mode have
no secret and keep trying mutual TLS. Root installs stay `AGENT_RUN_AS_ROOT=1`; once the
stream connects, the server page shows On (root).

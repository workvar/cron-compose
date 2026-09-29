# CronCompose v0.0.22

Choose root or non-root at install time (or switch to root later without waiting
on the live toggle), and the installer now handles a gRPC endpoint published on
its own hostname.

## Highlights

- **Two install commands when creating a server** — the "Add server" page now
  shows both a dedicated-user install command (recommended, default) and a
  root install command (`AGENT_RUN_AS_ROOT=1`), each with a pros/cons list so
  you can pick the right one for the box instead of defaulting to root out of
  convenience.
- **"Reinstall as root" on existing servers** — the server detail page can now
  issue a fresh enrollment token and hand you a root install command directly,
  without going through the "Agent root access" toggle. Useful as a fallback
  when the toggle can't settle because the agent hasn't reconnected yet.
- **Installer asks for a separate gRPC hostname** — `./install.sh` now has an
  "Agent gRPC public hostname" question (blank = same as the public URL).
  gRPC is raw TCP, not HTTP, so it's often fronted differently than the web
  app (for example its own Cloudflare Tunnel/Access TCP application).
  Answering it now correctly writes `PUBLIC_GRPC_ADDR` and extends
  `TLS_HOSTS`; previously neither was set from a separate gRPC hostname, so
  agents behind a hostname-split tunnel could never complete their TLS
  handshake. The prompt accepts either a bare hostname or a full URL (it
  strips the scheme either way, same as the "Public URL" question).

## Upgrade

No action needed for an existing install unless your gRPC endpoint lives on a
different hostname than your web app. If it does, add `PUBLIC_GRPC_ADDR` (its
`host:port`) and that host in `TLS_HOSTS` to your `.env` by hand, then restart
the control plane; a fresh `./install.sh` run (or `--advanced` re-run) now
asks for this directly.

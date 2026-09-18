# CronCompose v0.0.11

Passkeys for passwordless sign-in and step-up, plus a per-server **Agent root
access** switch that elevates or demotes the agent through a narrow sudo helper.
The terminal user picker hides OS accounts you cannot reach until root mode is
actually on.

## Highlights

- **Passkeys (WebAuthn)** — Enroll and remove keys under Settings → Security.
  Sign in with a passkey when `PUBLIC_BASE_URL` / `PUBLIC_HTTP_URL` can derive a
  relying party. Conditional mediation is preferred when the browser supports it;
  an explicit “Sign in with passkey” button remains. Password is never accepted
  as step-up for privileged actions.
- **Agent root access** — Admins and owners get a per-server switch. Toggling
  requires a passkey assertion (`POST /auth/passkey/step-up/begin` then
  `POST /servers/:id/agent-root`). Status shows Off / Enabling / On (waiting) /
  On (root) / Error from `agent_root_enabled` × `agent_euid_root`.
- **`agent-privctl`** — On elevate/demote the agent runs
  `sudo -n /usr/libexec/croncompose/agent-privctl elevate|demote`. Systemd gets a
  drop-in `User=root` (and `ProtectHome=false` while elevated so root terminals
  can reach user homes). Sudoers grants only those two argv forms (plus existing
  connector/Ports paths) — never `NOPASSWD: ALL`. pm2 hosts without systemd get a
  clear error instead of a silent no-op.
- **Terminal picker** — Unavailable OS users stay hidden until root mode is
  active (`agent_root_enabled && agent_euid_root`). No “needs root agent” tease.

## Upgrade notes

### Control plane (source install)

From Settings → Updates, click **Update** on this host. Or by hand:

```sh
cd cron-compose
git fetch --tags
git checkout --force v0.0.11
./update.sh --no-pull
```

This release applies migrations **0017** (WebAuthn credentials/challenges) and
**0018** (server agent-root columns). Ensure `PUBLIC_BASE_URL` (or
`PUBLIC_HTTP_URL`) matches the browser origin so passkeys work.

### Agent (Linux / macOS)

Protocol additions: Hello `euid_root` / `service_user`, and `AgentRootCommand`.
A stack update rebuilds and installs `agent-privctl` under
`/usr/libexec/croncompose/` and refreshes sudoers. Standalone agents:

```sh
curl -sSL https://github.com/workvar/cron-compose/releases/latest/download/install-agent.sh | \
  sudo TOKEN=<token> \
       CONTROL_PLANE_HTTP=https://<host>/api \
       CONTROL_PLANE_ADDR=<host>:9090 \
       bash
```

Root elevate requires systemd (not pm2 alone) and a working sudoers install for
the agent service user.

# Agent root access + passkeys

**Date:** 2026-09-18  
**Status:** Approved for planning (awaiting human review of this file)  
**Approach:** Privileged helper + WebAuthn in the control plane

## Summary

Operators need to open a web terminal as other OS users on a server. Today that
requires the agent process to already be root; the agent normally runs as a
dedicated non-root user. This design adds:

1. **Passkeys (WebAuthn)** for passwordless login and for step-up auth.
2. A **per-server “Agent root access”** switch (admin/owner) that, after a
   passkey assertion, tells that agent to restart as root via a narrow sudo
   helper — and to demote again when the switch is turned off.
3. Terminal UI that **hides** other users until root access is on and the agent
   reports euid 0.

## Decisions (from brainstorming)

| Topic | Choice |
|-------|--------|
| What “root access” means | Run the agent process as root |
| How elevation happens | Agent restarts via sudo helper (`agent-privctl`) |
| Scope | Per server |
| On disable | Automatically demote back to the install user |
| Passkeys | Passwordless login **and** step-up |
| Step-up without passkey | Toggle locked until the actor enrolls a passkey |

## 1. Passkeys (WebAuthn)

### Data model

```sql
create table webauthn_credentials (
  id              text primary key,
  user_id         text not null references users(id) on delete cascade,
  credential_id   bytea not null unique,
  public_key      bytea not null,
  attestation_type text not null default '',
  transport       text[] not null default '{}',
  sign_count      bigint not null default 0,
  name            text not null default 'Passkey',
  created_at      timestamptz not null default now(),
  last_used_at    timestamptz
);

create table webauthn_challenges (
  id          text primary key,
  user_id     text references users(id) on delete cascade, -- null for discoverable login
  purpose     text not null, -- enroll | login | step_up
  challenge   bytea not null,
  expires_at  timestamptz not null,
  created_at  timestamptz not null default now()
);
```

Relying Party ID and origin come from `PUBLIC_BASE_URL` / `PUBLIC_HTTP_URL`
(hostname only for RP ID; full origin for allowed origins).

### APIs

| Method | Path | Auth | Purpose |
|--------|------|------|---------|
| POST | `/auth/passkey/login/begin` | public | Issue login challenge |
| POST | `/auth/passkey/login/finish` | public | Verify assertion, set session cookie |
| POST | `/auth/passkey/register/begin` | session | Issue enroll challenge |
| POST | `/auth/passkey/register/finish` | session | Store credential |
| GET | `/auth/passkeys` | session | List current user’s passkeys |
| DELETE | `/auth/passkeys/:id` | session | Remove a passkey |
| GET | `/auth/config` | public | Add `passkey_login: true` when WebAuthn is configured |

Library: Go WebAuthn (e.g. `github.com/go-webauthn/webauthn`) on the control
plane; browser uses `navigator.credentials.create` / `get`.

### UI

- **Login:** “Sign in with passkey” beside password / OAuth. Prefer conditional
  mediation when the browser supports it.
- **Settings → Security:** enroll (name the key), list, remove. Any signed-in
  user manages **their own** passkeys.

Passkeys are per CronCompose user, not per server.

## 2. Step-up for agent root toggle

- Only `admin` / `owner`.
- Enabling **or** disabling requires a **fresh** WebAuthn assertion. A normal
  session cookie is not sufficient.
- If the actor has zero passkeys, the switch is disabled with a link to
  Settings → Security. Password is **not** accepted as a substitute.
- Challenge purpose `step_up`, TTL ~2 minutes, single use. Success does not
  extend or widen the session; it only authorizes that one mutation.

### API

```
POST /servers/:id/agent-root
Body: { "enabled": true|false, "credential": <WebAuthn assertion JSON> }
```

Optional two-step variant (begin challenge, then finish with assertion) is fine
if the single round-trip payload is awkward; UX must still feel like one confirm
+ biometric/PIN.

On success:

1. Audit `server.agent_root.enable` or `server.agent_root.disable`.
2. Persist `servers.agent_root_enabled` (and `agent_root_changed_at` /
   `agent_root_changed_by`).
3. Push `AgentRootCommand{enabled}` to the connected agent.

## 3. Per-server agent root lifecycle

### Schema (servers)

```sql
alter table servers
  add column agent_root_enabled boolean not null default false,
  add column agent_euid_root boolean not null default false,
  add column agent_root_changed_at timestamptz,
  add column agent_root_changed_by text references users(id),
  add column agent_service_user text; -- last known non-root service user from agent
```

`agent_euid_root` is updated from agent Hello/Heartbeat (or a small field on
Hello). Terminal “root mode active” means:

`agent_root_enabled AND agent_euid_root`.

### Proto

```protobuf
// Server -> agent
message AgentRootCommand {
  bool enabled = 1; // true = elevate, false = demote
}

// Agent -> control plane (on Hello and after elevate/demote attempts)
// Extend Hello or Heartbeat:
//   bool euid_root = N;
//   string service_user = N; // intended non-root user when demoted
```

Also add `AgentRootCommand` to `ServerMessage` oneof.

### Privileged helper

Binary installed as root-owned, e.g.
`/usr/libexec/croncompose/agent-privctl`, invoked only as:

```
<agent-user> ALL=(root) NOPASSWD: /usr/libexec/croncompose/agent-privctl elevate, /usr/libexec/croncompose/agent-privctl demote
```

Installer extends `install/lib/agent_sudoers.sh` with these two commands (still
not `NOPASSWD: ALL`). Existing connector/Ports grants remain.

**elevate**

1. Read install marker (supervisor + original user), e.g.
   `.run/agent-supervisor` = `systemd` | `pm2`, `.run/agent-service-user`.
2. **systemd:** write drop-in
   `…/croncompose-agent.service.d/root.conf` with `User=root`,
   `daemon-reload`, restart unit.
3. **pm2:** user-mode pm2 cannot host a root process in-place. Stop the
   `croncompose-agent` pm2 app and start/enable a systemd unit (or transient
   service) running the same binary/identity as root. If systemd is
   unavailable, fail with a clear error.

**demote**

1. Remove root drop-in / stop root unit.
2. Restore original service user and supervisor (restart pm2 app or systemd
   unit with `User=<original>`).

The running agent, on `AgentRootCommand`, execs
`sudo -n /usr/libexec/croncompose/agent-privctl elevate|demote` and reports
progress/errors on the stream (ephemeral message or log). After restart, Hello
updates `agent_euid_root`.

### Failure modes

| Situation | Behavior |
|-----------|----------|
| Flag on, agent offline | Flag stays; UI shows “waiting for agent” |
| Flag on, sudoers missing | Agent reports error; UI shows exact sudoers line; picker stays locked |
| Flag on, euid still non-root | Same as above |
| Flag off, demote fails | Flag stays off for UI hiding; surface error; agent may still be root until fixed |

## 4. Terminal UI

### Server detail (admin/owner)

- Panel or row: **Agent root access** switch.
- Off + no passkeys: switch disabled; CTA to enroll a passkey.
- Confirm modal → WebAuthn → API.
- Status line: Off / Enabling… / On (agent is root) / On (waiting for elevation) / Error.

### Terminal user picker

| State | Picker contents |
|-------|-----------------|
| Root mode inactive | Only “Agent’s own user” (and any account where `available` is true because it is already the agent user). Other OS users are **hidden**. |
| Root mode active | Full list; all selectable |

Copy on the terminal page no longer teases hidden users with “needs root agent”
while the switch is off. If the switch is on but elevation failed, show the
helper/sudoers error instead of a useless list.

### Settings → Security

Passkey management only. No per-server root switches here.

## 5. Security notes

- Root mode is the most privileged agent posture; require passkey step-up and
  audit every toggle.
- Helper allowlist is two argv forms only; no shell, no extra args.
- Demotion is mandatory on disable (product decision); failed demotion is an
  ops incident, not silent success.
- Docs: update `docs/terminal.md`, `docs/security.md`, `docs/operations.md`,
  and installer README for the new sudoers lines.
- Recommend `PUBLIC_BASE_URL` set correctly so WebAuthn RP ID matches the
  browser hostname.

## 6. Testing

- WebAuthn enroll / login / step-up (unit tests with mocked authenticator where
  possible; integration with a test RP).
- Agent root command: helper elevate/demote against a fake supervisor script.
- Terminal picker filtering given `(enabled, euid_root)` combinations.
- API: 403 without role; 403/409 without passkey; 401 on bad assertion.

## 7. Out of scope

- Making connectors or job `run_as_user` depend on this flag (jobs already use
  `osuser.Resolve`; once the agent is root they work — no extra UI in this
  spec).
- Passkey as mandatory 2FA on every login (passwordless is optional).
- Global “all agents as root” switch.

## Implementation order (for the later plan)

1. Migrations + WebAuthn store/API + Settings Security + login button.
2. Step-up + `POST /servers/:id/agent-root` + server UI switch (flag only, no
   agent yet) + terminal hide logic gated on flag ∧ euid.
3. Proto `AgentRootCommand` + agent handler + `agent-privctl` + sudoers.
4. Wire end-to-end elevate/demote + status reporting + docs.

# How it works

CronCompose has one operator-facing loop: **define a server → enroll an agent → write a job → it runs on the box → you watch logs**. Everything else (RBAC, secrets, connectors, terminal) rides the same control plane and the same outbound agent stream.

![Agent enroll, sync, and run](diagrams/agent-job-sequence.png)

Full gallery: [Diagrams](Diagrams.md) · Why agent-local scheduling: [ADR-001 — Agent-local scheduler (not central)](Design-Decisions.md#adr-001-agent-local-scheduler-not-central)

## 1. Sign in

The control plane upserts a seed admin from `SEED_ADMIN_EMAIL` / `SEED_ADMIN_PASSWORD` on boot (role `owner`). You sign in with email and password; the browser stores an HttpOnly session cookie (`cc_session`, HMAC-SHA256, 7-day TTL). Optional OIDC SSO can sit alongside password login. API callers use a bearer token instead of a cookie.

Roles, lowest to highest: **viewer → operator → admin → owner**. See [Security](Security.md).

## 2. Add a server

Creating a server in the UI (or `cc server add`) mints a **one-time enrollment token** (short TTL, stored only as a hash) and prints an install command. The server row starts as `pending` until an agent enrolls.

## 3. Enroll the agent

On the target machine you run the printed command (or `agent enroll --token=...`). The agent:

1. Generates an ed25519 keypair and a CSR.
2. POSTs them to `POST /api/v1/agents/enroll` with the token (this is the only REST call an agent ever makes).
3. Receives a signed client certificate and the control-plane CA.
4. Persists identity under `DATA_DIR` (`/var/lib/croncompose` by default).
5. From then on authenticates with **mTLS**. The token is never used again.

The server row flips to `online`. If the cert is compromised, revoke it in the UI; the agent must re-enroll with a fresh token.

Agents **only dial out**. The target needs no inbound CronCompose ports.

## 4. Connect and sync

`agent run` (or the systemd unit) opens a long-lived gRPC bidi stream over mTLS. On connect it sends `Hello` (version, OS, arch, last sync cursor). The control plane replies with `SyncJobs`: the full set or a delta of job definitions this agent should hold, with secrets already resolved (the stream is encrypted).

The agent writes those definitions into a local bbolt store and acks with `ConfigAck`. Editing a job in the UI pushes a delta immediately if the agent is connected; offline agents pick it up on next connect.

## 5. Write a job

In the UI you pick a target (one server, or a **label selector** such as `role=edge`), an interpreter (`bash` / `sh` / `python3` / `node`), a 5-field cron expression, an IANA timezone, and the script. Optional: timeout, concurrency policy, catch-up policy, working directory, `run_as_user`, CPU/memory limits, secret refs.

Saving creates `job` + immutable `job_version` 1. Later edits to the script insert a new version and bump `current_version_id`; every run records which version it executed, so history stays accurate.

Built-in **job templates** (seeded by migration 0008) can start the wizard; you can also save an existing job as a template.

## 6. Local scheduling and execution

The agent evaluates cron **on the machine**, in the job’s timezone.

When a job fires (or a `RunNow` arrives):

1. Generate a run ULID and send `RunStarted`.
2. Apply **concurrency**: `skip` (default), `allow`, or `queue`.
3. Write the script to a temp file, set env (plain env + decrypted secrets as env vars — never interpolated into the script file), drop to `run_as_user` if set.
4. Run under `timeout_seconds`. Optional `systemd-run` CPU/memory/process/IO limits; if systemd is not actually running, the agent degrades to unlimited with a note in the log rather than failing the job.
5. Stream stdout/stderr as `LogChunk`s.
6. On exit, send `RunFinished`. Timeout kills the process group (`timed_out`).

While disconnected, the agent keeps firing jobs and writes events to a local **outbox**. On reconnect it replays them in order. The control plane upserts runs by `id`, so replay is idempotent.

**Catch-up** after a long outage (evaluated on agent startup against last fire time):

| Policy | Behaviour |
|--------|-----------|
| `once` (default) | Run one catch-up no matter how many windows were missed |
| `all` | Run each missed occurrence, capped at 20 |
| `skip` | Do nothing |

## 7. Watch it run

The run page opens an SSE stream at `/api/v1/runs/:id/logs/stream`. The control plane first dumps already-persisted lines, then live `log` events, then a `done` event with the final status. No polling.

Logs in Postgres are **capped** per run (`RUN_LOG_MAX_BYTES`, default 5 MiB). The job itself is not truncated — only stored bytes. Live SSE subscribers still see everything. Optional retention prunes old logs/runs/audit/operations on a schedule (off until you set a window).

Failed runs can notify **webhook, Slack, or email** targets, scoped by server labels and outcome.

## 8. What else rides the same stream

| Feature | What happens |
|---------|----------------|
| **Connectors** | Agent discovers nginx / systemd / Docker / pm2 (more kinds designed, not all implemented). Portal can read status, edit config with backup+validate, and drive start/stop/reload. |
| **Web terminal** | Admin/owner opens a PTY in the browser. Keystrokes go control plane → gRPC → agent PTY. Bytes are ephemeral — not stored, not replayed. |
| **Agent self-update** | Off unless version + HTTPS URL + pinned SHA-256 are all set. Agent verifies the digest, swaps the binary atomically, exits so systemd/pm2 restarts it. |

## Day in the life (happy path)

```
Operator                    Control plane                 Agent
   |                              |                         |
   |  add server                  |                         |
   |  copy install command        |                         |
   |----------------------------->|  mint one-time token    |
   |                              |                         |
   |  run install on the Pi -------------------------------->|
   |                              |<---- enroll (REST) -----|
   |                              |---- client cert ------->|
   |                              |<==== mTLS stream =======|
   |                              |---- SyncJobs ---------->|
   |  write job + cron            |                         |
   |----------------------------->|---- SyncJobs (delta) -->|
   |                              |                         |  local cron fires
   |                              |<---- RunStarted --------|
   |  open run page (SSE)         |<---- LogChunk ----------|
   |<---- live logs --------------|<---- RunFinished -------|
```

That loop is the product. Architecture detail: [Architecture](Architecture.md). Wire protocol: [Agent Protocol](Agent-Protocol.md).

# Agent protocol

> Sequence diagram: [Agent enroll, sync, and run](Diagrams.md#agent-enroll-sync-and-run) · Run states: [Job run status lifecycle](Diagrams.md#job-run-status-lifecycle)



The agent is a single Go binary on each target. It dials out, holds its own job definitions, runs them on a local schedule, and reports back. Spec: [`docs/agent-protocol.md`](https://github.com/workvar/cron-compose/blob/main/docs/agent-protocol.md). Protobuf: [`proto/agent.proto`](https://github.com/workvar/cron-compose/blob/main/proto/agent.proto).

## Lifecycle

1. **Enroll** once with a one-time token, receive a client certificate.
2. **Connect** a long-lived gRPC bidirectional stream over mTLS.
3. **Sync** job definitions into a local store.
4. **Schedule** and run jobs locally, even while disconnected.
5. **Report** run status and logs, buffering and replaying on reconnect.

## Enrollment

1. Operator creates a server → one-time `enrollment_token` (short TTL, hash-only).
2. Installer / `agent enroll` sends a CSR (hostname, OS, arch, pubkey) to `POST /api/v1/agents/enroll`.
3. Control plane validates the token, binds the agent to the server row, signs a **client certificate**, records the cert fingerprint, marks the token used.
4. Agent stores key + cert with restrictive perms. From then on: mTLS only.

Revoke in the UI to clear the fingerprint and deny-list the cert.

## Transport

One gRPC bidi stream: `AgentStream(stream AgentMessage) returns (stream ServerMessage)`. The agent opens it and reconnects with backoff if it drops. One stream carries jobs, logs, connectors, terminal, and updates.

### Agent → control plane

| Message | Purpose |
|---------|---------|
| `Hello` | Version, OS, arch, capabilities, last sync cursor |
| `Heartbeat` | Liveness: timestamp, load, currently running run IDs |
| `ConfigAck` | Which job definitions/versions the agent now holds |
| `RunStarted` | run_id, job_id, job_version_id, trigger, started_at |
| `LogChunk` | run_id, stream (stdout/stderr), seq, data |
| `RunFinished` | status, exit_code, finished_at, duration_ms |
| Connector / terminal / update acks | See [Connectors](Connectors.md) and [Web Terminal](Web-Terminal.md) |

### Control plane → agent

| Message | Purpose |
|---------|---------|
| `SyncJobs` | Full set or delta of definitions, secrets resolved inline |
| `RunNow` | On-demand run |
| `CancelRun` | Cancel by run_id |
| `UpdateAgent` | Self-update to a pinned version |
| Connector commands / `TerminalInput` | Portal features on the same stream |

## Local store and scheduling

The control plane is the source of truth for definitions. The agent keeps a bbolt copy so it survives restarts and outages.

- On connect: `Hello` with cursor → `SyncJobs` → `ConfigAck`.
- Live edits push a delta to a connected agent; offline agents catch up on next connect.
- Secrets are resolved by the control plane, stored with restrictive perms, injected as env at exec — never into the script file.
- Cron is evaluated in the job’s IANA timezone (`robfig/cron`).
- Catch-up: `once` (default), `all` (cap 20), or `skip`.

## Buffering and replay

While disconnected, the agent keeps running jobs and writes `RunStarted` / `LogChunk` / `RunFinished` to a local outbox. On reconnect it replays in order. The control plane upserts runs by `id`, so replay never duplicates.

Terminal bytes are the exception: they are ephemeral and **not** put in the outbox, so a reconnect does not replay a shell session.

## Why this transport

Outbound-only (NAT friendly), one connection for both directions, typed protobuf, mTLS for identity and encryption. A JSON-over-WebSocket fallback was considered and is off the table unless a self-hoster reports that gRPC is too heavy.

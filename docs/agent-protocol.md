# Agent protocol

The agent is a single Go binary on each target server. It dials out to the control plane,
holds its own job definitions, runs them on a local schedule, and reports back. This doc
covers enrollment, the transport, job sync, execution, and log streaming.

## Lifecycle overview

1. **Enroll** once with a one-time token, receive a client certificate.
2. **Connect** a long-lived gRPC bidirectional stream over mTLS.
3. **Sync** job definitions from the control plane into a local store.
4. **Schedule** and run jobs locally, even while disconnected.
5. **Report** run status and logs up the stream, buffering and replaying on reconnect.

## 1. Enrollment

Inbound access to the server is never required. Flow:

1. In the UI, the operator creates a server, which generates a one-time
   `enrollment_token` (short TTL, stored only as a hash). The UI shows an install
   command containing the token and the control-plane address.
2. The operator runs the installer on the server. The agent calls the unary RPC
   `Enroll(token, hostname, os, arch, pubkey)` over TLS.
3. The control plane validates the token (unused, unexpired), binds the agent to the
   server row, issues a **client certificate** (or signs the agent's CSR), records the
   cert fingerprint on the server, and marks the token used.
4. The agent stores its key + cert locally (root-only file perms) and from then on
   authenticates with mTLS. The enrollment token is never needed again.

If a cert is compromised, the operator revokes it in the UI (clears the fingerprint and
adds it to a deny list); the agent must re-enroll with a fresh token.

## 2. Transport

A single **gRPC bidirectional stream** over mTLS: `AgentStream(stream AgentMessage) returns (stream ServerMessage)`.

The agent opens it after enrollment and keeps it open, reconnecting with backoff if it
drops. One stream carries both directions.

### Agent to control plane (`AgentMessage`)

| Message       | Purpose                                                            |
|---------------|-------------------------------------------------------------------|
| `Hello`       | Sent on connect: agent version, os, arch, capabilities, last synced job version cursor. |
| `Heartbeat`   | Periodic liveness: timestamp, load, list of currently running run IDs. |
| `ConfigAck`   | Acknowledges which job definitions/versions the agent now holds.   |
| `RunStarted`  | A run began: run_id, job_id, job_version_id, trigger, started_at.  |
| `LogChunk`    | Output: run_id, stream (stdout/stderr), seq, data.                 |
| `RunFinished` | A run ended: run_id, status, exit_code, finished_at, duration_ms.  |
| `DeployEvent` | Git deploy progress: run_id, kind (log/started/finished), data. Clone tokens must never appear in logs. |

### Control plane to agent (`ServerMessage`)

| Message      | Purpose                                                             |
|--------------|--------------------------------------------------------------------|
| `SyncJobs`   | Full set or delta of job definitions this agent should hold, with secrets resolved inline (the stream is encrypted). |
| `RunNow`     | On-demand run: job_id, job_version_id, a server-assigned run_id.    |
| `CancelRun`  | Cancel a running job by run_id.                                     |
| `UpdateAgent`| Request the agent self-update to a target version (later phase).    |
| `DeployCommand` | Clone + install on this host: op start/stdin/cancel. `clone_token` is never logged. After install, `process_manager` is `none`, `pm2` (ecosystem or `npm start`), `systemd` (user unit), or `docker` (`compose up -d`). |

### Why bidi gRPC

Outbound-only (NAT friendly), one connection for both directions, typed protobuf
contract, and mTLS for identity and encryption in one mechanism. A JSON-over-WebSocket
channel is the fallback if gRPC proves heavy for self-hosters; the message set above
maps cleanly onto it.

## 3. Job sync and the local store

The control plane is the source of truth for job *definitions*. The agent keeps a local
copy in an embedded store (SQLite or bbolt) so it survives restarts and outages.

- On connect, the agent sends `Hello` with its last known sync cursor. The control plane
  replies with `SyncJobs` containing any changes (new, edited, disabled, deleted jobs).
- The agent applies them to its local store and replies `ConfigAck`.
- When an operator edits a job in the UI, the control plane pushes a `SyncJobs` delta to
  the relevant connected agent immediately; offline agents pick it up on next connect.
- Secrets referenced by a job are resolved by the control plane and included in the
  synced definition. They are written to the local store with restrictive perms and only
  ever passed to the job as environment variables at exec time, never written to disk in
  the script itself.

## 4. Local scheduling and execution

The agent runs an internal cron scheduler (for example `robfig/cron`) over its local job
set. Each schedule is evaluated in the job's IANA timezone, so DST is handled on the
machine that runs the job.

When a job fires (or a `RunNow` arrives):

1. Generate a run_id (ULID) locally and send `RunStarted`.
2. Apply the **concurrency policy**:
   - `skip`: if a run of this job is already in progress, record a `skipped` run.
   - `allow`: start a new run alongside the existing one.
   - `queue`: wait for the in-progress run to finish, then start.
3. Write the script body to a temp file, set env (non-secret env + resolved secrets),
   set working dir, drop to `run_as_user` if configured.
4. Execute with the chosen interpreter under a context with the job's `timeout_seconds`.
5. Stream stdout and stderr as `LogChunk` messages, tagged by stream and seq.
6. On exit, send `RunFinished` with status, exit code, and duration. On timeout, kill
   the process group and mark `timed_out`. Honor `max_retries` with backoff.

### Catch-up after being offline

If the agent was offline across one or more scheduled times, the job's `catchup_policy`
decides: `once` (run a single catch-up), `all` (run each missed occurrence), or `skip`.
Default is `once`. This is evaluated by the agent on startup against its local store.

## 5. Buffering and replay

While disconnected, the agent keeps running jobs and writes `RunStarted`, `LogChunk`,
and `RunFinished` records to its local store. On reconnect it replays them in order. The
control plane upserts runs by `id`, so replay is idempotent and never creates duplicates.

This is what makes the "jobs keep running even if the control plane is down" guarantee
real rather than aspirational.

## Protobuf sketch

```proto
service AgentService {
  rpc Enroll(EnrollRequest) returns (EnrollResponse);     // unary, TLS, token auth
  rpc AgentStream(stream AgentMessage) returns (stream ServerMessage); // bidi, mTLS
}

message AgentMessage {
  oneof body {
    Hello       hello       = 1;
    Heartbeat   heartbeat   = 2;
    ConfigAck   config_ack  = 3;
    RunStarted  run_started = 4;
    LogChunk    log_chunk   = 5;
    RunFinished run_finished= 6;
  }
}

message ServerMessage {
  oneof body {
    SyncJobs    sync_jobs   = 1;
    RunNow      run_now     = 2;
    CancelRun   cancel_run  = 3;
    UpdateAgent update_agent= 4;
  }
}
```

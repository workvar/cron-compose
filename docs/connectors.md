# Connectors

Connectors let CronCompose see and manage the service managers that already run on a
target server: web servers and proxies (nginx, Apache, Caddy, Traefik, HAProxy), process
and container managers (pm2, systemd, Docker), the system cron tables, and the firewall
(ufw). The agent discovers what is installed, reads current configuration and live state,
and (with sufficient privilege) edits config and drives lifecycle actions, all from the
portal.

This is an additive feature. It reuses the existing outbound gRPC stream, the agent's
offline resilience, the audit log, and RBAC. No new inbound access to a server is ever
required, so the same Raspberry-Pi-behind-NAT story still holds.

## Goals

- Discover installed service managers on each server automatically.
- Show live status (running / stopped, version, object counts) and current configuration.
- Edit configuration safely from the GUI: validate before activating, back up before
  writing, roll back on failure.
- Drive lifecycle actions (start / stop / restart / reload / enable / disable) on units,
  containers, and processes.
- Keep the agent unprivileged by default; privileged actions go through a narrow,
  explicit, per-connector allowlist.
- Reuse the existing transport, audit log, and roles. Add no new daemons.

## Non-goals (for now)

- Installing or removing the underlying software. We manage what is already present.
- Cross-server templating or fleet rollout. One connector instance at a time; fleet
  rollout is a later phase, consistent with how jobs evolve (see [roadmap.md](roadmap.md)).
- A vendor-aware config IDE. The editor validates, diffs, and backs up; syntax checking is
  delegated to each tool's own validator (`nginx -t`, `haproxy -c`, and so on).

## Concepts

**Connector**: a typed integration with one service manager on one server, for example the
nginx install on `web-01`, or the pm2 daemon for user `deploy`. Identified by
`(server_id, kind, instance)`. Carries status, version, and a capability set.

**Resource**: a thing inside a connector. Two flavors, distinguished so the UI and agent
can treat them uniformly:

- `config_file`: editable text with a validator and a reload, for example
  `/etc/nginx/nginx.conf`, a Caddyfile, an haproxy.cfg, a crontab.
- `object`: a lifecycle unit with state and actions, for example a systemd unit, a Docker
  container, a pm2 process, a ufw rule.

**Snapshot**: an immutable backup of a config file's content, taken automatically before
any write and available for manual capture. Snapshots are what make every edit
reversible. They mirror the `job_versions` idea: never mutate in place, always record.

**Operation**: one read / validate / apply / lifecycle / rollback request. Persisted like
a `run`, with status and step-by-step output, so the GUI can show progress and history and
so the agent can report results even if the browser has gone away.

**Capabilities**: per-connector booleans reported at discovery: `manages_config`,
`manages_objects`, `can_validate`, `can_reload`, `can_lifecycle`, `can_edit`. The UI only
offers actions the agent says it can actually perform on that host.

## Supported connectors

| Kind | Detect | Config (read / edit) | Validate | Activate | Objects |
|------|--------|----------------------|----------|----------|---------|
| nginx | `nginx -v` | `/etc/nginx/**` (`nginx.conf`, `sites-enabled`, `conf.d`) | `nginx -t` | `nginx -s reload` or `systemctl reload nginx` | none |
| apache | `apachectl -v` / `httpd -v` | `/etc/apache2/**` (Debian) or `/etc/httpd/**` (RHEL) | `apachectl configtest` | `apachectl -k graceful` or `systemctl reload` | none |
| caddy | `caddy version` | `/etc/caddy/Caddyfile` or JSON config | `caddy validate --adapter caddyfile` | `caddy reload` or `systemctl reload caddy` | none |
| traefik | `traefik version` | static `/etc/traefik/traefik.{yml,toml}`, dynamic file-provider dir | parse + `traefik healthcheck` | dynamic file reloads itself; static needs `systemctl restart traefik` | routers / services (read-only) |
| haproxy | `haproxy -v` | `/etc/haproxy/haproxy.cfg` | `haproxy -c -f <cfg>` | `systemctl reload haproxy` (seamless `-sf`) | backends (read-only) |
| pm2 | `pm2 --version` (per user) | ecosystem file, `pm2 save` dump | n/a (JSON checked on start) | `pm2 reload`/`restart` | processes (`pm2 jlist`) |
| systemd | `systemctl --version` | unit files via `systemctl cat`; edit as drop-in override | `systemd-analyze verify <unit>` | `daemon-reload` then reload/restart | units (`list-units`/`list-unit-files`) |
| docker | `docker version -f json` | compose files (`docker compose ls`) | `docker compose config -q` | `docker compose up -d` / per-container restart | containers (`docker ps -a`) |
| cron | cron/crond present | `/etc/crontab`, `/etc/cron.d/*`, `crontab -l -u <user>` | parse 5/6-field rows | daemon auto-reloads on file change | entries (parsed rows) |
| ufw | `ufw version` | `/etc/ufw/*.rules` | `ufw --dry-run <rule>` | `ufw reload` | rules (`ufw status numbered`) |

Two connectors carry extra risk and get extra guardrails (see [Security](#security)):

- **ufw**: editing firewall rules over the network can lock you out. The agent refuses any
  operation that would drop the control path it is connected on, every change is preceded
  by `ufw --dry-run`, and `ufw disable` requires an explicit confirm flag.
- **cron** overlaps with CronCompose's own jobs. The connector shows raw system cron as it
  exists on disk. Each entry can optionally be imported as a managed CronCompose job (it
  then gains run history, logs, and the agent-local scheduler). Import is opt-in; the
  connector never silently rewrites the user's crontab.

## Data model

PostgreSQL, same conventions as [data-model.md](data-model.md): ULID text PKs,
`timestamptz`, `jsonb`. New migration `migrations/0005_connectors.sql`.

```sql
-- One service manager on one server. Cache refreshed by agent discovery.
create table connectors (
  id           text primary key,
  server_id    text not null references servers(id) on delete cascade,
  kind         text not null,                 -- nginx|apache|caddy|traefik|haproxy|pm2|systemd|docker|cron|ufw
  instance     text not null default '',      -- e.g. pm2 user; '' for singletons
  version      text,
  status       text not null default 'unknown', -- running|stopped|degraded|unknown
  manageable   boolean not null default false,  -- agent has privilege to act, not just read
  capabilities jsonb  not null default '{}',
  config_paths jsonb  not null default '[]',
  object_count integer not null default 0,
  detail       jsonb  not null default '{}',
  last_seen_at timestamptz,
  created_at   timestamptz not null default now(),
  unique (server_id, kind, instance)
);

-- Cached inventory of config files and lifecycle objects within a connector.
create table connector_resources (
  id           text primary key,
  connector_id text not null references connectors(id) on delete cascade,
  type         text not null,                 -- config_file|object
  ref          text not null,                 -- path | unit | container id | pm2 id | rule number
  name         text not null,
  state        text,                          -- active|running|stopped|enabled (objects); null for files
  checksum     text,                          -- config files
  size_bytes   integer,
  attributes   jsonb not null default '{}',
  updated_at   timestamptz not null default now(),
  unique (connector_id, type, ref)
);

-- Immutable config backups. Taken automatically before every apply; reversible.
create table connector_snapshots (
  id           text primary key,
  connector_id text not null references connectors(id) on delete cascade,
  ref          text not null,                 -- path of the config file
  content      text not null,
  checksum     text not null,
  reason       text not null,                 -- pre_apply|manual|scheduled|import
  created_by   text references users(id),
  created_at   timestamptz not null default now()
);
create index on connector_snapshots (connector_id, ref, created_at desc);

-- Action log and async op tracking. Mirrors runs.
create table connector_operations (
  id           text primary key,
  connector_id text not null references connectors(id) on delete cascade,
  server_id    text not null references servers(id) on delete cascade,
  op           text not null,                 -- discover|read|validate|apply|lifecycle|rollback
  action       text,                          -- start|stop|restart|reload|enable|disable (lifecycle)
  ref          text,                          -- target resource
  status       text not null default 'pending', -- pending|running|succeeded|failed|invalid|agent_offline|canceled
  dry_run      boolean not null default false,
  request      jsonb not null default '{}',
  result       jsonb not null default '{}',   -- steps, validator output, exit codes
  snapshot_id  text references connector_snapshots(id), -- backup taken, for rollback
  created_by   text references users(id),
  created_at   timestamptz not null default now(),
  started_at   timestamptz,
  finished_at  timestamptz
);
create index on connector_operations (connector_id, created_at desc);
```

`connectors` and `connector_resources` are a cache, refreshed by agent discovery, so the
list and detail views render instantly and survive a brief agent disconnect (the same
pattern as `servers.status` being updated by heartbeat). `connector_snapshots` and
`connector_operations` are durable history.

## Agent

New package `agent/internal/connectors/`, kept modular (one file per connector so each
stays small and independently testable):

```
agent/internal/connectors/
  provider.go     -- Provider interface + shared types (Resource, Status, ApplyResult, Step)
  registry.go     -- provider registry + capability/privilege detection at startup
  safety.go       -- backup, atomic write, validate-then-activate, auto-rollback wrapper
  privexec.go     -- privileged command runner (sudo -n allowlist), output capture
  nginx/nginx.go
  apache/apache.go
  caddy/caddy.go
  traefik/traefik.go
  haproxy/haproxy.go
  pm2/pm2.go
  systemd/systemd.go
  docker/docker.go
  cron/cron.go
  ufw/ufw.go
```

### Provider interface

Every connector implements one small interface. The shared `safety.go` wrapper, not the
individual providers, owns backup and rollback so the dangerous logic lives in one place.

```go
type Provider interface {
    Kind() string

    // Detect reports installed instances, version, paths, and capabilities.
    Detect(ctx context.Context) ([]Instance, error)

    // Status returns live state for one instance.
    Status(ctx context.Context, inst Instance) (Status, error)

    // ListResources returns config files and/or objects.
    ListResources(ctx context.Context, inst Instance) ([]Resource, error)

    // ReadConfig returns the current bytes and checksum of a config file.
    ReadConfig(ctx context.Context, ref string) (Content, error)

    // Validate checks proposed content without activating it.
    Validate(ctx context.Context, ref string, content []byte) (Check, error)

    // Write replaces config bytes atomically (temp file + rename). No reload.
    Write(ctx context.Context, ref string, content []byte) error

    // Activate reloads or restarts so new config takes effect.
    Activate(ctx context.Context, inst Instance) (Step, error)

    // Lifecycle drives an object: start|stop|restart|reload|enable|disable.
    Lifecycle(ctx context.Context, ref, action string) (Step, error)
}
```

### Safety pipeline (apply)

`apply` is the only write path. It runs as one ordered pipeline and records each step in
`ConnectorResult.steps`:

1. **Concurrency check**: reject if the on-disk checksum differs from `base_checksum` the
   editor started from (someone or something changed the file underneath you).
2. **Backup**: snapshot current content (returned to the control plane, stored in
   `connector_snapshots`, linked from the operation).
3. **Validate**: run the connector's validator against the proposed content in a temp
   location. Abort with status `invalid` if it fails; nothing on disk has changed yet.
4. **Write**: atomic write (temp file, `fsync`, `rename`), keep a `.cc.bak` alongside.
5. **Activate**: reload or restart.
6. **Health gate + auto-rollback**: re-check `Status`. If the service did not come back
   healthy, restore the snapshot, activate again, and report `failed` with the rollback
   noted. A bad config never leaves the service down.

`dry_run` runs steps 1 to 3 only and returns the diff and validator output.

### Privilege model

The agent stays unprivileged by default, consistent with [security.md](security.md).
Managing nginx, systemd, ufw, and writing under `/etc` needs elevation, so:

- The installer can write `/etc/sudoers.d/croncompose-agent` with an allowlist of the
  exact commands each enabled connector needs (for example `NOPASSWD: /usr/sbin/nginx -t`,
  `/bin/systemctl reload nginx`). Nothing else is grantable through it.
- Docker is reached by adding the agent user to the `docker` group rather than via sudo.
- pm2 actions run as the owning user (`sudo -u deploy pm2 ...`).
- At startup the agent probes which commands it can actually run and sets `manageable` and
  the capability flags per connector. The UI shows read-only connectors as "detected, not
  manageable" with a one-line hint on how to grant management. We never silently fail a
  button the user can see.

## Protocol extensions

The connector channel is request / response with correlation, which the current stream
does not have (commands today are fire-and-forget). Add three messages and a correlation
id. Field numbers continue from the existing `oneof`s in `proto/agent.proto`.

```proto
message ServerMessage {
  oneof body {
    SyncJobs         sync_jobs         = 1;
    RunNow           run_now           = 2;
    CancelRun        cancel_run        = 3;
    UpdateAgent      update_agent      = 4;
    ConnectorCommand connector_command = 5; // new
  }
}

message AgentMessage {
  oneof body {
    Hello           hello            = 1;
    Heartbeat       heartbeat        = 2;
    ConfigAck       config_ack       = 3;
    RunStarted      run_started      = 4;
    LogChunk        log_chunk        = 5;
    RunFinished     run_finished     = 6;
    ConnectorEvent  connector_event  = 7; // new, unsolicited discovery push
    ConnectorResult connector_result = 8; // new, reply to a ConnectorCommand
  }
}

message ConnectorCommand {
  string request_id     = 1;  // ULID, correlates the reply
  string op             = 2;  // discover|status|list|read|validate|apply|lifecycle|rollback
  string connector_kind = 3;  // empty for discover-all
  string connector_id   = 4;  // control-plane id for a targeted op
  string ref            = 5;  // path | unit | container | pm2 id | rule number
  string action         = 6;  // lifecycle: start|stop|restart|reload|enable|disable
  bytes  content        = 7;  // apply: proposed config bytes
  string base_checksum  = 8;  // optimistic concurrency
  bool   dry_run        = 9;
}

message ConnectorResult {
  string request_id = 1;
  string status     = 2;  // succeeded|failed|invalid|unauthorized|unsupported
  string message    = 3;  // validator output / human-readable detail
  bytes  content    = 4;  // read: config bytes
  string checksum   = 5;
  repeated ConnectorStep steps = 6; // apply pipeline, one per stage
  bytes  payload_json = 7;          // structured inventory/status
}

message ConnectorStep { string name = 1; bool ok = 2; string output = 3; int32 exit_code = 4; }

message ConnectorEvent {            // like a heartbeat, but for connector inventory
  repeated DiscoveredConnector connectors = 1;
  google.protobuf.Timestamp ts = 2;
}

message DiscoveredConnector {
  string kind = 1; string instance = 2; string version = 3; string status = 4;
  repeated string config_paths = 5;
  bool can_validate = 6; bool can_reload = 7; bool can_lifecycle = 8; bool can_edit = 9;
  int32 object_count = 10; string detail_json = 11;
}
```

The agent sends `ConnectorEvent` on connect and after any change it makes, so the cache
stays fresh. Targeted reads and writes use `ConnectorCommand` / `ConnectorResult`.

## Control plane

New module `control-plane/internal/connectors/` (`model.go`, `store.go`, `handler.go`,
`routes.go`, `dispatch.go`), following the servers and jobs modules.

Correlation is the one new mechanism. Add a small `PendingRequests` registry alongside the
existing `agentgw.Registry`:

```go
// pending.go in agentgw: request_id -> result channel.
type PendingRequests struct { /* mu + map[string]chan *agentv1.ConnectorResult */ }
func (p *PendingRequests) Begin(reqID string) chan *agentv1.ConnectorResult
func (p *PendingRequests) Resolve(res *agentv1.ConnectorResult) // called from handleAgentMessage
func (p *PendingRequests) Cancel(reqID string)
```

Flow for a targeted op (read / validate / apply / lifecycle / rollback):

1. REST handler creates a `connector_operations` row (`pending`), allocates a `request_id`,
   registers a channel via `Begin`, and sends a `ConnectorCommand` through
   `Registry.Send(serverID, ...)`.
2. If the agent is offline, `Registry.Send` returns `ErrAgentOffline`; the handler marks
   the op `agent_offline` and returns `409` (read) or `202` (a queued write, optional).
3. The handler waits on the channel with a per-op timeout (read/validate short, apply and
   lifecycle longer and configurable). `stream.go` `handleAgentMessage` gains a case for
   `ConnectorResult` that calls `Resolve`, and a case for `ConnectorEvent` that upserts the
   `connectors` and `connector_resources` cache.
4. The handler persists the result (steps, snapshot, status) onto the operation row and
   returns it. Long operations stream step output to the browser by reusing the existing
   `LogBroker`, keyed on `operation_id` instead of `run_id`.

Every write also writes an `audit_log` entry, as all mutating endpoints already do.

## REST API

Under `/api/v1`, conventions per [api.md](api.md) (JSON, cursor pagination, SSE for live
output, audit on mutations).

| Method | Path | Purpose |
|--------|------|---------|
| GET  | `/servers/:id/connectors` | List discovered connectors for a server (cache). |
| POST | `/servers/:id/connectors/refresh` | Ask the agent to re-run discovery. |
| GET  | `/connectors/:id` | Connector detail, status, and capabilities. |
| GET  | `/connectors/:id/resources` | List config files and objects (cache). |
| GET  | `/connectors/:id/config?ref=` | Read a config file live from the agent. |
| POST | `/connectors/:id/validate` | Validate proposed content. Body: `ref`, `content`. |
| POST | `/connectors/:id/apply` | Back up, validate, write, activate. Body: `ref`, `content`, `base_checksum`, `dry_run`. |
| POST | `/connectors/:id/actions` | Lifecycle action. Body: `ref`, `action`, `dry_run`. |
| GET  | `/connectors/:id/snapshots` | List config backups. |
| POST | `/connectors/:id/snapshots/:sid/rollback` | Restore a backup and reactivate. |
| GET  | `/connectors/:id/operations` | Operation history (paginated). |
| GET  | `/connectors/:id/operations/:opid` | One operation, with steps. |
| GET  | `/connectors/:id/operations/:opid/stream` | SSE live output for a running op. |
| POST | `/connectors/:id/import-cron` | Cron only: import a system entry as a managed job. |

## Web UI

A new top-level **Connectors** entry in the sidebar (`web/components/Sidebar.tsx` +
`icons.tsx`), plus a Connectors section on the server detail page. New routes and
components, kept small:

```
web/app/connectors/page.tsx            -- overview across all servers, grouped, status chips
web/app/connectors/[id]/page.tsx       -- detail (SSR fetch of connector + resources)
web/components/connectors/
  ConnectorCard.tsx       -- kind icon, status chip, version, object count
  ResourceTable.tsx       -- objects with state + action buttons; config files with edit
  ConfigEditor.tsx        -- client: textarea/code view, Validate, diff preview, Apply
  ActionButton.tsx        -- client: lifecycle action with a confirm modal
  SnapshotList.tsx        -- backups with restore
  OperationLog.tsx        -- recent operations, live output via SSE
web/lib/types.ts          -- add Connector, ConnectorResource, ConnectorSnapshot, Operation
web/lib/api.ts            -- already generic (apiGet/apiPost/...); no change needed
```

Interaction details that matter for full read/write:

- The config editor shows the current file with its checksum, lets the user edit, and
  gates **Apply** behind a **Validate** pass and a diff confirmation. Apply sends
  `base_checksum`; a mismatch surfaces a clear "changed on disk, reload before saving"
  message rather than clobbering.
- Lifecycle buttons (start / stop / restart / reload / enable / disable) confirm before
  firing, and dangerous ones (stop, disable, ufw disable) require a typed confirm.
- Read-only / not-manageable connectors render their controls disabled with the hint on
  how to grant management, so the UI never offers an action that will fail.
- The cron connector adds an **Import as job** affordance on each entry, linking into the
  existing job wizard with the schedule and command prefilled.

## Security

Connectors widen what the agent can touch, so the guardrails are part of the design, not a
follow-up.

- **Least privilege.** Agent stays unprivileged by default. Each privileged command is an
  explicit entry in a generated sudoers allowlist; Docker via group membership; pm2 as the
  owning user. Capabilities are probed at startup and reflected in the UI.
- **Validate before activate.** Every config change is checked by the tool's own validator
  in a temp location first. Invalid config never reaches the live path.
- **Backup and auto-rollback.** Every apply snapshots the prior content and restores it if
  the service fails its post-activate health check. Snapshots are durable and listable.
- **Optimistic concurrency.** Edits carry the checksum they were based on; stale writes are
  rejected.
- **Lockout protection.** The ufw connector refuses any rule change that would drop the
  control path the agent is connected on, dry-runs every change, and gates `disable`.
- **RBAC**, mapped onto the existing roles in [security.md](security.md):

  | Capability | viewer | operator | admin | owner |
  |------------|:------:|:--------:|:-----:|:-----:|
  | View status / config, list operations | yes | yes | yes | yes |
  | Lifecycle actions (start/stop/restart/reload) | no | yes | yes | yes |
  | Edit and apply config, enable/disable, rollback | no | no | yes | yes |
  | ufw / firewall changes | no | no | yes | yes |

- **Audit.** Every read of config and every mutation writes an `audit_log` entry
  (`connector.read`, `connector.apply`, `connector.reload`, ...); `connector_operations` is
  the detailed companion record with full step output.
- **Secrets.** Config files can contain credentials. Reads are gated by role and audited;
  the editor offers optional masking of common secret patterns in the diff view. Tighter
  integration with the secrets store (reference instead of inline) is a later phase.

## Phasing

Sized so each phase ships something usable, like the rest of the roadmap.

- **Phase A, read-only discovery.** Agent discovery + `ConnectorEvent`, the cache tables,
  `GET` endpoints, the overview and detail UI, live status and config viewing. No writes.
  Lowest risk, immediately useful, and exercises the whole pipe except mutation.
- **Phase B, lifecycle actions.** `ConnectorCommand` / `ConnectorResult` correlation, the
  operations table and log, start / stop / restart / reload / enable / disable with confirm
  modals, the sudoers allowlist in the installer.
- **Phase C, config read/write.** The full safety pipeline (backup, validate, atomic write,
  activate, auto-rollback), snapshots and restore, the config editor with validate and
  diff, optimistic concurrency.
- **Phase D, breadth and polish.** Remaining connectors, ufw lockout safeguards, cron
  import-as-job, optional secret masking, SSE streaming for long operations.

Suggested connector order within the phases: nginx and systemd first (most common, clean
validators and reloads), then Docker and pm2, then Caddy / HAProxy / Apache, then Traefik,
cron, and ufw.

## Open questions

1. **Privilege bootstrap.** Generate the sudoers allowlist automatically during install
   (smoothest), or document it and let the operator opt in per connector (safest)? Leaning
   opt-in per connector, shown in the UI when a connector is detected but not manageable.
2. **Config size and streaming.** Most configs are small; some Docker logs and pm2 dumps
   are not. Read inline up to a cap (say 1 MB) and stream larger payloads over the broker?
3. **Apply timeout policy.** A reload is quick; a Docker `up -d` that pulls images is not.
   Per-op default timeouts with an override, and the op continues server-side past a
   browser disconnect (it already does, via the operations table).
4. **Cron and native jobs.** How tightly to couple import: a one-time copy, or a live link
   that keeps the managed job and the crontab in sync? Start with a one-time copy.
5. **Multi-instance connectors.** pm2-per-user and multiple Docker contexts are real. The
   `instance` column covers it; confirm the discovery heuristics per connector.

## Implementation status

Phases A, B and C are implemented. Phase D (breadth) is not.

**Phase A, read-only discovery.** Providers for nginx, systemd, docker and pm2. The
agent sweeps on connect and every five minutes, and after every mutating command, and
pushes a `ConnectorEvent` through the durable outbox. The control plane upserts it into
the `connectors` / `connector_resources` cache (migration 0005).

**Phase B, lifecycle.** `POST /connectors/:id/actions` with `{action, ref}`, operator
role and above. start / stop / restart / reload / enable / disable, and nothing else:
`ValidAction` is a closed allowlist, checked before any string reaches a shell. Per
provider: systemd maps each verb to the same systemctl subcommand; docker has no reload
and maps enable/disable to the restart policy; pm2's disable is stop-then-save, because
that is what actually keeps a process down across a reboot.

**Phase C, config read and write.** Admin role only, for both reading and writing:
these files hold upstream addresses, internal ports and occasionally credentials.

- `GET  /connectors/:id/config?path=` read one file
- `POST /connectors/:id/config` apply, with `dry_run` for check-only
- `GET  /connectors/:id/snapshots` backup history
- `POST /connectors/:id/snapshots/:snapshotID/restore` roll back
- `GET  /connectors/:id/operations` what has been done to this connector

Every write goes through one pipeline in `agent/internal/connectors/safety.go`:

    backup -> precheck -> validate -> write -> validate-live -> activate -> health

`precheck` compares the caller's `base_checksum` against the file on disk, so an edit
based on a stale read is refused rather than silently overwriting somebody else's
change. `validate` checks the candidate without touching the live tree; for an nginx
include that can only be a structural check, which is why `validate-live` re-runs
`nginx -t` over the whole tree after the write. A failure at `validate-live`,
`activate` or `health` restores the pre-write bytes and reactivates, so a bad config
never outlives the request that caused it.

Confinement matters as much as the pipeline: `nginxProvider.owns` rejects any path
outside the config directories discovery reported. Without it the connector would be an
arbitrary file read/write primitive wearing an nginx label.

**Privilege.** The agent stays unprivileged by default. `privexec.go` escalates only
through `sudo -n`, only for a closed list of binaries (systemctl, nginx, tee, cp, mv,
install, ufw), and reports `unauthorized` rather than half-applying when the grant is
missing. Docker is never escalated: daemon access is a group membership, and escalating
to root to reach a socket the operator deliberately did not grant would be the wrong
default. Discovery reports capabilities honestly, so the UI disables what the agent
genuinely cannot do.

**Durability.** Migration 0006 adds `connector_operations` (one append-only row per
command, written before the command is sent so an unanswered one still leaves a trace)
and `connector_snapshots` (pre-apply bytes, pruned to the newest 20 per file). Results
travel back on the agent's EPHEMERAL direct-send path, not the durable outbox: a caller
is blocked on an HTTP request, and a replayed result after a reconnect would resolve
nothing.

**Phase D, breadth.** apache, caddy, traefik, haproxy, system cron and ufw are designed
above and have no providers yet. Adding one means a file in
`agent/internal/connectors/`, registration in `registry.go`, and nothing else: the
command dispatch, the safety pipeline, the REST surface and the UI are all generic.

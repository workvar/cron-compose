-- Durable history for connector writes.
--
-- 0005 added the discovery *cache* (connectors, connector_resources), which the agent
-- overwrites on every discovery sweep. These two tables are the opposite: append-only
-- records that must outlive the cache.
--
--   connector_operations  one row per lifecycle/config command, like `runs` for jobs.
--   connector_snapshots   the bytes of a config file before we overwrote it, like
--                         `job_versions` for scripts. This is what a rollback reads.

begin;

create table if not exists connector_operations (
  id            text primary key,
  connector_id  text not null references connectors(id) on delete cascade,
  server_id     text not null references servers(id) on delete cascade,
  request_id    text not null,                  -- ULID correlating the agent round trip
  op            text not null,                  -- discover|status|list|read|validate|apply|lifecycle|rollback
  action        text not null default '',       -- lifecycle: start|stop|restart|reload|enable|disable
  ref           text not null default '',       -- path | unit | container | pm2 id
  dry_run       boolean not null default false,
  status        text not null default 'pending',-- pending|succeeded|failed|invalid|unauthorized|unsupported|timeout|offline
  message       text not null default '',
  steps         jsonb not null default '[]'::jsonb,
  actor_user_id text references users(id) on delete set null,
  created_at    timestamptz not null default now(),
  finished_at   timestamptz
);

create index if not exists connector_operations_connector_idx
  on connector_operations (connector_id, created_at desc);
create index if not exists connector_operations_server_idx
  on connector_operations (server_id, created_at desc);
create unique index if not exists connector_operations_request_idx
  on connector_operations (request_id);

create table if not exists connector_snapshots (
  id            text primary key,
  connector_id  text not null references connectors(id) on delete cascade,
  ref           text not null,                  -- the config path this snapshot is of
  checksum      text not null default '',       -- sha256 of content, as the agent reported it
  content       bytea not null,
  size_bytes    bigint not null default 0,
  reason        text not null default 'pre_apply', -- pre_apply|manual
  operation_id  text references connector_operations(id) on delete set null,
  actor_user_id text references users(id) on delete set null,
  created_at    timestamptz not null default now()
);

create index if not exists connector_snapshots_connector_ref_idx
  on connector_snapshots (connector_id, ref, created_at desc);

commit;

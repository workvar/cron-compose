-- Connectors (Phase A): discover and view the service managers already running on a
-- server (nginx, systemd, docker, pm2, and so on). These two tables are a CACHE that the
-- agent refreshes via the ConnectorEvent message; the list and detail views read from
-- them, so they render instantly and survive a brief agent disconnect, the same way
-- servers.status is kept fresh by heartbeats.
--
-- Durable history (connector_snapshots, connector_operations) lands with the read/write
-- phases. See docs/connectors.md.

begin;

-- One service manager on one server. Identified by (server_id, kind, instance).
create table if not exists connectors (
  id           text primary key,
  server_id    text not null references servers(id) on delete cascade,
  kind         text not null,                  -- nginx|apache|caddy|traefik|haproxy|pm2|systemd|docker|cron|ufw
  instance     text not null default '',       -- e.g. pm2 user; '' for singletons
  version      text,
  status       text not null default 'unknown',-- running|stopped|degraded|unknown
  manageable   boolean not null default false, -- agent has privilege to act, not just read
  capabilities jsonb  not null default '{}'::jsonb,
  config_paths jsonb  not null default '[]'::jsonb,
  object_count integer not null default 0,
  detail       jsonb  not null default '{}'::jsonb,
  last_seen_at timestamptz,
  created_at   timestamptz not null default now(),
  unique (server_id, kind, instance)
);

create index if not exists connectors_server_idx on connectors (server_id);

-- Cached inventory of config files and lifecycle objects within a connector.
create table if not exists connector_resources (
  id           text primary key,
  connector_id text not null references connectors(id) on delete cascade,
  type         text not null,                  -- config_file|object
  ref          text not null,                  -- path | unit | container id | pm2 id | rule number
  name         text not null,
  state        text,                           -- active|running|stopped|enabled (objects); null for files
  checksum     text,                           -- config files
  size_bytes   bigint,
  attributes   jsonb not null default '{}'::jsonb,
  updated_at   timestamptz not null default now(),
  unique (connector_id, type, ref)
);

create index if not exists connector_resources_connector_idx on connector_resources (connector_id);

commit;

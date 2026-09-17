alter table servers
  add column if not exists agent_root_enabled boolean not null default false,
  add column if not exists agent_euid_root boolean not null default false,
  add column if not exists agent_root_changed_at timestamptz,
  add column if not exists agent_root_changed_by text references users(id),
  add column if not exists agent_service_user text;

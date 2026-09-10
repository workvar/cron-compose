-- Git connections, deploy projects, and per-language clone paths.
begin;

create table if not exists git_connections (
  id               text primary key,
  user_id          text not null references users(id) on delete cascade,
  provider         text not null, -- github | gitlab
  purpose          text not null, -- login | git
  provider_user_id text not null,
  login            text not null default '',
  email            text not null default '',
  token_enc        bytea not null,
  scopes           text not null default '',
  created_at       timestamptz not null default now(),
  updated_at       timestamptz not null default now(),
  unique (user_id, provider, purpose)
);

create index if not exists git_connections_user_idx on git_connections (user_id);

create table if not exists oauth_identities (
  provider         text not null,
  provider_user_id text not null,
  user_id          text not null references users(id) on delete cascade,
  login            text not null default '',
  email            text not null default '',
  created_at       timestamptz not null default now(),
  primary key (provider, provider_user_id)
);

create table if not exists deploy_settings (
  id             text primary key default 'default',
  language_paths jsonb not null default '{}',
  updated_at     timestamptz not null default now()
);

insert into deploy_settings (id, language_paths)
values ('default', '{}')
on conflict (id) do nothing;

create table if not exists deploy_projects (
  id                text primary key,
  name              text not null,
  provider          text not null,
  repo_full_name    text not null,
  repo_id           text not null default '',
  clone_url         text not null,
  default_branch    text not null default 'main',
  server_id         text not null references servers(id) on delete cascade,
  language          text not null default 'unknown',
  install_script    text not null default '',
  root_directory    text not null default '.',
  clone_path        text not null,
  port              integer not null default 0,
  process_manager    text not null default 'none', -- none | pm2 | systemd | docker
  env               jsonb not null default '{}',
  apps              jsonb not null default '[]',
  webhook_secret     text not null default '',
  deploy_token_hash text not null default '',
  write_spec        boolean not null default true,
  created_by        text references users(id),
  created_at        timestamptz not null default now(),
  updated_at        timestamptz not null default now()
);

create index if not exists deploy_projects_server_idx on deploy_projects (server_id);
create index if not exists deploy_projects_repo_idx on deploy_projects (provider, repo_full_name);

create table if not exists deploy_runs (
  id          text primary key,
  project_id  text not null references deploy_projects(id) on delete cascade,
  server_id   text not null references servers(id) on delete cascade,
  trigger     text not null, -- manual | webhook | api
  status      text not null default 'pending', -- pending | running | succeeded | failed | canceled | agent_offline
  branch      text not null default '',
  exit_code   integer,
  error       text,
  started_at  timestamptz,
  finished_at timestamptz,
  created_at  timestamptz not null default now()
);

create index if not exists deploy_runs_project_idx on deploy_runs (project_id, created_at desc);

create table if not exists deploy_run_logs (
  id         bigserial primary key,
  run_id     text not null references deploy_runs(id) on delete cascade,
  stream     text not null default 'stdout',
  seq        integer not null,
  chunk      text not null,
  ts         timestamptz not null default now(),
  unique (run_id, stream, seq)
);

commit;

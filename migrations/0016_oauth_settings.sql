-- Lets an admin configure GitHub/GitLab OAuth app credentials from the web UI
-- instead of editing .env and restarting the control plane. One row per provider;
-- the client secret is encrypted with the same cryptobox used for job secrets.
-- A row missing or with an empty client_id means "not configured here", and the
-- control plane falls back to the GITHUB_OAUTH_*/GITLAB_OAUTH_* env vars.
begin;

create table if not exists oauth_settings (
  provider          text primary key check (provider in ('github', 'gitlab')),
  client_id         text not null default '',
  client_secret_enc bytea,
  redirect_url      text not null default '',
  base_url          text not null default '',
  updated_at        timestamptz not null default now()
);

commit;

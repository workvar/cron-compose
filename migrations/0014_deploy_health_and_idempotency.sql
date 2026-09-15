-- Deploy hardening: opt-in health checks, a per-project health state, a bounded run
-- budget, and webhook delivery de-duplication.
begin;

-- Health checks are opt-in: an empty path means "a run succeeds when the install
-- script exits 0", which is the behavior every existing project already has. Setting
-- a path is what makes a deploy prove the app answers before it counts as successful,
-- and therefore what lets auto-rollback catch a clean install that crashes on boot.
alter table deploy_projects
  add column if not exists health_path text not null default '',
  add column if not exists health_port int not null default 0,
  add column if not exists health_timeout_seconds int not null default 60;

-- Whole-run budget handed to the agent. 0 means the agent's own default.
alter table deploy_projects
  add column if not exists deploy_timeout_seconds int not null default 0;

-- Where this project stands right now, as opposed to what its last run did:
--   unknown     never deployed, or deployed by a version that did not track this
--   healthy     last run succeeded
--   degraded    last run failed and nothing recovered it
--   rolled_back running the previous commit after an automatic rollback
alter table deploy_projects
  add column if not exists health_state text not null default 'unknown';

-- Webhook de-duplication. GitHub and GitLab both retry deliveries, and a repo that
-- has both the push webhook and the generated CI job will report the same commit
-- twice. Either way the second arrival must not start a second deploy.
create table if not exists deploy_webhook_deliveries (
  project_id  text not null,
  delivery_id text not null,
  commit_sha  text not null default '',
  created_at  timestamptz not null default now(),
  primary key (project_id, delivery_id)
);

create index if not exists deploy_webhook_deliveries_created_idx
  on deploy_webhook_deliveries (created_at);

create index if not exists deploy_runs_project_commit_idx
  on deploy_runs (project_id, commit_sha);

commit;

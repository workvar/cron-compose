-- Auto-rollback for failed deploys, and letting notification targets fire on deploy
-- runs (not just scheduled job runs).
begin;

-- Opt-in per project: never rewrite what's running on a server unless the operator
-- asked for it, matching the "opt-in, never silent" rule the cron connector follows.
alter table deploy_projects
  add column if not exists auto_rollback boolean not null default false;

-- Populated from the agent's "commit: <sha>" log line once a checkout finishes. Empty
-- until then (agent_offline runs, or runs against an older agent, never get one).
-- This is what a later failed run rolls back to: the most recent run on the same
-- project with a non-empty commit_sha and status = 'succeeded'.
alter table deploy_runs
  add column if not exists commit_sha text not null default '';

-- Which event families a target fires for. Empty existing rows keep firing on job runs
-- only, matching what they did before this column existed; a target has to opt into
-- "deploy" to also hear about deploy runs.
alter table notification_targets
  add column if not exists events jsonb not null default '["job_run"]'::jsonb;

commit;

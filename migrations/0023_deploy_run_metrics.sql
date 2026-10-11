-- Per-run deploy analytics reported by the agent on finish (phase timings, release size).
begin;

alter table deploy_runs
  add column if not exists metrics jsonb not null default '{}';

commit;

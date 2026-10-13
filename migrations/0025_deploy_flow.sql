-- Ordered deploy flow waves (install/build/cleanup/pre_start/start/health tasks).
begin;

alter table deploy_projects
  add column if not exists flow jsonb not null default '[]'::jsonb;

commit;

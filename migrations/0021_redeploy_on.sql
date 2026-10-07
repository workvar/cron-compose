-- Which Git events should auto-redeploy a project: branch push, tag push, and/or release.
begin;

alter table deploy_projects
  add column if not exists redeploy_on jsonb not null default '["branch"]';

commit;

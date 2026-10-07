-- Deploy as a chosen OS account (pi, root, …). Empty means the agent's own user.
-- Paths and install scripts then run under that account's identity and login PATH
-- (so nvm/fnm installs under ~pi are visible), instead of the agent service user.
begin;

alter table deploy_projects
  add column if not exists run_as_user text not null default '';

commit;

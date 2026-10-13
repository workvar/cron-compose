-- When a deploy start-wave fails, stop the whole run (default) or continue to the next wave.
begin;

alter table deploy_projects
  add column if not exists on_wave_failure text not null default 'stop';

commit;

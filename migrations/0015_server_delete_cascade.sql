-- Deleting a server currently fails with a foreign key violation as soon as it has
-- any run history, because runs.server_id was left without an ON DELETE behavior
-- back in 0001_init.sql (every other server_id reference already cascades: jobs,
-- connector_operations, port_labels, deploy_projects, deploy_runs). This blocks the
-- new "delete server" action in the web UI for any server that has actually run a
-- job. Runs are a historical log tied to the server they ran on, so cascade them
-- the same way the other tables do.
begin;

alter table runs drop constraint if exists runs_server_id_fkey;
alter table runs
  add constraint runs_server_id_fkey
  foreign key (server_id) references servers(id) on delete cascade;

commit;

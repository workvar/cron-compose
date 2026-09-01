-- Indexes the retention pruner needs.
--
-- Without these, every hourly sweep is a sequential scan over the two largest tables in
-- the database. The pruner is designed to be unnoticeable; these are what make it so.

begin;

create index if not exists runs_created_at_idx on runs (created_at);
create index if not exists audit_log_ts_idx on audit_log (ts);

commit;

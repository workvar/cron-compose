-- Two more resource limits, both enforced by the agent through systemd-run.
--
-- tasks_max is the one that matters most in practice: CPU and memory caps slow a
-- runaway job down, a process cap is what stops a fork bomb from taking the box with
-- it. io_weight is relative rather than absolute, so a backup job can be told to yield
-- to everything else without being given a number nobody can calibrate.

begin;

alter table jobs add column if not exists tasks_max integer not null default 0;
alter table jobs add column if not exists io_weight integer not null default 0;

commit;

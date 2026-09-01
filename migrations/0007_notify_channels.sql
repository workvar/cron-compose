-- Notification channels and scoping.
--
-- 0003 shipped one channel (a JSON POST to a URL) that fired for every failed run on
-- every server. That is the right default for one machine and useless for a fleet: the
-- person who cares about the database box does not want a page about a Raspberry Pi.
--
-- config holds channel-specific settings, so adding a channel does not add columns.
-- server_labels scopes a target to part of the fleet using the same label containment
-- the job targeting already uses.

begin;

alter table notification_targets
  add column if not exists config jsonb not null default '{}'::jsonb;

-- Empty selector means "every server", which is what existing rows should keep doing.
alter table notification_targets
  add column if not exists server_labels jsonb not null default '{}'::jsonb;

-- Which run outcomes fire this target. Empty means every non-success status, matching
-- the previous behaviour.
alter table notification_targets
  add column if not exists on_statuses jsonb not null default '[]'::jsonb;

-- url is meaningless for the email channel, so it stops being required.
alter table notification_targets alter column url drop not null;
alter table notification_targets alter column url set default '';
update notification_targets set url = '' where url is null;

alter table notification_targets
  add column if not exists last_error text not null default '';
alter table notification_targets
  add column if not exists last_fired_at timestamptz;

create index if not exists notification_targets_enabled_idx
  on notification_targets (enabled);

commit;

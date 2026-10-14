-- Browser Web Push: per-user subscriptions and a singleton VAPID keypair.
-- Lets the control plane wake browsers that have granted notification permission
-- even when the CronCompose tab is closed.

begin;

create table if not exists push_vapid (
  id int primary key check (id = 1),
  public_key text not null,
  private_key text not null,
  subject text not null default 'mailto:croncompose@localhost',
  created_at timestamptz not null default now()
);

create table if not exists push_subscriptions (
  id text primary key,
  user_id text not null references users(id) on delete cascade,
  endpoint text not null,
  p256dh text not null,
  auth text not null,
  user_agent text not null default '',
  -- Which events this browser wants. Known values:
  -- deploy_started, deploy_succeeded, deploy_failed, job_failed, update_available
  events jsonb not null default '["deploy_started","deploy_succeeded","deploy_failed","job_failed","update_available"]'::jsonb,
  created_at timestamptz not null default now(),
  last_seen_at timestamptz not null default now(),
  unique (endpoint)
);

create index if not exists push_subscriptions_user_idx
  on push_subscriptions (user_id);

create index if not exists push_subscriptions_events_idx
  on push_subscriptions using gin (events);

commit;

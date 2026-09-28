-- Persist OAuth refresh tokens so git connections can renew access tokens
-- without forcing the operator to reconnect.
alter table git_connections
  add column if not exists refresh_token_enc bytea,
  add column if not exists token_expires_at timestamptz;

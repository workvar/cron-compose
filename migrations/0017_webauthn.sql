begin;

create table webauthn_credentials (
  id              text primary key,
  user_id         text not null references users(id) on delete cascade,
  credential_id   bytea not null unique,
  public_key      bytea not null,
  attestation_type text not null default '',
  transport       text[] not null default '{}',
  sign_count      bigint not null default 0,
  name            text not null default 'Passkey',
  created_at      timestamptz not null default now(),
  last_used_at    timestamptz
);

create table webauthn_challenges (
  id          text primary key,
  user_id     text references users(id) on delete cascade,
  purpose     text not null,
  challenge   bytea not null,
  expires_at  timestamptz not null,
  created_at  timestamptz not null default now()
);

commit;

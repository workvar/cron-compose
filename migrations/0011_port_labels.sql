-- Operator-authored names for listening sockets, so a port can be remembered as
-- "Next.js UI" after the process name or PID changes. Keyed by the bind tuple on
-- a server, not by PID.

begin;

create table if not exists port_labels (
  id         text primary key,
  server_id  text not null references servers(id) on delete cascade,
  proto      text not null default 'tcp',
  address    text not null,
  port       integer not null,
  label      text not null,
  updated_at timestamptz not null default now(),
  unique (server_id, proto, address, port)
);

create index if not exists port_labels_server_idx on port_labels (server_id);

commit;

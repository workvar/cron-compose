# CronCompose v0.0.41

Agents that the control plane rejects with `unknown cert` reconnect on their
own. You do not need a shell on the host.

## Highlights

- **Stale agent certificates rebind** — Enrollment writes the server id into
  the client certificate, and TLS already checks that this control plane signed
  it. When the fingerprint stored on the server row no longer matches, the next
  connection saves the fingerprint the agent is presenting and accepts the
  stream. Offline hosts come back on their usual retry, about 30 seconds after
  the control plane restarts.
- **Lookup failures stay distinct** — A database error during auth is reported
  as `cert lookup failed`, so a canceled query is a separate error from a
  certificate the control plane does not recognize.

## Upgrade

Update the **control plane** only. Agents and web are unchanged. No new
migration.

A certificate this control plane signed earlier for the same server is
accepted, and it becomes the stored fingerprint. A host enrolled against a
different CA needs `agent enroll` on that machine.

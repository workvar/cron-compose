-- Per-server secret for agents that connect through an edge that ends TLS (for
-- example a Cloudflare proxy), where a client certificate cannot be presented.
-- Only the SHA-256 of the secret is stored. Null means the agent has none and must
-- re-enroll before it can use the edge listener.
alter table servers add column if not exists agent_secret_hash text;

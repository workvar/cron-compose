# Security

> Related ADRs: [ADR-002 — Outbound mTLS gRPC (not inbound SSH / WebSocket-first)](Design-Decisions.md#adr-002-outbound-mtls-grpc-not-inbound-ssh-websocket-first) · [ADR-005 — Secrets: envelope encryption, write-only API, inject at sync](Design-Decisions.md#adr-005-secrets-envelope-encryption-write-only-api-inject-at-sync) · [ADR-008 — Agent root is explicit and narrow](Design-Decisions.md#adr-008-agent-root-is-explicit-and-narrow)

CronCompose runs arbitrary shell scripts on the operator’s servers. Security is core, not an afterthought.

## User authentication

- Email + password, hashed (Argon2id / bcrypt). Session cookie `cc_session`: HMAC-SHA256, HttpOnly, SameSite=Lax, 7-day TTL. In production serve over TLS so `Secure` applies.
- API tokens: bearer, stored only as a hash, plaintext shown once at creation, revocable.
- Optional **OIDC SSO**: auto-provisions users by email with `OIDC_DEFAULT_ROLE` (default `viewer`). Password login stays available. See [Operations](Operations.md).

Open endpoints (no user session): `/healthz`, `/api/v1/auth/login`, `/api/v1/agents/enroll` (one-time token of its own).

## Authorization (RBAC)

| Role | Can |
|------|-----|
| **viewer** | Read jobs, runs, logs, connector status |
| **operator** | Create/edit/run jobs, connector lifecycle actions |
| **admin** | Manage servers, secrets, tokens, connector config, notification targets |
| **owner** | Everything, including user management |

Comparison is numeric rank: a route gated at `operator` admits operator/admin/owner.

Typical gating:

- All authenticated GETs under `/api/v1` require at least `viewer`.
- Job mutations and run-now require `operator`.
- Server mutations (create, edit, delete, issue token) require `admin`.
- Web terminal is **admin/owner only**.
- Secret *values* are never returned by any endpoint, regardless of role.

## Agent trust

Highest-risk boundary: an agent runs whatever scripts the control plane sends.

- **Enrollment** uses a one-time, short-TTL token, stored only as a hash, exchanged exactly once for a client certificate.
- **Steady state** is **mTLS**. The control plane verifies the client cert (`RequireAndVerifyClientCert`); the agent verifies the server cert. `AgentStream` authenticates by SHA-256 fingerprint of the peer cert against `servers.cert_fingerprint`.
- **Revocation** clears the fingerprint and deny-lists it; the agent must re-enroll.
- **Direction**: agents only ever dial out. No CronCompose ports need to be open on the target.
- Run the agent as a dedicated unprivileged system user, not root, unless a specific job’s `run_as_user` requires it.

The control plane bootstraps a self-signed CA + server cert under `TLS_DIR` on first start. Production can overwrite those files with a real PKI; the process loads whatever is on disk. **Persist `TLS_DIR`.** Regenerating the CA orphans every enrolled agent.

## Secrets

- Encrypted at rest with **AES-256-GCM** envelope encryption. `SECRETS_MASTER_KEY` is a 32-byte hex key (64 hex chars). Back it up offline; losing it makes every stored secret unrecoverable.
- Decrypt only at sync time; send over the already-encrypted mTLS stream.
- On the agent, secrets have restrictive file perms and are passed as environment variables at exec — never interpolated into the script body, never written to logs (known values are scrubbed).
- API is write-only for values.

## Execution bounds

Each run:

- Runs as `run_as_user` (default: the agent’s unprivileged user), in `working_dir`.
- Bounded by `timeout_seconds`; the whole process group is killed on timeout.
- Honors `concurrency_policy` (`skip` / `allow` / `queue`).
- Optional systemd-run CPU/memory/process/IO limits.

Honest caveat: a job is arbitrary code on the user’s own machine. CronCompose limits *who* can define jobs (RBAC), proves *which* control plane and agent are talking (mTLS), and bounds *how* a job runs. It does not fully sandbox hostile code.

## Audit logging

Mutating actions (create/edit/delete server, job, secret, token; manual run; cancel; enroll; revoke; terminal open/close; connector ops) write `audit_log` with actor, action, target, and metadata.

## Two TLS surfaces (do not mix them)

| Surface | How to terminate |
|---------|------------------|
| Browser / REST (HTTP port) | Reverse proxy or load balancer **in front** of `HTTP_ADDR`. Control plane speaks cleartext HTTP. |
| Agents (gRPC port) | **End-to-end mTLS.** L4/TCP passthrough only. Never HTTP-terminate. |

## Threat model (brief)

| Threat | Mitigation |
|--------|------------|
| Stolen enrollment token | One-time use, short TTL, hash-only storage |
| Compromised agent cert | Revoke + deny-list, force re-enroll |
| Eavesdropping on agent channel | mTLS on the gRPC stream |
| Secret exposure at rest | AES-GCM; write-only API |
| Secret exposure in logs | Scrubbing; secrets via env |
| Privilege escalation via jobs | Unprivileged default; explicit `run_as_user` |
| Malicious insider editing jobs | RBAC + audit log |
| Inbound attack on home servers | Agents dial out only |
| Runaway job | Timeout, concurrency, optional cgroups |
| Unverified agent self-update | Updates require a pinned SHA-256; no unverified path |

## Production hardening checklist

- [ ] `SESSION_SECRET` and `SECRETS_MASTER_KEY` are real random 32-byte hex, not the dev defaults. Master key backed up offline.
- [ ] TLS in front of HTTP; gRPC on TCP passthrough.
- [ ] `SEED_ADMIN_*` strategy: either keep them as the managed source of truth, or unset after first login so UI-managed passwords persist (they re-apply on every boot).
- [ ] `/metrics` firewalled, or gated with `METRICS_TOKEN`.
- [ ] Persistent volumes for Postgres and `TLS_DIR`, both backed up.
- [ ] Agents run as a non-root system user with the packaged systemd unit.
- [ ] `LOG_LEVEL=info` (or `warn`) in production.

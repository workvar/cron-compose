# Security

CronCompose runs arbitrary shell scripts on the user's servers, so security is core, not
an afterthought. This doc covers user auth, RBAC, agent trust, secrets, the execution
sandbox, and a short threat model.

## User authentication

- MVP: email + password with a strong hash (Argon2id or bcrypt), session cookie
  (HttpOnly, Secure, SameSite=Lax).
- External/API access: bearer `api_token`, stored only as a hash, shown in plaintext
  once at creation, scoped and revocable.
- Later: SSO via OIDC, optional TOTP MFA.

## Authorization (RBAC)

Roles, from most to least privileged:

- **owner** — full control incl. user management and billing/settings.
- **admin** — manage servers, jobs, secrets, tokens; cannot manage owners.
- **operator** — create/edit/run jobs and read logs; cannot manage secrets or servers.
- **viewer** — read-only: see jobs, runs, and logs.

Every mutating API call checks the caller's role. Secret *values* are never returned by
any endpoint regardless of role.

## Agent trust

This is the highest-risk boundary: an agent runs whatever scripts the control plane sends.

- **Enrollment** uses a one-time, short-TTL token, stored only as a hash. It is exchanged
  exactly once for a client certificate, then is useless.
- **Steady state** uses **mTLS**. The control plane verifies the agent's client cert; the
  agent verifies the control plane's server cert. The cert fingerprint is bound to the
  server row at enrollment.
- **Revocation**: an operator can revoke an agent in the UI, which clears the bound
  fingerprint and deny-lists it. The agent must re-enroll with a fresh token.
- **Direction**: agents only ever dial out. The control plane needs no inbound path to
  any target server, which shrinks the attack surface on the server side to zero open
  ports for CronCompose.
- The agent should run as a dedicated unprivileged system user, not root, unless a
  specific job's `run_as_user` requires elevation (and that should be rare and explicit).
- **Agent root access** is a per-server exception: after passkey step-up, the agent
  restarts as root via `/usr/libexec/croncompose/agent-privctl elevate|demote`. Sudoers
  grants only those two argv forms (plus existing connector/Ports paths) — never
  `NOPASSWD: ALL`. Disabling the switch must demote; a failed demotion is an ops
  incident. Passkeys use the hostname of `PUBLIC_BASE_URL` as the WebAuthn RP ID.

## Secrets

- Stored encrypted at rest with **AES-256-GCM**. Use envelope encryption: a master key
  (from a KMS, or an operator-provided key for self-hosting) wraps per-secret data keys.
- The control plane decrypts a secret only at the moment it syncs a job definition to an
  agent, and sends it over the already-encrypted mTLS stream.
- On the agent, secrets live in the local store with root-only file perms and are passed
  to jobs only as environment variables at exec time. They are never interpolated into
  the script body on disk and never written to logs.
- Secret *values* are write-only through the API; no endpoint returns them.
- Log capture should optionally scrub known secret values from output before storage.

## Execution sandbox

Each run on the agent:

- Runs as the configured `run_as_user` (default: the agent's unprivileged user), in the
  configured `working_dir`.
- Is bounded by `timeout_seconds`; on timeout the whole process group is killed.
- Honors a `concurrency_policy` (skip / allow / queue) to prevent runaway overlap.
- Later: resource limits via `systemd-run` scopes or cgroups (CPU, memory), and optional
  network/file isolation. Tracked in the roadmap.

The honest caveat: a job is arbitrary code on the user's own machine. CronCompose limits
*who* can define jobs (RBAC), proves *which* control plane and agent are talking (mTLS),
and bounds *how* a job runs (user, timeout, concurrency). It does not claim to fully
sandbox hostile code; that is the operator's responsibility via `run_as_user` and OS
controls.

## Audit logging

Every mutating action (create/edit/delete server, job, secret, token; manual run; cancel;
enroll; revoke) writes an `audit_log` entry with actor, action, target, and metadata.
This is the record for "who scheduled this job" and "who ran this at 3am".

## Threat model (brief)

| Threat                                   | Mitigation                                    |
|------------------------------------------|-----------------------------------------------|
| Stolen enrollment token                  | One-time use, short TTL, hash-only storage.   |
| Compromised agent cert                   | Revoke + deny-list, force re-enroll.          |
| Eavesdropping on agent channel           | mTLS on the gRPC stream.                       |
| Secret exposure at rest                  | AES-GCM envelope encryption; write-only API.  |
| Secret exposure in logs                  | Optional scrubbing; secrets passed via env.   |
| Privilege escalation via jobs            | Run as unprivileged user by default; explicit `run_as_user`. |
| Malicious insider editing jobs           | RBAC + full audit log.                         |
| Inbound attack on home servers           | Agents dial out only; no open ports needed.    |
| Runaway job exhausting a server          | Timeout, concurrency policy, later cgroups.    |

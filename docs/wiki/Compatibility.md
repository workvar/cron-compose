# Compatibility

## Platforms

| Component | Linux | macOS | Windows |
|-----------|-------|-------|---------|
| Control plane (from-source installer) | Yes | Yes | Yes (`install.ps1`) — control plane only |
| Control plane (Docker Compose) | Yes | Yes (Docker Desktop) | Yes (Docker Desktop) |
| Agent (run jobs, connectors, terminal) | Yes | Yes | **No** — Unix shell and process APIs |
| `cc` CLI | Yes | Yes | Should build; not a first-class target |
| Web UI (browser) | Any modern browser | Any modern browser | Any modern browser |

Agents are the job runners. A Windows box can host the control plane; the machines that execute cron jobs must be Linux or macOS.

## CPU architectures

| Artifact | Architectures |
|----------|----------------|
| Agent static binaries (GitHub Releases) | `linux/amd64`, `linux/arm64` |
| Agent `.deb` / `.apk` | amd64, arm64 |
| Control plane / CLI from source | Whatever Go 1.25 supports on the build host (amd64, arm64, including Raspberry Pi) |

Raspberry Pi (64-bit OS) is a first-class agent and control-plane target.

## Runtime versions

| Dependency | Version |
|------------|---------|
| Go | **1.25+** (CI uses 1.25) |
| Node.js | **20+** (CI uses 20; UI is Next.js 16 / React 19) |
| PostgreSQL | **16** (Compose image `postgres:16`; 16 is what CI integration tests against) |
| Docker Compose | v2 |
| pm2 | Installed by the from-source installer if missing |

Regenerating protobuf additionally needs `protoc`, `protoc-gen-go`, and `protoc-gen-go-grpc`. Checked-in `*.pb.go` is enough for a normal build.

## Job interpreters

Jobs run through the interpreter configured on the job (default `bash`):

- `bash`
- `sh`
- `python3`
- `node`

The binary must exist on the **agent** host. CronCompose does not install runtimes on targets.

Cron syntax is **standard 5-field** only (`minute hour day-of-month month day-of-week`), plus a preset picker in the wizard. No seconds field — sub-minute cadence wants a daemon, not cron. Timezones are IANA names evaluated on the agent.

## Network and NAT

| Constraint | Support |
|------------|---------|
| Agent behind NAT / CGNAT / home router | Yes — agent dials out |
| Control plane behind HTTP reverse proxy (nginx, Caddy, Cloudflare Tunnel) | Yes, for the **HTTP** port (`/app` + `/api`) |
| Agent gRPC through an HTTP proxy / Cloudflare HTTP tunnel | **No** — mTLS must be TCP/L4 passthrough (or a dedicated hostname that is not HTTP-terminated) |
| Inbound ports on the agent host | None required |
| Multiple control-plane replicas | **Not supported** — log broker and connector request registry are in-process |

Typical production split: browsers hit `https://cc.example.com` (TLS terminated at the proxy → HTTP `:8080`); agents hit `cc.example.com:9090` or a dedicated `agents.example.com:9090` with TCP passthrough.

## Connectors (implemented vs designed)

| Kind | Status |
|------|--------|
| nginx | Implemented |
| systemd | Implemented |
| Docker | Implemented |
| pm2 | Implemented |
| Apache, Caddy, Traefik, HAProxy | Designed, not implemented |
| System cron, ufw | Designed, not implemented |

Connectors manage software **already installed** on the target. They do not install nginx/Docker/etc.

## Resource limits

CPU quota, memory, max processes, and IO weight use `systemd-run`. The agent checks that systemd is **running**, not merely installed. On hosts without a running systemd (some containers, macOS), the job still runs unlimited, with a note in the log.

## Browser

The UI is a Next.js App Router app at `/app`. Use a current Chromium, Firefox, or Safari. The web terminal uses xterm.js over WebSocket; older browsers that lack WebSocket will not get a terminal.

## Scale limits (honest)

- **Single control-plane replica.** A second replica would not see the first one’s agent streams.
- **Log storage in Postgres**, capped per run (default 5 MiB). Object storage is not built.
- **Single tenant.** No `org_id`; one deployment is one team.
- **Windows installer parity** for the agent is not implemented.

## Versioning

Tagged releases: **v0.0.3** (latest), v0.0.2, v0.0.1. See [Releases](Releases.md). Agent packages and static binaries are attached to GitHub Releases. The control plane can offer agents a checksum-verified self-update when `AGENT_UPDATE_VERSION`, `AGENT_UPDATE_URL`, and `AGENT_UPDATE_SHA256` are all set (or via GitHub release polling). There is no unverified update path.

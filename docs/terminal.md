# Web terminal

The web terminal gives an operator a shell on any enrolled server, straight from the
portal. It runs in two modes: an interactive PTY shell (arrow keys, tab completion,
full-screen programs like vim and htop) and a one-shot command console (type a command,
stream its output, exit). Both share one transport and one UI.

This is an additive feature. It reuses the agent's existing outbound gRPC stream, the
per-server connection registry, the audit log, and RBAC. No new inbound access to a server
is required, so the Raspberry-Pi-behind-NAT story still holds: the browser talks to the
control plane, and the control plane relays over the stream the agent already dialed out.

## Access control

Opening a terminal is admin/owner only. It is the most powerful action in the product: a
shell runs as the agent's operating-system user (often root, depending on how the agent
was installed), so it can do anything that user can. Viewers and operators cannot open a
terminal, and the entry point is hidden from them in the UI.

Every session is written to the audit log: `terminal.open` and `terminal.close`, each
tagged with the actor, the server, the session id, and the mode.

## Flow

```
browser (xterm.js)
   | WebSocket  /api/servers/:id/terminal/ws   (session cookie, admin/owner)
   v
control plane (Fiber)  --- TerminalBus routes output by session_id
   | ServerMessage.TerminalInput  /  AgentMessage.TerminalOutput
   v  (existing mTLS gRPC AgentStream)
agent  ---  PTY running the login shell or a one-shot command
```

1. The browser opens a WebSocket. The control plane authenticates the session cookie and
   checks the admin/owner role (same middleware as the REST API).
2. The control plane mints a `session_id` (ULID), registers it on the `TerminalBus`, and
   sends `TerminalInput{op:"open"}` to the agent over the stream.
3. The agent starts a pseudo-terminal (creack/pty) running either the login shell
   (`$SHELL -i`) or `$SHELL -c "<command>"`, and streams `TerminalOutput{kind:"data"}`
   back. This output is EPHEMERAL: it is sent on a direct, non-persisted path, never
   through the agent's durable outbox, so terminal bytes are never stored or replayed on
   reconnect.
4. Keystrokes and resizes flow browser -> control plane -> agent as more `TerminalInput`
   messages. PTY output flows the other way and is written to xterm.

When the browser disconnects, the control plane sends `TerminalInput{op:"close"}`. When the
agent's stream drops, the agent kills every live session, so no orphan shells linger.

## Protocol

### gRPC (proto/agent.proto)

Two messages were added to the existing stream envelopes; `session_id` correlates a
session, and many can run concurrently per server.

- `ServerMessage.terminal_input` (field 6): `op` is one of `open | data | resize | close
  | signal`; `kind` (on open) is `shell | command`; plus `data`, `cols`, `rows`,
  `command`, `signal`.
- `AgentMessage.terminal_output` (field 9): `kind` is one of `data | started | exit |
  error`; plus `data`, `exit_code`, `message`.

### WebSocket (browser <-> control plane)

- Browser to server: JSON text frames. First frame must be
  `{"type":"init","mode":"shell|command","command":"...","cols":N,"rows":N}`. After that:
  `{"type":"stdin","data":"..."}`, `{"type":"resize","cols":N,"rows":N}`,
  `{"type":"signal","signal":"INT"}`, `{"type":"close"}`. (Raw binary frames are also
  accepted as stdin.)
- Server to browser: PTY output as binary frames (raw bytes, so partial UTF-8 and control
  sequences are preserved); control events as JSON text frames: `{"type":"started"}`,
  `{"type":"exit","code":N}`, `{"type":"error","message":"..."}`.

Sending the command in the init frame (not the URL) keeps it out of access logs.

## Why WebSocket and a PTY

A terminal needs low-latency, bidirectional, byte-accurate streaming, which SSE (used for
run logs) cannot provide for input. A PTY (rather than the job executor's `bash -c` with
piped stdout/stderr) is what makes interactive programs, job control, colors, and window
resizing work. The agent stays Unix-only here, consistent with the executor.

## Files

- `proto/agent.proto`: `TerminalInput`, `TerminalOutput`, and the two oneof fields.
- `agent/internal/terminal/`: `session.go` (one PTY), `manager.go` (sessions by id).
- `agent/internal/runtime/terminal.go`: the manager wiring and the ephemeral `sendDirect`
  path; plus small edits to `runtime.go` (direct channel, drain-loop case, CloseAll on
  disconnect, `terminal` capability in Hello) and `sync.go` (route TerminalInput).
- `control-plane/internal/agentgw/terminal.go`: `TerminalBus` (point-to-point, the
  terminal analogue of `LogBroker`); plus edits to `server.go`, `service.go`, `stream.go`.
- `control-plane/internal/terminal/`: `handler.go` (WebSocket bridge), `routes.go`.
- `web/components/terminal/TerminalView.tsx`, `web/app/servers/[id]/terminal/page.tsx`,
  `web/app/terminal.css`; entry point added to the server detail page (admin/owner only).

## Build

The control plane and agent are not compiled in this repo's authoring sandbox (no Go
toolchain or protoc), so after pulling these changes:

1. `make proto` to regenerate `proto/agent/v1` with the terminal messages.
2. `cd agent && go mod tidy` (pulls `github.com/creack/pty`).
3. `cd control-plane && go mod tidy` (pulls `github.com/fasthttp/websocket`).
4. `cd web && npm install` (pulls `@xterm/xterm`, `@xterm/addon-fit`).
5. Rebuild control plane, agent, and web (or run `./update.sh`), then restart.

The web build (`next build`) and `tsc --noEmit` were verified green.

## Security notes and future work

- The shell runs as the agent's user with no extra sandboxing. Running the agent as a
  dedicated, least-privilege user is recommended; honoring a per-session `run_as_user`
  (via sudo allowlist, mirroring the connectors `privexec` design) is future work.
- The WebSocket handshake is guarded against cross-site WebSocket hijacking by an Origin
  check: a browser Origin is accepted only when it is same-origin (same hostname as the
  request), the configured `PUBLIC_HTTP_URL` host, or a loopback address (so `next dev`
  works locally). An empty Origin (non-browser clients, which cannot be driven cross-site)
  is allowed; the session cookie plus admin/owner role still apply in every case. Set
  `PUBLIC_HTTP_URL` to your public base URL in production so the public hostname is
  accepted even when the control plane sees a different internal Host behind a proxy.
- Output under extreme backpressure is dropped rather than blocking the agent's stream
  receive loop (a 512-message per-session buffer). Interactive use never hits this.
- Possible later additions: session recording to the audit store, idle timeout, a global
  or per-server "disable terminal" switch, and copy/paste-friendly download of a session
  transcript.
```

## run_as

A session can request an OS user via `run_as` in the init frame (`TerminalInput.run_as`,
field 9). Empty means the agent's own user, which is the previous behaviour.

Resolution happens in `agent/internal/osuser` before the PTY is created. A user that
does not exist, or that this agent cannot become because it is not root, fails the
session with an explicit error frame. It never silently falls back to the agent's user:
an operator who asked for a shell as `deploy` and got one as `root` would have no way to
know.

When the switch succeeds the session takes that user's login shell, home directory,
uid/gid and supplementary groups, so it behaves like an ssh login rather than like the
agent wearing someone else's name.

The endpoint remains admin/owner only and every session is still audited, now including
the requested user.

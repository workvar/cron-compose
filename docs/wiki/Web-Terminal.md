# Web terminal

Operators can open a shell on any enrolled server from the portal. Two modes share one transport and one UI:

- **Interactive PTY** — arrow keys, tab completion, vim, htop.
- **One-shot command console** — type a command, stream output, exit.

This reuses the agent’s outbound gRPC stream. No inbound access to the server. Spec: [`docs/terminal.md`](https://github.com/workvar/cron-compose/blob/main/docs/terminal.md).

## Access control

**Admin/owner only.** A shell runs as the agent’s OS user (often powerful, depending on install). Viewers and operators cannot open a terminal; the entry point is hidden from them.

To open shells as other OS users, enable [Agent-Root-Access](Agent root access.md) on the server page.

Every session is audited: `terminal.open` and `terminal.close`, with actor, server, session id, and mode.

## Flow

```
browser (xterm.js)
   | WebSocket  /api/servers/:id/terminal/ws   (session cookie, admin/owner)
   v
control plane  --- TerminalBus by session_id
   | ServerMessage.TerminalInput  /  AgentMessage.TerminalOutput
   v  (existing mTLS gRPC AgentStream)
agent  ---  PTY (login shell or $SHELL -c "<command>")
```

1. Browser opens a WebSocket. Control plane checks session + role.
2. Control plane mints a ULID `session_id`, sends `TerminalInput{op:"open"}`.
3. Agent starts a PTY (`creack/pty`) and streams `TerminalOutput{kind:"data"}`.
4. Keystrokes and resizes flow browser → control plane → agent; PTY bytes flow back into xterm.

On browser disconnect the control plane sends `close`. If the agent stream drops, the agent kills every live session so no orphan shells linger.

## Ephemeral on purpose

Terminal bytes are **not** written to the agent’s durable outbox. They are never stored or replayed on reconnect. Job logs persist; shells do not.

## Why WebSocket and a PTY

A terminal needs low-latency, bidirectional, byte-accurate streaming. SSE (used for run logs) cannot carry input. A PTY, rather than `bash -c` with pipes, is what makes interactive programs, job control, colors, and window resize work.

# CronCompose v0.0.38

Fixes deploy run pages stuck on **(no output yet)** while the run showed
**Running** (and a misleading **exit 0**). Live deploy logs could vanish or never
open in the browser even when the agent was working.

## Highlights

- **Durable deploy log stream** — Deploy progress events now go through the
  agent's durable outbox (same path as job run logs) instead of a droppable
  direct buffer, so install output survives brief stream blips.
- **SSE that actually opens** — Log streams flush a `: connected` comment and
  periodic keepalives when the snapshot is empty, so EventSource/proxies see an
  open body before the first chunk.
- **No fake exit 0** — Marking a run `running` no longer stamps `exit_code=0`.
  The UI only shows the exit pill after the run finishes.
- **REST log fallback** — `GET /deploy-runs/:id/logs` plus a short poll on the
  run page, so a flaky EventSource still fills the terminal.

## Upgrade

Update the **control plane** and **agent**. No new migration. Open a new deploy
(or re-open a live run page) after both are on v0.0.38 to see live output.

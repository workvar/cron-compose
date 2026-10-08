# CronCompose v0.0.40

Deploys no longer die when the agent briefly loses the control plane, and the
live run page shows how far each process has gotten.

## Highlights

- **Installs survive a stream blip** — The deploy was tied to the agent’s gRPC
  stream, so a stalled connection (a Next.js build pegging a small host)
  canceled `npm install && npm run build` with `context canceled` after the
  compile had already succeeded. The install now keeps running. Its log stays
  in the outbox and flushes when the agent reconnects. The run still stops on
  its own timeout, an explicit cancel, or agent shutdown.
- **Per-process progress** — The deploy run page shows a completion bar and a
  step rail: Preflight, Clone, Installing {name}, Building {name}, Activate,
  Starting, Checking. The step that is running shows its own percent.
- **One terminal per process** — When a project has several processes, each has
  a collapsible log. On a wide screen they sit side by side, so you can watch
  one build while another is still installing.

## Upgrade

Update the **agent** and **web**. The control plane is unchanged. No new
migration.

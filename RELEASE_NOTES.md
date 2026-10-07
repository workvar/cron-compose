# CronCompose v0.0.33

Deploy opens on large server cards with live process counts. Opening a server
shows PM2, systemd, and Docker tabs so you can import an already-running
process into a git-style Deploy Project—without restarting it. Environment
values stay masked until a passkey step-up reveals them.

## Highlights

- **Deploy = pick a server** — `/deploys` lists big cards (emoji, status,
  OS/arch) with PM2 / systemd / Docker process counts. Click through to that
  server’s process tabs.
- **Import running processes** — from each tab, Import prefills name, command,
  cwd, and (when detectable) the git remote from the working directory. Creates
  a Deploy Project and **does not** start a redeploy run.
- **Richer agent inventory** — PM2 `jlist`, systemd `systemctl show`, and
  Docker `inspect` expose command, cwd, args, and env key lists on discovery.
- **Passkey-gated env** — values show as dots until step-up. Live inspect via
  `POST /connectors/:id/objects/:ref/inspect`; sealed project env via
  `POST /deploys/:id/env/reveal`.
- **APIs** — `GET /servers/:id/deploy-inventory`,
  `POST /servers/:id/deploys/import-process`.

## Upgrade

Rebuild and restart the **control plane**, **web UI**, and **agent** (inspect
and enriched attributes require the new agent). No database migration is
required. Enroll a passkey before revealing environment values.

# CronCompose v0.0.65

The agent runs on Windows, a deploy card controls one app, and an unchanged
install or build is reused from a cache on the server.

## Highlights

- **Windows agent**: Jobs, deploys, web terminal, connectors, ports, and tools work
  on Windows 10/11 and Windows Server 2019+. `bash`/`sh` scripts run under Git
  Bash so existing scripts carry over; `powershell`, `pwsh`, and `cmd` are new
  job interpreters.
- **Release binaries**: Each release attaches
  `croncompose-agent-windows-amd64.exe`, `croncompose-agent-windows-arm64.exe`,
  `checksums.json`, and `SHA256SUMS`. A Windows CI job (vet, tests, service install
  smoke test) gates the release.
- **One-line install**: The portal's **Install on Windows** command runs
  `install-agent.ps1` from an Administrator PowerShell: download, sha256 check,
  enroll, and register the `croncompose-agent` service (LocalSystem, auto-start,
  restart on failure).
- **Auto-update**: Windows agents check GitHub every 6 hours and install newer
  releases from the checksum-pinned `.exe`; *Settings → Updates* also works.
  `AGENT_AUTO_UPDATE=0` pins a version. Linux and macOS update as before.
- **Windows parity**: Resource limits through Job Objects (CPU, memory, max
  processes), whole-tree kill on timeout and cancel, ConPTY terminals
  (PowerShell), a Windows services connector, `netstat`-based Ports, winget or
  Chocolatey installs on the Tools page, `pm2 resurrect` at start, and embedded
  timezone data.
- **One app at a time**: Start, Skip, Stop, and Restart on a deploy card run that
  app's steps. Restart re-runs that app from install through health and leaves
  the others running. A shared PM2 ecosystem is narrowed to that app before
  start, so updating its env does not restart its siblings.
- **Install and build cache**: An unchanged lockfile restores `node_modules` or
  `vendor` instead of installing again. Unchanged source restores the binary or
  `.next` output. A source-only change still compiles, with the previous
  `.next/cache` warmed first. The cache is `<project>/.cc-cache` and is kept
  when old releases are pruned. Delete that directory to force a clean install
  and build.

## Upgrade

Update the agent, control plane, and web together. The step buttons call a new
API, and caching and one-app restarts run in the agent.

1. Pull this tag on the control plane and restart it, so the portal shows the
   Windows install command and the per-app step actions.
2. Linux and macOS agents: update from *Settings → Updates*.
3. Windows: add a server and run its **Install on Windows** command, or let an
   existing Windows agent auto-update. Install Git for Windows first if you
   deploy apps or run `bash` jobs.

See `docs/wiki/Windows-Agent.md` for what differs on Windows.

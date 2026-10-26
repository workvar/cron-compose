# Windows agent

The CronCompose agent runs on **Windows 10, Windows 11, and Windows Server 2019 or later**
(x64 and ARM64). It is the same agent as on Linux: it dials out to the control plane over
mTLS, keeps its own copy of every job and schedule, runs deploys, opens web terminals, and
reports connectors. This page covers what is different on Windows.

## Install

1. In the portal, add a server. The *Server created* page shows an **Install on Windows**
   command.
2. Open **PowerShell as Administrator** on the Windows machine and paste it:

```powershell
$env:TOKEN='<token>'; $env:AGENT_ENROLL_HTTP='https://cc.example.com/api'; $env:AGENT_GRPC_ADDR='agents.example.com:9077'; irm https://github.com/workvar/cron-compose/releases/latest/download/install-agent.ps1 | iex
```

The installer:

- downloads `croncompose-agent-windows-<arch>.exe` from the GitHub release and checks its
  sha256 against the release's `checksums.json` (no Go or build tools needed);
- copies it to `C:\Program Files\CronCompose\croncompose-agent.exe`;
- creates `C:\ProgramData\CronCompose` (readable only by SYSTEM and Administrators, since it
  holds the agent's private key) and writes its settings to `agent.env` there;
- enrolls with the control plane, then registers and starts the **croncompose-agent**
  Windows service.

Optional settings, set as `$env:NAME='value';` before `irm`:

| Variable | Default | Meaning |
|---|---|---|
| `AGENT_GRPC_SNI` | host part of `AGENT_GRPC_ADDR` | Name to verify in the control plane's certificate |
| `AGENT_VERSION` | the release the script came from | Tag to install |
| `GITHUB_REPO` | `workvar/cron-compose` | Where releases come from |
| `INSTALL_DIR` | `C:\Program Files\CronCompose` | Binary location |
| `DATA_DIR` | `C:\ProgramData\CronCompose` | State, certificates, logs |
| `AGENT_AUTO_UPDATE` | `1` | `0` turns off self-updates from GitHub |

### Recommended: Git for Windows

Deploys need `git`, and `bash` / `sh` jobs and deploy scripts run under Git Bash, so the same
scripts work on Linux and Windows. Install it machine-wide, then restart the agent:

```powershell
winget install --id Git.Git -e --scope machine
Restart-Service croncompose-agent
```

`powershell`, `pwsh`, and `cmd` jobs work without it.

## What works, and how

| Feature | Windows behaviour |
|---|---|
| Scheduled jobs, run now, catch-up, offline runs | Same as Linux. Timezones use Go's embedded IANA database. |
| Interpreters | `bash`, `sh` (Git Bash), `powershell`, `pwsh`, `cmd`, `python3` (uses `python.exe`, skipping the Microsoft Store stub), `node` |
| Resource limits (CPU, memory, max processes) | Enforced with a Windows **Job Object** instead of `systemd-run`. `io_weight` is ignored with a note in the run log. |
| Timeout and cancel | Kill the whole process tree (the Job Object), not just the interpreter |
| Deploys | Same pipeline: clone, release folders, install scripts (Git Bash), `current` link swap, health checks. `/opt/apps/web` style paths map to `C:\opt\apps\web`. |
| Process managers | `pm2` and `docker` (Docker Desktop). `systemd` fails with a clear message. The agent runs `pm2 resurrect` at start because `pm2 startup` does not exist on Windows. |
| Web terminal | ConPTY with PowerShell 7 (`pwsh`) when installed, else Windows PowerShell. `AGENT_SHELL` overrides it. |
| Connectors | pm2, Docker, nginx (if installed), plus **Windows services** (list, start, stop, restart, pause, resume, Event Log, listening ports) in place of systemd |
| Ports page | `netstat -ano`, with process names and service/pm2 owners |
| Tools page | Detects tools on the machine and installs node, go, python, git through winget (falls back to Chocolatey); pm2, yarn, pnpm through `npm -g` |
| Agent root access | Always on: the service runs as LocalSystem, the Windows equivalent of root |
| Run as another user | Not supported. Jobs, deploys, and terminals run as the service account. Leave *run as* empty, or use `root`. |
| Networks (Wi-Fi, Bluetooth, cellular) | Not available, same as macOS: the Networks page reports no network backend for the server. |

## Updates

Windows agents update from prebuilt, checksum-verified release binaries, never from source.

- **Automatically**: every 6 hours (`AGENT_AUTO_UPDATE_HOURS`) the agent asks GitHub for the
  latest release. When it is newer, the agent downloads that release's `.exe`, verifies it
  against `checksums.json`, swaps it in, and exits. The service's recovery actions restart
  it on the new version within seconds, and the previous binary is deleted on the next
  start.
- **From the UI**: *Settings → Updates → Update* works the same way and shows progress.

To pin a version, set `AGENT_AUTO_UPDATE=0` in `C:\ProgramData\CronCompose\agent.env` and
run `Restart-Service croncompose-agent`.

## Operate

```powershell
Get-Service croncompose-agent                               # status
Restart-Service croncompose-agent                           # after editing agent.env
Get-Content C:\ProgramData\CronCompose\logs\agent.log -Tail 50 -Wait
& 'C:\Program Files\CronCompose\croncompose-agent.exe' version
sc.exe qfailure croncompose-agent                           # restart-on-failure actions
```

Settings live in `C:\ProgramData\CronCompose\agent.env` (`KEY=value` lines, the same
variable names as the Linux unit). Environment variables set on the service win over the
file.

## Uninstall

```powershell
$env:UNINSTALL='1'; irm https://github.com/workvar/cron-compose/releases/latest/download/install-agent.ps1 | iex
```

Add `$env:PURGE='1';` to also delete `C:\ProgramData\CronCompose` (the agent identity, logs,
and pm2 home). Then delete the server in the portal.

## Troubleshooting

| Symptom | Fix |
|---|---|
| `bash is not available` in a deploy or job | Install Git for Windows (above), then restart the service |
| `pm2: not found` during a deploy | `npm install -g pm2` from an Administrator prompt, or install it from the Tools page |
| Agent shows offline | Check the log file, then `Test-NetConnection agents.example.com -Port 9077` |
| Web terminal does not open | Needs Windows 10 1809 / Server 2019 (build 17763) or later |
| Job times out but a child keeps running | Report it: the Job Object should stop the whole tree |

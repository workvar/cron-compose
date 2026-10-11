# CLI (`cc`)

Power-user client for the CronCompose control plane. Talks to the same REST API the UI uses, signs requests with a saved session cookie, and supports SSE live-log streaming for `run logs --follow`.

## Install

```sh
make cli
sudo install -m 0755 cli/bin/cc /usr/local/bin/cc
```

Config: `~/.croncompose/config.json` (mode 0600). Override the control-plane URL with `CC_API_BASE`, for example `export CC_API_BASE=https://cc.example.com/api/v1`.

## Quick start

```sh
cc login --email you@example.com   # prompts for password
cc whoami
cc server ls
cc server add --name kitchen-pi --description "Pi behind the kitchen TV"
cc job ls --server <server_id>
```

## Reference

### auth

| Command | Purpose |
|---------|---------|
| `cc login` | Prompt for email/password, save the session |
| `cc logout` | Clear the saved session |
| `cc whoami` | Print the signed-in user and role |

### servers

| Command | Purpose |
|---------|---------|
| `cc server ls` | List servers with status + last seen |
| `cc server add --name <n>` | Create a server, print install command |
| `cc server rm <server_id>` | Delete a server |

### jobs

| Command | Purpose |
|---------|---------|
| `cc job ls [--server <id>]` | List jobs |
| `cc job add --name --cron --script-file ...` | Create a job |
| `cc job enable <id>` / `cc job disable <id>` | Toggle the schedule |
| `cc job run <id>` | Manually run (fans out to all targets) |
| `cc job rm <id>` | Delete a job |

`--server <id>` for a single target, or `--labels env=prod,role=worker` for a label selector. Script from a file, or `-` for stdin:

```sh
cc job add \
  --server srv_abc \
  --name backup \
  --cron "0 */6 * * *" \
  --tz UTC \
  --script-file ./backup.sh \
  --cpu-quota 50 --memory-mb 256 \
  --secrets API_KEY,DB_PASSWORD
```

### runs

| Command | Purpose |
|---------|---------|
| `cc run ls <job_id>` | Recent runs for a job |
| `cc run get <run_id>` | JSON detail |
| `cc run logs <run_id>` | Print full captured logs |
| `cc run logs <run_id> --follow` | Live SSE stream until the run finishes |

### secrets (admin)

| Command | Purpose |
|---------|---------|
| `cc secret ls` | List secret names (values never returned) |
| `cc secret add --name API_KEY` | Prompt for value, save encrypted |
| `cc secret add --name K --stdin` | Pipe value from stdin |
| `cc secret rm <secret_id>` | Delete |

### audit (admin)

| Command | Purpose |
|---------|---------|
| `cc audit` | Print recent audit entries |

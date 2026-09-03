# CronCompose v0.0.4

Releases no longer ship prebuilt binaries. Hosts clone the tag and build locally, and Settings can update a live stack without GitHub Actions minutes.

## Highlights

- **Notes-only GitHub releases** — tagging `v*` publishes `RELEASE_NOTES.md` plus a baked `install-agent.sh` (and a Windows stub). No agent binaries, `.deb`, or `.apk` packages are built in CI.
- **Source agent installer** — `install-agent.sh` clones the release tag, `go build`s the agent, enrolls it, installs the service, then deletes the source tree. Needs `git` and Go 1.25+ on the target.
- **In-place stack updates** — Settings polls GitHub about once a day (or **Check now**). **Update** on the control-plane host checks out the tag, rebuilds web + control plane + agent, restarts, and strips leftover build inputs. A fullscreen overlay stays up until the stack is back.
- **Remote agents rebuild themselves** — the same Update button tells a standalone agent to clone the tag, rebuild its binary, swap it, and discard the clone.

## Upgrade notes

### Control plane (source install)

From Settings → Updates, click **Update** on this host. Or by hand:

```sh
cd cron-compose
git fetch --tags
git checkout --force v0.0.4
./update.sh --no-pull
```

No new database migrations in this release.

### Agent (Linux / macOS)

```sh
curl -sSL https://github.com/workvar/cron-compose/releases/latest/download/install-agent.sh | \
  sudo TOKEN=<token> \
       CONTROL_PLANE_HTTP=https://<host>/api/v1 \
       CONTROL_PLANE_ADDR=<host>:9090 \
       bash
```

Windows is not supported for the agent (Unix process APIs). `install-agent.ps1` only says so.

### Existing installs

Set these if they are not already in `.env` (new installs write them automatically):

```
GITHUB_RELEASE_REPO=workvar/cron-compose
INSTALL_SCRIPT_URL=https://github.com/workvar/cron-compose/releases/latest/download/install-agent.sh
AGENT_UPDATE_POLL_MINUTES=1440
CC_SOURCE_ROOT=<path-to-checkout>
```

The local control-plane agent needs label `croncompose.role=stack` for the updating overlay (new installer enrollments set this).

## Documentation

- [Operations — agent packaging](docs/operations.md#agent-packaging)
- [Deployment — updates](DEPLOYMENT.md#updates-source-builds)

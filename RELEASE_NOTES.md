# CronCompose v0.0.20

Agents now enroll against the right gRPC address, say *why* they cannot connect,
and Agent root access reports real errors instead of a silent 60-second timeout.

## Highlights

- **Enrollment hands out the public gRPC address** — The enroll endpoint used to
  return the control plane's *listen* address (`GRPC_ADDR`, e.g. `:9077`). Agents
  saved it in `identity.json`, where it overrides `CONTROL_PLANE_ADDR`, so a
  remote agent dialed port 9077 on its own machine forever. Enrollment now
  returns `PUBLIC_GRPC_ADDR` and never an address without a host.
- **Agents recover from a hostless saved address** — An agent whose
  `identity.json` holds a bare `":9077"` now ignores it, logs
  `ignoring enrolled control-plane address without a host`, and dials
  `CONTROL_PLANE_ADDR`. No re-enroll needed once the agent is updated.
- **Connection errors say what is wrong** — Refused ports, TLS mismatches and
  wrong services all used to read `context deadline exceeded`. The agent now
  reports the underlying error with a hint: *nothing is listening there*,
  *something other than this CronCompose control plane answered on that port*,
  or *the host does not resolve*.
- **gRPC port clashes fail loudly** — If another process already holds
  `GRPC_ADDR`, the control plane stops at startup with
  `another process already uses this port` and the `ss` command to find it.
- **Startup shows where agents dial** — The control plane logs
  `agents dial public_grpc_addr=…`, and warns when `PUBLIC_GRPC_ADDR` is unset
  and derived from `PUBLIC_BASE_URL`. That host is often behind an HTTP
  proxy/CDN (e.g. Cloudflare's orange cloud), which cannot carry raw gRPC.
- **Agent root access: no more silent timeouts** — Before elevating,
  `agent-privctl` checks that the agent really runs as
  `croncompose-agent.service` (not pm2/nohup/a shell). After `daemon-reload` it
  checks that the unit resolves `User=root`, so another drop-in overriding
  `User=` is caught. Both failures show in the UI. The timeout message now
  points at `systemctl show`, `journalctl` and the wiki.
- **Packaged agents can elevate** — The .deb/.apk unit's `ProtectSystem=full`
  made `/etc` read-only for the helper. The unit now has
  `ReadWritePaths=-/etc/systemd/system/croncompose-agent.service.d`, and
  postinstall creates that directory.

## Docs

- Wiki: [Agent installation](https://github.com/workvar/cron-compose/wiki/Agent-Installation)
  (gRPC address requirements, pre-install checks, verification, error table,
  uninstall) and [Agent root access](https://github.com/workvar/cron-compose/wiki/Agent-Root-Access)
  (troubleshooting, manual enable/disable).
- `docs/operations.md`: Agent root access troubleshooting.

## Upgrade notes

### Control plane (source install)

From Settings → Updates, click **Update** on this host. Or by hand:

```sh
cd cron-compose
git fetch --tags
git checkout --force v0.0.20
./update.sh --no-pull
```

No migrations. After restarting, check the log line `agents dial …`. If it warns
that `PUBLIC_GRPC_ADDR` is derived and your web host is proxied, point a
DNS-only record at the control plane and set:

```sh
PUBLIC_GRPC_ADDR=agents.example.com:9077
TLS_HOSTS=localhost,127.0.0.1,agents.example.com
```

Install commands from **Add server** use `PUBLIC_GRPC_ADDR`, so fix it before
enrolling new agents.

### Agent (Linux / macOS)

Update agents (Settings → Updates, or reinstall) to get the hostless-address
fallback and the clearer connection errors. To fix an existing agent by hand,
check its saved address:

```sh
sudo cat /var/lib/croncompose/identity.json
```

If `control_plane_grpc_addr` is `":<port>"`, stop the agent, delete that field
or set it to a reachable `host:port`, and start the agent again.

`agent-privctl` is rebuilt and reinstalled by `update.sh` and the installers.
Package installs from before this release can add the writable path with
`sudo systemctl edit croncompose-agent`:

```ini
[Service]
ReadWritePaths=-/etc/systemd/system/croncompose-agent.service.d
```

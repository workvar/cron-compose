# Agent installation

How to put a CronCompose agent on a server from the portal, check it connected, and remove it cleanly. For control-plane setup see [Installation](Installation.md).

## Before you start: the gRPC address

Agents connect **out** to the control plane over mTLS gRPC. The address they dial must reach the control plane's `GRPC_ADDR` port **directly**, as raw TCP.

| Setting (control plane `.env`) | Meaning |
|---|---|
| `GRPC_ADDR` | Listen address, e.g. `:9077`. Not something agents can dial. |
| `PUBLIC_GRPC_ADDR` | `host:port` agents dial, e.g. `agents.example.com:9077`. |
| `PUBLIC_BASE_URL` | Web URL. If `PUBLIC_GRPC_ADDR` is unset, agents dial **this host** on the `GRPC_ADDR` port. |

**Proxied hosts don't work.** A hostname behind Cloudflare's orange-cloud proxy (or any HTTP proxy/CDN) answers only web ports. Agents dialing it time out with `context deadline exceeded`. Give gRPC its own DNS-only (grey-cloud) record pointing at the origin, or a TCP tunnel, and set:

```sh
PUBLIC_GRPC_ADDR=agents.example.com:9077
TLS_HOSTS=localhost,127.0.0.1,agents.example.com   # the name agents verify
```

Restart the control plane. Its startup log prints `agents dial public_grpc_addr=…`, or a warning if the address was derived from `PUBLIC_BASE_URL`.

**Check reachability from the target server first:**

```sh
nc -vz -w5 agents.example.com 9077
openssl s_client -connect agents.example.com:9077 -servername agents.example.com </dev/null 2>/dev/null | grep -E 'subject|issuer'
```

`nc` must connect. `openssl` should show a certificate issued by the CronCompose CA. A different certificate, or no TLS at all, means another service owns that port.

**Check nothing else uses the port on the control-plane host:**

```sh
sudo ss -ltnp | grep ':9077'
```

Only the `control-plane` process should appear. If another service holds the port, the control plane now fails at startup with `another process already uses this port`. Change `GRPC_ADDR` and `PUBLIC_GRPC_ADDR` to a free port.

## Install from the portal

1. **Servers → Add server**, give it a name.
2. Copy the install command. The token is shown once and expires (`ENROLL_TOKEN_TTL`, default 30m). It looks like:

   ```sh
   curl -sSL <install-script-url> | sudo TOKEN=<token> CONTROL_PLANE_HTTP=<public-http-url> CONTROL_PLANE_ADDR=<public-grpc-addr> bash
   ```

   Check that `CONTROL_PLANE_ADDR` is a reachable, non-proxied `host:port` before running it. If it isn't, fix `PUBLIC_GRPC_ADDR` and create the server again.
3. Run it on the target server as a sudo-capable user.

On Linux the script:

| Creates | Path |
|---|---|
| Service user | `croncompose` (system, no login) |
| Binary | `/usr/local/bin/croncompose-agent` |
| Root helper | `/usr/libexec/croncompose/agent-privctl` |
| systemd unit | `/etc/systemd/system/croncompose-agent.service` |
| State (identity, mTLS cert, job cache) | `/var/lib/croncompose` (0700) |
| sudoers grants | `/etc/sudoers.d/croncompose-agent` |

On macOS it installs a LaunchDaemon (`/Library/LaunchDaemons/com.croncompose.agent.plist`) instead of the systemd unit.

## Verify

```sh
systemctl status croncompose-agent
journalctl -u croncompose-agent -f
sudo cat /var/lib/croncompose/identity.json
```

- The server shows **online** in the portal within a few seconds.
- `identity.json` holds `server_id` and possibly `control_plane_grpc_addr`. That address overrides `CONTROL_PLANE_ADDR` from the unit. It must have a host: a bare `":9077"` came from an older control plane that sent its listen address. Current agents ignore such a value, log `ignoring enrolled control-plane address without a host`, and use `CONTROL_PLANE_ADDR` instead.

### Connection errors in the journal

| Error contains | Cause |
|---|---|
| `connection refused` / `nothing is listening there` | Control plane down, or wrong port. |
| `first record does not look like a TLS handshake`, `certificate signed by unknown authority`, `something other than this CronCompose control plane answered` | Another service owns that port, or the address points at the wrong machine. |
| `certificate is valid for …` | The dialed host isn't in the control plane's `TLS_HOSTS`. Add it and restart (the CA is kept). |
| `context deadline exceeded` only | Packets dropped: firewall, security group, or a proxied (orange-cloud) host. |
| `no such host` | DNS. |

Older agents reported every one of these as `context deadline exceeded`. Update the agent to see the specific cause.

## Change the address of an installed agent

```sh
sudo systemctl stop croncompose-agent
sudo systemctl edit croncompose-agent          # [Service] Environment=CONTROL_PLANE_ADDR=agents.example.com:9077
sudo nano /var/lib/croncompose/identity.json   # set or delete "control_plane_grpc_addr"
sudo systemctl start croncompose-agent
```

## Uninstall an agent (Linux, script install)

1. In the portal, delete the server (or note its name to clean up later).
2. On the server:

```sh
sudo systemctl disable --now croncompose-agent
sudo rm -f  /etc/systemd/system/croncompose-agent.service
sudo rm -rf /etc/systemd/system/croncompose-agent.service.d
sudo systemctl daemon-reload
sudo systemctl reset-failed croncompose-agent 2>/dev/null || true

sudo rm -f  /usr/local/bin/croncompose-agent
sudo rm -rf /usr/libexec/croncompose
sudo rm -f  /etc/sudoers.d/croncompose-agent
sudo rm -rf /var/lib/croncompose          # identity, cert, job cache: this server can't reconnect after this
sudo rm -rf /etc/croncompose              # only if present (package installs)
sudo userdel croncompose 2>/dev/null || true
```

3. Confirm nothing is left:

```sh
systemctl status croncompose-agent        # "could not be found"
pgrep -af croncompose-agent               # no output
ls /etc/sudoers.d/ | grep croncompose     # no output
```

**Package installs:** use `sudo apt remove croncompose-agent` (or `sudo apk del croncompose-agent`). Then remove `/var/lib/croncompose`, `/etc/croncompose`, `/etc/sudoers.d/croncompose-agent` and any `/etc/systemd/system/croncompose-agent.service.d/`, which the package doesn't own.

**macOS:**

```sh
sudo launchctl bootout system /Library/LaunchDaemons/com.croncompose.agent.plist
sudo rm -f /Library/LaunchDaemons/com.croncompose.agent.plist /usr/local/bin/croncompose-agent
sudo rm -rf /usr/local/var/croncompose
```

**Control-plane host:** the local agent is part of the stack. Use `./uninstall.sh` from the checkout (see [Installation](Installation.md)), not the steps above.

## Reinstall

After uninstalling, create the server again in the portal (or reuse it with a fresh token) and run the new install command. Related: [Agent-Root-Access](Agent root access.md).

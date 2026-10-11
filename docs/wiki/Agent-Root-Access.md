# Agent root access

**Agent root access** (server page, admin/owner, passkey step-up) restarts the agent as `root` so the [Web-Terminal](Web terminal.md) can open shells as other OS users. Spec: [`docs/operations.md`](https://github.com/workvar/cron-compose/blob/main/docs/operations.md#agent-root-access).

## How it works

1. The control plane sends `AgentRootCommand{enabled:true}` to the agent.
2. The agent runs `sudo -n /usr/libexec/croncompose/agent-privctl elevate`.
3. `agent-privctl` checks the agent really is `croncompose-agent.service`, writes the drop-in `/etc/systemd/system/croncompose-agent.service.d/root.conf` (`User=root`), runs `daemon-reload`, checks the unit now resolves `User=root`, then restarts the unit.
4. The agent reconnects and its Hello reports `euid_root=true`. The badge turns **On (root)**.

Turning the switch off runs `agent-privctl demote`, which removes `root.conf` and restarts the unit as the original user.

Requirements: Linux with systemd, the agent running **as the `croncompose-agent.service` unit** (not pm2, nohup or a shell), the helper installed at `/usr/libexec/croncompose/agent-privctl`, and the sudoers grant from the installer.

## Troubleshooting: "Agent did not report root after 60s"

This means the agent never came back as root **and** reported no error. Run these on the server.

### 1. Is the unit the process that is running?

```sh
systemctl show -p User,MainPID,FragmentPath,DropInPaths croncompose-agent
pgrep -af 'agent run|croncompose-agent'
```

- `MainPID=0`, or the `pgrep` PID differs from `MainPID` → the agent was started by **pm2 / nohup / a shell**. Restarting the unit does nothing for it. See [Move the agent under systemd](#move-the-agent-under-systemd).
- `User=` is not `root` although `root.conf` exists → another drop-in in `DropInPaths` sets `User=` after it. Remove that line and run `sudo systemctl daemon-reload`.

Confirm with the running PID's cgroup; it must end in `croncompose-agent.service`:

```sh
cat /proc/<pid>/cgroup
```

### 2. Did the drop-in get written?

```sh
ls -l /etc/systemd/system/croncompose-agent.service.d/
cat /etc/systemd/system/croncompose-agent.service.d/root.conf
```

`root.conf` missing on a **.deb/.apk** install from before this fix: the packaged unit has `ProtectSystem=full`, which makes `/etc` read-only for the agent and its sudo children. Fix:

```sh
sudo mkdir -p /etc/systemd/system/croncompose-agent.service.d
sudo systemctl edit croncompose-agent
# add:
#   [Service]
#   ReadWritePaths=-/etc/systemd/system/croncompose-agent.service.d
sudo systemctl restart croncompose-agent
```

### 3. Does the helper run for the agent user?

```sh
ls -l /usr/libexec/croncompose/agent-privctl   # root:root, 0755
sudo cat /etc/sudoers.d/croncompose-agent
sudo -u <agent-user> sudo -n -l | grep agent-privctl
```

If the helper is missing, build and install it from the checkout:

```sh
cd agent && go build -o bin/agent-privctl ./cmd/agent-privctl
sudo install -d -m 0755 /usr/libexec/croncompose
sudo install -m 0755 -o root -g root bin/agent-privctl /usr/libexec/croncompose/agent-privctl
```

If the grant is missing, re-run the installer's sudoers step:

```sh
sudo ./install/lib/agent_sudoers.sh <agent-user>
```

Never grant `NOPASSWD: ALL`.

### 4. Did the agent crash after restarting as root?

```sh
journalctl -u croncompose-agent -n 100 --no-pager
```

## Move the agent under systemd

The default `./install/install.sh` stack runs everything under **pm2**, where root access is not supported (`agent-privctl` fails with `systemd required to run agent as root under pm2`). To switch the stack to systemd units:

```sh
pm2 delete croncompose-agent     # stop the pm2 copy so two agents don't share one identity
sudo ./install/systemd-setup.sh  # writes croncompose-*.service, installs helper + sudoers
echo systemd | sudo tee "$CC_RUNTIME_DIR/agent/.run/agent-supervisor"
```

Then toggle Agent root access again.

## Manual enable (without the helper)

Use this when you want root access and cannot get `agent-privctl` working. Leave the switch **on** in the UI (the timeout does not turn it off); the badge turns **On (root)** once the agent reconnects as root.

`DATA_DIR` is `/var/lib/croncompose` for package / `install-agent.sh` installs, or `$CC_RUNTIME_DIR/agent` for the full stack. `<agent-user>` is the normal service user (`croncompose`, or the user who ran the installer).

```sh
sudo mkdir -p /etc/systemd/system/croncompose-agent.service.d
sudo tee /etc/systemd/system/croncompose-agent.service.d/root.conf >/dev/null <<'EOF'
[Service]
User=root
ProtectHome=false
UnsetEnvironment=AGENT_VERSION
ProtectSystem=false
EOF

# Markers the helper uses to demote back to the right user later.
sudo mkdir -p "$DATA_DIR/.run"
echo <agent-user> | sudo tee "$DATA_DIR/.run/agent-service-user"
echo systemd      | sudo tee "$DATA_DIR/.run/agent-supervisor"

sudo systemctl daemon-reload
sudo systemctl restart croncompose-agent
systemctl show -p User croncompose-agent                                     # User=root
ps -o user= -p "$(systemctl show -p MainPID --value croncompose-agent)"      # root
```

## Manual disable

Turn the switch off in the UI, then on the host:

```sh
sudo rm -f /etc/systemd/system/croncompose-agent.service.d/root.conf
sudo systemctl daemon-reload
sudo systemctl restart croncompose-agent
systemctl show -p User croncompose-agent   # back to the service user
```

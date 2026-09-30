# CronCompose v0.0.23

Fixes the "Enable as root" toggle and the "Reinstall as root" panel staying on
screen after an agent was installed as root. Root installs are now recorded, the
installer makes sure the new agent is the one running, and a toggle change made
while the agent was offline is delivered when it reconnects.

## Highlights

- **Installer replaces the running agent.** Re-running the install command used
  to leave the old non-root process alive, because `systemctl enable --now`
  does not restart an active unit. The installer now stops the agent before
  re-enrolling and restarts it afterwards.
- **Installer reports who the agent runs as.** After starting, it prints the
  agent pid and uid, and warns when that does not match the mode you chose or
  when a stray non-systemd agent process is still running.
- **Root installs show as On (root).** The agent now tells the control plane at
  enrollment whether it runs as root, so the flag matches reality from the first
  connection. This covers macOS root installs too. Agents older than this
  release do not send the field, and the control plane leaves the flag alone.
- **Toggle changes are no longer lost while offline.** If the agent was
  disconnected when you flipped the switch, the control plane sends the command
  once when the agent next connects. It sends once per desired value, so a host
  that cannot elevate (pm2, no sudoers grant) reports its error instead of
  restarting in a loop.
- **Legacy root agents are never demoted automatically.** A turned-off flag is
  only re-sent as a demote when an operator switched it off. Agents installed as
  root before this release keep running as root.
- **Tests and docs.** New installer shell tests, agent enroll contract test,
  control-plane unit and database tests, and new sections in `docs/operations.md`,
  `docs/security.md` and `DEVELOPMENT.md`.

## Upgrade

No database migration. Upgrade the control plane, then install the new agent
build on your hosts.

A host that already reinstalled as root on v0.0.22 and still shows the toggle is
most likely running the old non-root process. Run this once on that host:

    sudo systemctl restart croncompose-agent

Root installs made with an older agent will not be marked as root until the new
agent enrolls or reconnects with this release.

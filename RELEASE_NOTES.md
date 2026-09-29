# CronCompose v0.0.21

The installer's suggested agent gRPC port is now 9077 instead of 9090.

## Highlights

- **Installer suggests gRPC port 9077** — `./install.sh`'s "Agent gRPC port"
  prompt defaulted to `9090`. It now defaults to `9077`, matching the port
  already used in agent install commands and in Cloudflare Tunnel routing for
  remote agents. `.env`'s `GRPC_ADDR` is always written out explicitly no
  matter which port you pick, so this changes only what a fresh install
  suggests, not how an install behaves.

## Upgrade

No action needed for an existing install, this only changes the suggested
default the next time `./install.sh` runs fresh. To move an existing instance
from `9090` to `9077` by hand, set `GRPC_ADDR=:9077` in `.env`, update
`PUBLIC_GRPC_ADDR` and any port-forward or tunnel rule that points at the
gRPC port to match, and restart the control plane.

# CronCompose v0.0.30

Source updates from a root systemd agent failed with
`module cache not found: neither GOMODCACHE nor GOPATH is set`. systemd starts
the agent without a home directory, and Go refuses to build in that case.

## Highlights

- **The agent fills Go's cache paths** before `go build` when `HOME`, `GOPATH`,
  `GOMODCACHE` or `GOCACHE` are empty.
- **The installer writes those variables into the unit** so a fresh root or
  `croncompose` install has a stable cache under `/root` or the data directory.

## Upgrade

If you are stuck on 0.0.28 with this error, set the variables and restart before
retrying the UI update. The running binary still needs a cache path even after
this release is tagged:

```sh
sudo tee /etc/systemd/system/croncompose-agent.service.d/go-cache.conf >/dev/null <<'EOF'
[Service]
Environment=HOME=/root
Environment=GOPATH=/root/go
Environment=GOMODCACHE=/root/go/pkg/mod
Environment=GOCACHE=/root/.cache/go-build
EOF
sudo systemctl daemon-reload
sudo systemctl restart croncompose-agent
```

Then click **Retry** on the server page. After 0.0.30 is installed, later source
updates set the same paths themselves.

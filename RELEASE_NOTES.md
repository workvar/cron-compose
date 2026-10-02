# CronCompose v0.0.29

cloudflared only speaks HTTP/2 to an `https://` origin. The edge listener is plain
HTTP/2, so a tunnel route of `http://localhost:9078` with "Use HTTP/2 to origin" still
ended in `502` and `broken pipe`. `install.sh` and `update.sh` now install nginx on
loopback to terminate TLS and forward gRPC to that listener.

## Highlights

- **nginx TLS bridge.** When edge mode is on, the scripts install nginx if it is missing,
  copy the control plane certificate to `/etc/nginx/croncompose/`, and listen on
  `127.0.0.1:<EDGE_NGINX_PORT>` (9443, or the next free port). The plain edge listener
  is unchanged.
- **The Cloudflare URL is https.** The scripts print `https://localhost:<EDGE_NGINX_PORT>`.
  Turn **Use HTTP/2 to origin** on and **Disable TLS certificate verification** on.
  An `http://` origin ignores the HTTP/2 setting.

## Upgrade

Run `./update.sh` on the control plane. It installs nginx and prints the port. Then, in
the tunnel public hostname for gRPC:

1. Set the URL to `https://localhost:<EDGE_NGINX_PORT>`.
2. Additional application settings, TLS: **Use HTTP/2 to origin** on.
3. **Disable TLS certificate verification** on.

Leave the plain listener (`EDGE_GRPC_ADDR`) as it is. nginx is the tunnel origin.
Reinstall agents only if they were enrolled before edge mode and still have no secret.

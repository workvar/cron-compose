#!/bin/sh
# Run by the package manager after files are unpacked. Creates the service user +
# data dir and registers the systemd unit. Idempotent.

set -e

USER=croncompose
DATA_DIR=/var/lib/croncompose

if ! id -u "$USER" >/dev/null 2>&1; then
  if command -v useradd >/dev/null 2>&1; then
    useradd --system --home-dir "$DATA_DIR" --shell /usr/sbin/nologin "$USER"
  elif command -v adduser >/dev/null 2>&1; then
    adduser -S -h "$DATA_DIR" -s /sbin/nologin "$USER"
  fi
fi

install -d -o "$USER" -g "$USER" -m 0700 "$DATA_DIR"
install -d -m 0755 /etc/croncompose

if [ -f /usr/share/croncompose/agent_sudoers.sh ]; then
  /usr/share/croncompose/agent_sudoers.sh "$USER" || true
fi

if command -v systemctl >/dev/null 2>&1; then
  systemctl daemon-reload || true
  echo "edit /etc/croncompose/agent.env then run: systemctl enable --now croncompose-agent"
fi

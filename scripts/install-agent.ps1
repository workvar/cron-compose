# CronCompose agent installer (Windows)
#
# The agent uses Unix process APIs (pty, systemd/launchd, /proc) and does not run
# on Windows. Install it on Linux or macOS instead:
#
#   curl -sSL https://github.com/workvar/cron-compose/releases/latest/download/install-agent.sh | `
#     sudo TOKEN=<token> AGENT_ENROLL_HTTP=https://<host>/api/v1 AGENT_GRPC_ADDR=<host>:9090 bash
#
# Baked release: __VERSION__

Write-Error "The CronCompose agent does not run on Windows. Use install-agent.sh on Linux or macOS."
exit 1

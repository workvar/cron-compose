# CronCompose v0.0.66

Deploys finish sooner and the run page keeps up with them. Output appears as it
is produced instead of sitting on "(waiting for output…)", and the quiet gaps
between steps are gone.

## Highlights

- **Live output that keeps up**: The agent groups script output and sends it
  about every 100ms instead of one message per line. Before, each line was
  written to disk and sent on its own, which on an SD card was slower than the
  build and could stall the installer on a full pipe.
- **Faster agent queue**: Concurrent writes share one disk sync, and a backlog
  is sent and cleared up to 256 messages at a time instead of one at a time.
- **Log storage off the hot path**: The control plane saves deploy logs in
  batches in the background instead of one insert per chunk on the agent's
  stream. A run is marked finished only after all of its logs are saved.
- **No more missing lines**: Live log events are now encoded as valid JSON, so
  lines with control characters are no longer dropped by the browser. The live
  stream holds 1024 pending events (was 64), and the final "done" event is never
  dropped, so a finished run no longer shows "running".
- **Lighter run page**: Live lines are applied about every 80ms instead of one
  re-render per line. The background refresh fetches only new lines
  (`GET /deploy-runs/:id/logs?after=<seq>`) instead of the whole log every 4s,
  and the page also refreshes the run status so it notices a finish on its own.
- **No silent ownership pass**: Deploys no longer change ownership across the
  whole project folder (every kept release and its `node_modules`) at start and
  after the switch. Only the top-level folders and the new release are changed,
  and files the deploy user already owns are skipped.
- **Background cleanup**: Pruned releases are moved into `<project>/.cc-trash`
  instantly and deleted in the background. Release size is measured while apps
  start instead of before.
- **Quicker start and health checks**: Health checks retry from 250ms and back
  off to 3s (was a flat 3s). Port readiness is checked every 200ms (was 500ms).
- **Visible cache saves**: Saving the install or build cache prints
  `saving install cache` and how long it took.

## Upgrade

Update the agent and the control plane together for the full effect. Either one
alone still helps, and nothing in the protocol or the database changed.

1. Pull this tag on the control plane and restart it.
2. Update agents from *Settings → Updates*, or let them auto-update.

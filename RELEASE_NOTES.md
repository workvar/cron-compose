# CronCompose v0.0.42

Tools installs stream live into the UI, deploys as a non-root user stop
inheriting root's Go cache, and the sidebar gains an Architecture map of the
runtime path.

## Highlights

- **Live Tools install/uninstall** — The agent sends stdout chunks over the
  gRPC stream while a package manager runs. The control plane forwards them as
  SSE, and the Tools page shows a real terminal and percent instead of waiting
  for a single finished event. Offline agents and reconnects keep pending tool
  work safe.
- **Deploy-as-user Go cache** — When `run_as` drops privileges, `GOCACHE` /
  `GOMODCACHE` / related vars are rewritten for that user's home. A root agent
  unit no longer hands `/root/.cache/go-build` to `pi` (or similar) and fails
  the build with a permission error.
- **Longer default deploy budget** — The default run timeout is 60 minutes
  (still capped at 2 hours). Multi-app installs on small hosts no longer die
  at the old 15-minute floor when `deploy_timeout` is unset.
- **Tools and layout polish** — Progressive tool list load with local cache,
  row progress, uninstall flow, wider server selectors, project cards on a
  server, and deploy edit using the create-style view.
- **Docs and Architecture** — Docs layout is closer to a reference site. A new
  Architecture tab (below Docs) embeds a runtime diagram: operator → web →
  control plane → mTLS agent → deploy → host workloads, with trust boundaries
  and detail in cards.

## Upgrade

Update the **agent**, **control plane**, and **web**. Agents need the new
`HostToolsEvent` chunk messages; the control plane and UI need the SSE path.
No new migration.

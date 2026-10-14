# CronCompose v0.0.61

When the connected branch moves ahead of the live deploy, the project page
offers an Update action.

## Highlights

- **Update deploy** — The project detail page checks the remote tip of the
  connected branch against the last successful deploy. If new commits landed,
  an **Update available** banner shows how far ahead the branch is (when the
  provider reports it) and an **Update deploy** button starts a manual run on
  that branch. Rechecks about once a minute.

## Upgrade

1. Restart the **control plane** and rebuild/restart the web UI.
2. Open a project whose connected branch has new commits — the banner appears
   when the tip differs from the live deploy SHA.
3. Agent update is **not** required for this release.

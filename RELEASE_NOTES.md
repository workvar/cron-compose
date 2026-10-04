# CronCompose v0.0.31

Deploy import now lets you choose which `croncompose.yml` in the repo to use,
shows the steps that file will run, and confirms before starting. Editing an
existing project also has searchable server and branch pickers.

## Highlights

- **Pick a croncompose.yml from the repo** — searchable list of
  `croncompose.yml` / `.yaml` files anywhere in the branch (not only the root).
- **Review before deploy** — Next shows the agent plan (preflight → clone →
  install → release → start → health) derived from the YAML, then Confirm.
- **Searchable server & branch selectors** — set the target machine on create
  and edit; GitHub/GitLab projects get a searchable branch picker.
- **APIs** — `GET /git/specs` lists candidate files; `GET /git/spec` fetches and
  parses one.

## Upgrade

Pull or rebuild the control plane and web UI. No agent or database migration is
required for this release.

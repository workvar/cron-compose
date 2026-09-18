# Deploy project blocks + browseable roots (Hybrid)

**Date:** 2026-09-18  
**Status:** Draft for review  
**Approach:** C — Hybrid (lazy browse + optional full tree)

## Problem

Import git only exposes a single shared root / language / install, plus checkboxes for
*detected* workspaces. Detection misses many sub-projects (e.g. `mobile`, `web`,
`backend`). Users need to pick arbitrary folders and configure each as its own deployable
app.

## Goals

- Configure **one or more project blocks** per import, each with its own root folder and
  build/runtime settings.
- **Browse** the repo’s directory tree to pick roots (not free-text only).
- Keep **detected workspaces** as quick picks.
- Optional **Load full tree** for searchable flat selection on large monorepos.
- Reuse existing `apps[]` on create — no schema migrations.

## Non-goals

- Changing how the agent clones the repo (still one clone path; apps are subdirs).
- Editing existing deploy projects’ app list in this change (Import wizard only; Settings
  env editor already works on `project.apps`).
- Recursive detection improvements beyond using existing inspect workspaces as chips.

## Current state

- `DeployApp` / `SpecApp`: `name`, `root`, `language`, `install`, `port`,
  `process_manager`, `env`.
- `POST /deploys` already accepts `apps[]`; empty apps → one default app from project
  fields.
- Inspect (`GET /git/inspect`) returns `workspaces`, language, install, root, etc.
- Wizard Build step uses one shared language/root/install and optional workspace
  checkboxes that only change which roots share those settings.

## API

### `GET /git/dirs`

Auth: same as other `/git/*` (session + git connection for `provider`).

Query:

| Param | Required | Description |
|-------|----------|-------------|
| `provider` | yes | `github` \| `gitlab` |
| `repo` | yes | `owner/name` or GitLab path with namespace |
| `branch` | no | defaults to repo default branch (resolve via provider if empty) |
| `path` | no | parent directory; empty = repo root |
| `recursive` | no | `1` / `true` → flat list of all directories under `path` (or whole repo if path empty), capped |

Response:

```json
{
  "path": "apps",
  "recursive": false,
  "truncated": false,
  "items": [
    { "name": "web", "path": "apps/web" },
    { "name": "api", "path": "apps/api" }
  ]
}
```

Rules:

- **Non-recursive:** only **directories** that are immediate children of `path`.
- **Recursive:** all directory paths under `path` (depth-first or sorted), max **2000**
  items; set `truncated: true` if capped.
- Paths are repo-relative, `/`-separated, no leading `./`. Repo root as a selectable
  target is represented as `"."` in the UI (not returned as a child of itself).
- Ignore `.git` and common junk (`.github` may still appear — user can pick or skip).
- 409 if provider not connected; 502 on provider API failure.

Implementation sketch:

- **GitHub non-recursive:** `GET /repos/{repo}/contents/{path}?ref=` → filter `type=dir`.
- **GitHub recursive:** existing `git/trees/{ref}?recursive=1` → filter `type=tree`,
  optional prefix filter by `path`.
- **GitLab non-recursive:** `repository/tree?path=&ref=&per_page=100` (no recursive).
- **GitLab recursive:** `repository/tree?recursive=true&path=&ref=` with pagination until
  cap.

### Unchanged

- `GET /git/inspect` — still seeds defaults and workspace chips.
- `POST /deploys` — body includes full `apps[]` from blocks; project-level
  `language` / `install_script` / `root_directory` / `port` / `process_manager` set from
  the **first** block for backward-compatible top-level fields / `croncompose.yml`.

## UI — Import wizard

### Draft model

Replace shared `language` / `root` / `install` / `port` / `processManager` /
`selectedApps` as the source of truth with:

```ts
type ProjectBlock = {
  id: string;           // client-only key
  name: string;
  root: string;         // "." or "apps/web"
  language: string;
  install: string;
  port: string;
  processManager: string;
};
```

Keep project-level: `provider`, `repo`, `inspect`, `serverId`, `branch`, `clonePath`,
`appEnv`.

On inspect success: one block seeded from detection (`root` from
`inspect.root_directory` or `.`, language/install from inspect, process hint from
inspect). Detected `workspaces` shown as chips on every block’s folder picker.

### Step 1 — Build (project blocks)

- Heading: configure apps to deploy from this repo.
- List of **panels** (blocks). Each panel:
  - Name (default: last segment of root, or repo name for `.`)
  - **Root folder** control:
    - Current path display + “Change”
    - Picker popover/panel: breadcrumb, list of child dirs (lazy `/git/dirs`), Up /
      Use this folder, workspace chips, **Load full tree** → searchable flat list
      (`recursive=1`), type-to-filter; still allow typing a path manually as escape hatch
  - Language, install script, PORT, process manager (moved from Runtime)
  - Remove (disabled when only one block)
- **+ Add project** — new empty block (`root` unset until picked; Continue disabled if any
  block lacks a root).

### Step 2 — Runtime (shared only)

- Agent server (SearchableSelect if consistent with rest of app)
- Branch
- Clone path on agent
- `AppEnvEditor` bound to apps derived from blocks (env keyed by app name)

### Step 3 — Review

- Repo, server, clone path
- For each block: name, root, language, install summary, port, process manager

### Submit

Build `apps` from blocks; set top-level project fields from `apps[0]`.

## Testing

- Go: `ListDirs` unit tests with httptest GitHub/GitLab fixtures (shallow + recursive +
  truncation).
- Web: helpers for normalizing roots, deriving block name from path, mapping blocks →
  `DeployApp[]`.
- Manual: monorepo with undetected folder; add two blocks; deploy creates two apps.

## Risks

| Risk | Mitigation |
|------|------------|
| Huge trees / rate limits | Cap 2000; lazy default; recursive opt-in |
| GitLab pagination incomplete | Document truncate; prefer lazy browse |
| Duplicate roots | Warn or block Continue if two blocks share the same root |
| Name collisions in AppEnvEditor | Auto-rename (`web`, `web-2`) when names clash |

## Out of scope follow-ups

- Re-inspect a chosen subdirectory for language/install suggestions.
- Edit app roots on an existing project’s detail page with the same picker.

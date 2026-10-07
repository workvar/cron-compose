# CronCompose v0.0.35

Shared environment variables apply to every process, and the framework picker
now covers Next.js, NestJS, React, Go, .NET/C#, and more — selecting one fills
build script, run command, port, and process manager. Repo detection recognizes
those stacks too, and framework ids map to the right clone-path runtime.

## Highlights

- **Shared env** — Environment tab has a “Shared (all processes)” section.
  Values are stored as project-level `env` (same as top-level `env:` in
  `croncompose.yml`). Process-level vars override the same key. Secrets stay
  on individual processes.
- **Framework presets** — Next.js, NestJS, React, Vue, Nuxt, Remix, SvelteKit,
  Astro, Express, Go, Rust, C# / ASP.NET, .NET, FastAPI, Django, Flask, Spring
  Boot, Rails, Laravel, and base runtimes. Picking one seeds install, run,
  port, and process manager.
- **Smarter detection** — control-plane Detect returns framework ids (e.g.
  `nextjs`, `nestjs`, `dotnet`, `fastapi`) from `package.json`, `.csproj`,
  requirements, and similar markers.
- **Runtime mapping** — framework ids resolve to clone paths and agent
  fallbacks (`nextjs` → `/opt/apps/node/…`, `csharp` → `/opt/apps/dotnet/…`).
- **Cleanup** — removed unused `ProjectBlockCard`; deploy-steps and agent
  start fallbacks use the shared runtime map instead of long switch lists.

## Upgrade

Rebuild and restart the **control plane**, **web UI**, and **agent**. No new
migration. Existing projects keep per-app env; add shared vars from the project
Environment panel or a top-level `env:` in `croncompose.yml`.

import type { ProjectBlock } from "./project-blocks";

/**
 * Curated framework / language presets for the deploy wizard. Picking one
 * fills install, run, port, and process manager with sensible defaults.
 * `runtime` is the clone-path / agent bucket (node, go, python, …).
 */
export type FrameworkPreset = {
  id: string;
  label: string;
  /** Simple Icons slug when it differs from id. */
  icon?: string;
  runtime: string;
  install: string;
  run: string;
  /**
   * Shell run after install inside the release to drop source/caches. Keep in
   * sync with control-plane DefaultCleanup and agent defaultCleanup.
   */
  cleanup: string;
  port: string;
  processManager: string;
};

export const FRAMEWORKS: FrameworkPreset[] = [
  // JavaScript / TypeScript
  {
    id: "nextjs",
    label: "Next.js",
    icon: "nextdotjs",
    runtime: "node",
    install: "npm ci && npm run build",
    run: "npm start",
    port: "3000",
    cleanup: "rm -rf .git .github node_modules/.cache .next/cache .turbo",
    processManager: "pm2",
  },
  {
    id: "nestjs",
    label: "NestJS",
    icon: "nestjs",
    runtime: "node",
    install: "npm ci && npm run build",
    run: "node dist/main.js",
    port: "3000",
    cleanup: "rm -rf .git .github src test node_modules/.cache .turbo *.md",
    processManager: "pm2",
  },
  {
    id: "react",
    label: "React (Vite)",
    icon: "react",
    runtime: "node",
    install: "npm ci && npm run build",
    run: "npx --yes serve -s dist -l 3000",
    port: "3000",
    cleanup: "rm -rf .git .github src node_modules/.cache .turbo *.md",
    processManager: "pm2",
  },
  {
    id: "vue",
    label: "Vue (Vite)",
    icon: "vuedotjs",
    runtime: "node",
    install: "npm ci && npm run build",
    run: "npx --yes serve -s dist -l 3000",
    port: "3000",
    cleanup: "rm -rf .git .github src node_modules/.cache .turbo *.md",
    processManager: "pm2",
  },
  {
    id: "nuxt",
    label: "Nuxt",
    icon: "nuxtdotjs",
    runtime: "node",
    install: "npm ci && npm run build",
    run: "node .output/server/index.mjs",
    port: "3000",
    cleanup: "rm -rf .git .github src node_modules/.cache .turbo *.md",
    processManager: "pm2",
  },
  {
    id: "remix",
    label: "Remix",
    icon: "remix",
    runtime: "node",
    install: "npm ci && npm run build",
    run: "npm start",
    port: "3000",
    cleanup: "rm -rf .git .github src node_modules/.cache .turbo *.md",
    processManager: "pm2",
  },
  {
    id: "sveltekit",
    label: "SvelteKit",
    icon: "svelte",
    runtime: "node",
    install: "npm ci && npm run build",
    run: "node build",
    port: "3000",
    cleanup: "rm -rf .git .github src node_modules/.cache .turbo *.md",
    processManager: "pm2",
  },
  {
    id: "astro",
    label: "Astro",
    icon: "astro",
    runtime: "node",
    install: "npm ci && npm run build",
    run: "node ./dist/server/entry.mjs",
    port: "4321",
    cleanup: "rm -rf .git .github src node_modules/.cache .turbo *.md",
    processManager: "pm2",
  },
  {
    id: "express",
    label: "Express",
    icon: "express",
    runtime: "node",
    install: "npm ci",
    run: "npm start",
    port: "3000",
    cleanup: "rm -rf .git .github node_modules/.cache *.md",
    processManager: "pm2",
  },
  {
    id: "node",
    label: "Node.js",
    icon: "nodedotjs",
    runtime: "node",
    install: "npm ci",
    run: "npm start",
    port: "3000",
    cleanup: "rm -rf .git .github node_modules/.cache *.md",
    processManager: "pm2",
  },
  {
    id: "typescript",
    label: "TypeScript",
    icon: "typescript",
    runtime: "node",
    install: "npm ci && npm run build",
    run: "npm start",
    port: "3000",
    cleanup: "rm -rf .git .github node_modules/.cache *.md",
    processManager: "pm2",
  },
  {
    id: "bun",
    label: "Bun",
    icon: "bun",
    runtime: "bun",
    install: "bun install && bun run build",
    run: "bun start",
    port: "3000",
    cleanup: "rm -rf .git .github node_modules/.cache *.md",
    processManager: "pm2",
  },
  {
    id: "deno",
    label: "Deno",
    icon: "deno",
    runtime: "deno",
    install: "deno cache main.ts",
    run: "deno run -A main.ts",
    port: "8000",
    cleanup: "rm -rf .git .github *.md",
    processManager: "systemd",
  },

  // Go / Rust / systems
  {
    id: "go",
    label: "Go",
    icon: "go",
    runtime: "go",
    install: "go build -o app .",
    run: "./app",
    port: "8080",
    cleanup: "rm -rf .git .github *.md",
    processManager: "systemd",
  },
  {
    id: "rust",
    label: "Rust",
    icon: "rust",
    runtime: "rust",
    install: "cargo build --release",
    run: "./target/release/app",
    port: "8080",
    cleanup: "rm -rf .git .github src target/debug *.md",
    processManager: "systemd",
  },

  // .NET / C#
  {
    id: "dotnet",
    label: ".NET",
    icon: "dotnet",
    runtime: "dotnet",
    install: "dotnet restore && dotnet publish -c Release -o out",
    run: "bash -lc 'dotnet out/*.dll --urls http://0.0.0.0:8080'",
    port: "8080",
    cleanup: "rm -rf .git .github *.md",
    processManager: "systemd",
  },
  {
    id: "csharp",
    label: "C# / ASP.NET",
    icon: "csharp",
    runtime: "dotnet",
    install: "dotnet restore && dotnet publish -c Release -o out",
    run: "bash -lc 'dotnet out/*.dll --urls http://0.0.0.0:8080'",
    port: "8080",
    cleanup: "rm -rf .git .github *.md",
    processManager: "systemd",
  },

  // Python
  {
    id: "fastapi",
    label: "FastAPI",
    icon: "fastapi",
    runtime: "python",
    install:
      "python3 -m venv .venv && . .venv/bin/activate && pip install -r requirements.txt",
    run: ".venv/bin/uvicorn main:app --host 0.0.0.0 --port 8000",
    port: "8000",
    cleanup: "rm -rf .git .github __pycache__ .pytest_cache *.md",
    processManager: "systemd",
  },
  {
    id: "django",
    label: "Django",
    icon: "django",
    runtime: "python",
    install:
      "python3 -m venv .venv && . .venv/bin/activate && pip install -r requirements.txt",
    run: ".venv/bin/gunicorn myproject.wsgi:application --bind 0.0.0.0:8000",
    port: "8000",
    cleanup: "rm -rf .git .github __pycache__ .pytest_cache *.md",
    processManager: "systemd",
  },
  {
    id: "flask",
    label: "Flask",
    icon: "flask",
    runtime: "python",
    install:
      "python3 -m venv .venv && . .venv/bin/activate && pip install -r requirements.txt",
    run: ".venv/bin/gunicorn app:app --bind 0.0.0.0:8000",
    port: "8000",
    cleanup: "rm -rf .git .github __pycache__ .pytest_cache *.md",
    processManager: "systemd",
  },
  {
    id: "python",
    label: "Python",
    icon: "python",
    runtime: "python",
    install:
      "python3 -m venv .venv && . .venv/bin/activate && pip install -r requirements.txt",
    run: "python3 -m app",
    port: "8000",
    cleanup: "rm -rf .git .github __pycache__ .pytest_cache *.md",
    processManager: "systemd",
  },

  // JVM
  {
    id: "spring",
    label: "Spring Boot",
    icon: "springboot",
    runtime: "java",
    install: "./gradlew bootJar || mvn -q -DskipTests package",
    run: "bash -lc 'java -jar target/*.jar || java -jar build/libs/*.jar'",
    port: "8080",
    cleanup: "rm -rf .git .github src *.md",
    processManager: "systemd",
  },
  {
    id: "java",
    label: "Java",
    icon: "openjdk",
    runtime: "java",
    install: "mvn -q -DskipTests package || ./gradlew build",
    run: "bash -lc 'java -jar target/*.jar || java -jar build/libs/*.jar'",
    port: "8080",
    cleanup: "rm -rf .git .github src *.md",
    processManager: "systemd",
  },
  {
    id: "kotlin",
    label: "Kotlin",
    icon: "kotlin",
    runtime: "java",
    install: "./gradlew build",
    run: "bash -lc 'java -jar build/libs/*.jar'",
    port: "8080",
    cleanup: "rm -rf .git .github src *.md",
    processManager: "systemd",
  },

  // Ruby / PHP / Elixir
  {
    id: "rails",
    label: "Ruby on Rails",
    icon: "rubyonrails",
    runtime: "ruby",
    install: "bundle install && bundle exec rake assets:precompile",
    run: "bundle exec puma -C config/puma.rb",
    port: "3000",
    cleanup: "rm -rf .git .github tmp/cache log/*.log *.md",
    processManager: "systemd",
  },
  {
    id: "ruby",
    label: "Ruby",
    icon: "ruby",
    runtime: "ruby",
    install: "bundle install",
    run: "bundle exec ruby app.rb",
    port: "3000",
    cleanup: "rm -rf .git .github tmp/cache log/*.log *.md",
    processManager: "systemd",
  },
  {
    id: "laravel",
    label: "Laravel",
    icon: "laravel",
    runtime: "php",
    install: "composer install --no-dev --optimize-autoloader",
    run: "php artisan serve --host=0.0.0.0 --port=8000",
    port: "8000",
    cleanup: "rm -rf .git .github tests *.md",
    processManager: "systemd",
  },
  {
    id: "php",
    label: "PHP",
    icon: "php",
    runtime: "php",
    install: "composer install --no-dev",
    run: "php -S 0.0.0.0:8000 -t public",
    port: "8000",
    cleanup: "rm -rf .git .github tests *.md",
    processManager: "systemd",
  },
  {
    id: "elixir",
    label: "Elixir",
    icon: "elixir",
    runtime: "elixir",
    install: "mix deps.get && mix compile",
    run: "mix phx.server",
    port: "4000",
    cleanup: "rm -rf .git .github *_test.exs *.md",
    processManager: "systemd",
  },

  // Containers
  {
    id: "docker",
    label: "Docker Compose",
    icon: "docker",
    runtime: "docker",
    install: "docker compose pull",
    run: "",
    port: "",
    cleanup: "",
    processManager: "docker",
  },
];

/** Base runtimes that own a clone-path setting (not every framework). */
export const RUNTIME_LANGUAGES: Array<{ value: string; label: string }> = [
  { value: "node", label: "Node.js" },
  { value: "python", label: "Python" },
  { value: "go", label: "Go" },
  { value: "rust", label: "Rust" },
  { value: "dotnet", label: ".NET" },
  { value: "ruby", label: "Ruby" },
  { value: "php", label: "PHP" },
  { value: "elixir", label: "Elixir" },
  { value: "java", label: "Java" },
  { value: "docker", label: "Docker" },
  { value: "deno", label: "Deno" },
  { value: "bun", label: "Bun" },
];

const byId = new Map(FRAMEWORKS.map((f) => [f.id, f]));

export function frameworkPreset(id: string): FrameworkPreset | undefined {
  return byId.get((id || "").trim().toLowerCase());
}

/** Maps a framework id (nextjs, csharp, …) to its clone-path / agent runtime. */
export function runtimeLanguage(id: string): string {
  const preset = frameworkPreset(id);
  if (preset) return preset.runtime;
  const lang = (id || "").trim().toLowerCase();
  switch (lang) {
    case "javascript":
    case "typescript":
    case "nodejs":
      return "node";
    case "golang":
      return "go";
    case "aspnet":
    case "aspnetcore":
      return "dotnet";
    default:
      return lang || "unknown";
  }
}

/**
 * Fields to apply when the operator picks a framework. Always fills deploy
 * scripts from the preset so changing framework re-seeds install/run/port/pm.
 */
export function fieldsForFramework(id: string): Partial<ProjectBlock> | null {
  const p = frameworkPreset(id);
  if (!p) return null;
  return {
    language: p.id,
    install: p.install,
    run: p.run,
    port: p.port,
    processManager: p.processManager,
    cleanup: p.cleanup,
    autoDetect: false,
  };
}

/** Start command only — used when detection/spec leaves run empty. */
export function defaultRunForFramework(id: string): string {
  const direct = frameworkPreset(id)?.run;
  if (direct) return direct;
  // Aliases like golang → go, javascript → node.
  const runtime = runtimeLanguage(id);
  if (runtime !== (id || "").trim().toLowerCase()) {
    return frameworkPreset(runtime)?.run ?? "";
  }
  return "";
}

import type { Metadata } from "next";
import Link from "next/link";
import CopyButton from "@/components/CopyButton";
import { SPEC_TEMPLATE } from "@/lib/deploy-spec";
import "./docs.css";

export const metadata: Metadata = {
  title: "croncompose.yml reference · CronCompose",
  description: "How to write a croncompose.yml file that tells CronCompose how to build and run your app.",
};

function Code({ children, title }: { children: string; title?: string }) {
  return (
    <div className="doc-code">
      <div className="doc-code-bar">
        <span>{title || "yaml"}</span>
        <CopyButton value={children} className="light" />
      </div>
      <pre>{children}</pre>
    </div>
  );
}

type Row = { key: string; type: string; def?: string; desc: React.ReactNode };

function KeyTable({ rows }: { rows: Row[] }) {
  return (
    <div className="doc-table-wrap">
      <table className="doc-table">
        <thead>
          <tr><th>Key</th><th>Type</th><th>Default</th><th>What it does</th></tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.key}>
              <td><code>{r.key}</code></td>
              <td>{r.type}</td>
              <td>{r.def ? <code>{r.def}</code> : "—"}</td>
              <td>{r.desc}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

const MINIMAL = `version: 1
install: npm ci && npm run build
process_manager: pm2
`;

const TOP_KEYS: Row[] = [
  { key: "version", type: "number", def: "1", desc: "Schema version. Only 1 exists today. A newer number is rejected so an old CronCompose never half-reads a new file." },
  { key: "name", type: "string", def: "repo name", desc: "Project name shown in CronCompose." },
  { key: "repo", type: "string", desc: <>What to clone: <code>owner/name</code>, an https URL, or a <code>git@</code> URL. Required when you paste the file. Can be left out when the file lives in the repo you import.</> },
  { key: "provider", type: "github | gitlab", def: "from repo URL, else github", desc: "Only needed with a bare owner/name on GitLab." },
  { key: "branch", type: "string", def: "repo default", desc: "Branch to deploy on the first run, and the branch watched when redeploy_on includes branch." },
  { key: "server", type: "string", desc: "Server name or id to deploy to. Pre-selected in the importer when it matches; you can still change it." },
  { key: "clone_path", type: "absolute path", def: "/opt/apps/<language>/<repo>", desc: "Folder on the server releases land in (usually under /opt). Each process runs from its app folder under current/ after activation." },
  { key: "install", type: "shell command", def: "detected", desc: "Build script. Runs in the app root after every clone. Install deps, compile, migrate. Left out, CronCompose uses what it detects." },
  { key: "run", type: "shell command", def: "from language", desc: <>Start command after the release is live. Working directory is the activated app folder under the deploy path — for a Go binary that is often just <code>./app</code>.</> },
  { key: "root", type: "relative path", def: ".", desc: "App folder inside the repo, for single-app repos. Use apps for more than one." },
  { key: "language", type: "string", def: "detected", desc: <>Framework or language id — e.g. <code>nextjs nestjs react go csharp dotnet python fastapi</code>, or a base runtime like <code>node</code>. Picks clone-path defaults and seeds install/run when those fields are omitted.</> },
  { key: "port", type: "1–65535", desc: <>Exported as <code>PORT</code> and used as the default health-check port. Leave unset to let the agent detect the listening port and show it in the web UI.</> },
  { key: "process_manager", type: "none | pm2 | systemd | docker", def: "none", desc: "What keeps the app running after install. See Process managers." },
  { key: "redeploy_on", type: "list", def: '["branch"]', desc: <>Git events that redeploy: <code>branch</code> (push to branch), <code>tag</code> (tag push), <code>release</code> (published release). Requires a connected GitHub or GitLab account.</> },
  { key: "env", type: "map", desc: "Non-secret environment variables, shared by every app." },
  { key: "health", type: "object", desc: "Optional HTTP probe that must pass before a deploy counts as successful." },
  { key: "deploy_timeout", type: "seconds", def: "3600", desc: "Budget for a whole deploy. The agent caps it at 2 hours." },
  { key: "auto_rollback", type: "boolean", def: "false", desc: "When a deploy fails, redeploy the last commit that worked." },
  { key: "apps", type: "list", desc: "Several apps from one repo (monorepo). Each item takes the keys below." },
];

const APP_KEYS: Row[] = [
  { key: "name", type: "string", def: "folder name", desc: "Unique per project. Used for pm2 and systemd unit names and the deploy folder label." },
  { key: "root", type: "relative path", def: ".", desc: <>Folder inside the repo. No leading <code>/</code>, no <code>..</code>. Unique per project.</> },
  { key: "install", type: "shell command", def: "top-level, else detected", desc: "Build script for this app." },
  { key: "run", type: "shell command", def: "top-level, else from language", desc: "Start command for this app (cwd = activated deploy folder)." },
  { key: "language", type: "string", def: "top-level language", desc: "Overrides the top-level value." },
  { key: "port", type: "1–65535", def: "top-level if only one app", desc: <>This app&apos;s <code>PORT</code>.</> },
  { key: "process_manager", type: "string", def: "top-level value", desc: "Overrides the top-level value." },
  { key: "env", type: "map or list", desc: <>App-only variables. Wins over top-level <code>env</code> on the same key.</> },
];

const HEALTH_KEYS: Row[] = [
  { key: "health.path", type: "string", desc: <>Required. Must start with <code>/</code>. Probed on <code>127.0.0.1</code> every 3s.</> },
  { key: "health.port", type: "1–65535", def: "app port", desc: "Port to probe." },
  { key: "health.timeout", type: "seconds", def: "60", desc: "How long the app has to answer 2xx before the deploy fails." },
];

const EXAMPLES: { id: string; title: string; lead: React.ReactNode; yaml: string }[] = [
  {
    id: "node",
    title: "Next.js / Node with pm2",
    lead: <>pm2 runs <code>npm start</code>, or your <code>ecosystem.config.js</code> when the repo has one.</>,
    yaml: `version: 1
install: npm ci && npm run build
run: npm start
port: 3000
process_manager: pm2
redeploy_on: [branch]
env:
  NODE_ENV: production
health:
  path: /api/health
auto_rollback: true
`,
  },
  {
    id: "python",
    title: "Python service with systemd",
    lead: "systemd runs the run script from the activated app folder, as a user unit that restarts on failure.",
    yaml: `version: 1
install: python3 -m venv .venv && . .venv/bin/activate && pip install -r requirements.txt
run: python3 -m app
port: 8000
process_manager: systemd
env:
  LOG_LEVEL: info
`,
  },
  {
    id: "go",
    title: "Go binary",
    lead: "Build writes the binary into the release; run is just ./app because the process starts with that folder as cwd.",
    yaml: `version: 1
install: go build -o app ./cmd/server
run: ./app
port: 8080
process_manager: systemd
redeploy_on: [branch, tag, release]
health:
  path: /healthz
`,
  },
  {
    id: "docker",
    title: "Docker Compose",
    lead: "The agent runs docker compose up -d on the compose file in the app folder.",
    yaml: `version: 1
language: docker
install: docker compose pull
process_manager: docker
`,
  },
  {
    id: "monorepo",
    title: "Monorepo with two apps",
    lead: "One clone, one deploy, two running apps. Top-level values are defaults for every app.",
    yaml: `version: 1
repo: acme/platform
branch: main
server: prod-1
process_manager: pm2
env:
  NODE_ENV: production        # shared by both apps
apps:
  - name: web
    root: apps/web
    install: pnpm install --frozen-lockfile && pnpm --filter web build
    port: 3000
  - name: api
    root: services/api
    language: go
    install: go build -o app .
    process_manager: systemd
    port: 8080
    env:
      LOG_LEVEL: debug
`,
  },
  {
    id: "script",
    title: "Clone and run a script only",
    lead: "No process manager: the install command is the whole deploy. Good for static sites served by nginx, or one-off setup.",
    yaml: `version: 1
repo: https://github.com/acme/site
install: npm ci && npm run build && rsync -a --delete dist/ /var/www/site/
`,
  },
];

const SECTIONS = [
  ["quick-start", "Quick start"],
  ["how-it-works", "How it is used"],
  ["reference", "Top-level keys"],
  ["apps", "apps"],
  ["env", "Environment variables"],
  ["health", "Health checks"],
  ["process-managers", "Process managers"],
  ["examples", "Examples"],
  ["validation", "Validation"],
  ["ci", "Deploy from CI"],
] as const;

export default function DocsPage() {
  return (
    <div className="docs">
      <nav className="docs-toc" aria-label="On this page">
        <div className="docs-toc-label">croncompose.yml</div>
        {SECTIONS.map(([id, label]) => (
          <a key={id} href={`#${id}`}>{label}</a>
        ))}
        <div className="docs-toc-label" style={{ marginTop: 18 }}>Use it</div>
        <Link href="/deploys/new">Import a project →</Link>
      </nav>

      <article className="docs-body">
        <header className="docs-hero">
          <p className="docs-eyebrow">Get setup</p>
          <h1>croncompose.yml reference</h1>
          <p className="docs-lead">
            Find the guides and fields you need to build, run, and redeploy with CronCompose.
          </p>
          <div className="docs-cards">
            <a className="docs-card" href="#quick-start">
              <span className="docs-card-icon" aria-hidden>▶</span>
              <strong>Quick start</strong>
              <span>Drop a minimal file in the repo root and import the project.</span>
            </a>
            <a className="docs-card" href="#reference">
              <span className="docs-card-icon" aria-hidden>{"{}"}</span>
              <strong>Key reference</strong>
              <span>Every top-level field, default, and what it changes on deploy.</span>
            </a>
            <a className="docs-card" href="#examples">
              <span className="docs-card-icon" aria-hidden>◇</span>
              <strong>Examples</strong>
              <span>Next.js, Go, monorepos, and script-only deploys.</span>
            </a>
            <Link className="docs-card" href="/deploys/new">
              <span className="docs-card-icon" aria-hidden>⇢</span>
              <strong>Import a project</strong>
              <span>Open Deploy and let CronCompose fill the form from your file.</span>
            </Link>
          </div>
        </header>

        <div className="docs-frameworks" aria-label="Common stacks">
          <div className="docs-framework">
            <div>
              <strong>Next.js / Node</strong>
              <span>pm2 or systemd with <code>npm ci && npm run build</code>.</span>
            </div>
          </div>
          <div className="docs-framework">
            <div>
              <strong>Go</strong>
              <span>Build a binary, run it under systemd with a health path.</span>
            </div>
          </div>
          <div className="docs-framework">
            <div>
              <strong>Monorepo apps</strong>
              <span>Use <code>apps:</code> for web + API from one repository.</span>
            </div>
          </div>
          <div className="docs-framework">
            <div>
              <strong>Docker Compose</strong>
              <span>Point <code>process_manager: docker</code> at your compose file.</span>
            </div>
          </div>
        </div>

        <h2 id="quick-start">Quick start</h2>
        <ol className="docs-steps">
          <li>Create <code>croncompose.yml</code> at the root of your repo. This is the smallest useful file:</li>
        </ol>
        <Code title="croncompose.yml">{MINIMAL}</Code>
        <ol className="docs-steps" start={2}>
          <li>Commit and push it.</li>
          <li>
            In CronCompose open <Link href="/deploys/new">Deploy → New project</Link> and click <strong>Import</strong> on the
            repo. You land on Configure with everything filled in. Pick a server if the file did not name one, then click{" "}
            <strong>Deploy</strong>.
          </li>
        </ol>
        <p>A fuller starting point with the keys most projects use:</p>
        <Code title="croncompose.yml">{SPEC_TEMPLATE}</Code>

        <h2 id="how-it-works">How it is used</h2>
        <ul>
          <li>
            <strong>From the repo.</strong> When you import a repo, CronCompose looks for{" "}
            <code>croncompose.yml</code>, <code>croncompose.yaml</code>, <code>.croncompose.yml</code> or{" "}
            <code>.croncompose.yaml</code> at the root of the branch, in that order, and uses the first one it finds.
          </li>
          <li>
            <strong>Pasted or uploaded.</strong> On the New project page, paste the file or upload it under{" "}
            <em>Deploy from croncompose.yml</em>. The file must then include <code>repo</code>. Public repos need no
            GitHub or GitLab connection.
          </li>
          <li>
            <strong>The file overrides detection.</strong> Anything you leave out (language, install command) is
            detected from the repo, the same way it is without a file. Write only what you want to pin.
          </li>
          <li>
            <strong>The file fills the form; the form wins.</strong> Anything you change on Configure is what gets deployed.
            After that, settings live in CronCompose and you edit them on the project page.
          </li>
          <li>
            <strong>Your file is never overwritten.</strong> Without a file, CronCompose commits one for you (plus a CI
            trigger). If the repo already has one, only the CI trigger is added.
          </li>
          <li>
            <strong>Export any time.</strong> The Configure page has <em>Export as croncompose.yml</em>, which turns
            whatever you set up by hand into a file you can commit.
          </li>
        </ul>

        <h2 id="reference">Top-level keys</h2>
        <p>Every key is optional. Unknown keys are reported as warnings and ignored, so a typo never blocks a deploy silently.</p>
        <KeyTable rows={TOP_KEYS} />

        <h2 id="build-run">Build &amp; run</h2>
        <p>
          <code>install</code> is the build script: it runs after clone inside the app&apos;s repo folder (deps, compile,
          migrate). Artifacts stay in that release under the target path — usually something like{" "}
          <code>/opt/apps/&lt;language&gt;/&lt;project&gt;</code>. When the release is activated, the process manager starts
          with the app folder as the working directory, so <code>run</code> is typically just the binary or start command
          relative to that folder (for example <code>./app</code> for Go). Leave <code>port</code> blank and the agent
          detects the listening port and exposes it in the web interface.
        </p>

        <h2 id="apps">apps</h2>
        <p>
          Use <code>apps</code> when one repo holds more than one thing to run. Each app is built in its own folder and
          started on its own. Any top-level <code>install</code>, <code>run</code>, <code>language</code>,{" "}
          <code>process_manager</code> and <code>env</code> act as defaults.
        </p>
        <KeyTable rows={APP_KEYS} />

        <h2 id="env">Environment variables</h2>
        <p>Write env as a map. Inside <code>apps</code>, a list of <code>{"{key, value}"}</code> pairs also works (that is what CronCompose writes back):</p>
        <Code>{`env:
  NODE_ENV: production
  PUBLIC_URL: "https://shop.example.com"

apps:
  - name: api
    env:
      - key: LOG_LEVEL
        value: info`}</Code>
        <div className="docs-callout warn">
          <strong>Keep secrets out of this file.</strong> It lives in git. Add passwords, tokens and API keys on the
          Configure page as <em>sensitive</em> variables instead: CronCompose encrypts them and never writes them back to the
          repo. Keys that look like secrets (<code>*_TOKEN</code>, <code>*_PASSWORD</code>, <code>*_API_KEY</code>…)
          get a warning when they have a value in the file.
        </div>
        <p>Quote values YAML would otherwise read as something else: <code>&quot;true&quot;</code>, <code>&quot;0755&quot;</code>, <code>&quot;yes&quot;</code>, or anything with <code>: </code> or <code>#</code> in it.</p>

        <h2 id="health">Health checks</h2>
        <p>
          Without a health check, a deploy succeeds as soon as <code>install</code> exits 0, even if the app then crashes
          on boot. With one, the deploy only succeeds once the app answers 2xx on the path, and{" "}
          <code>auto_rollback</code> can catch the crash.
        </p>
        <KeyTable rows={HEALTH_KEYS} />

        <h2 id="process-managers">Process managers</h2>
        <div className="doc-table-wrap">
          <table className="doc-table">
            <thead><tr><th>Value</th><th>What the agent does after install</th></tr></thead>
            <tbody>
              <tr><td><code>none</code></td><td>Nothing. The install command is the whole deploy.</td></tr>
              <tr><td><code>pm2</code></td><td>Starts your <code>ecosystem.config.js</code> if the app has one. Otherwise: Node runs <code>npm start</code>, Go runs <code>./app</code>, Python runs <code>python3 -m app</code>.</td></tr>
              <tr><td><code>systemd</code></td><td>Writes a user unit (<code>~/.config/systemd/user/&lt;name&gt;.service</code>) with the same start command, <code>Restart=on-failure</code>, and your env, then enables it.</td></tr>
              <tr><td><code>docker</code></td><td>Runs <code>docker compose up -d</code> on <code>docker-compose.yml</code> / <code>compose.yml</code> in the app folder.</td></tr>
            </tbody>
          </table>
        </div>

        <h2 id="examples">Examples</h2>
        {EXAMPLES.map((ex) => (
          <section key={ex.id} id={`example-${ex.id}`} className="docs-example">
            <h3>{ex.title}</h3>
            <p>{ex.lead}</p>
            <Code title="croncompose.yml">{ex.yaml}</Code>
          </section>
        ))}

        <h2 id="validation">Validation</h2>
        <p>CronCompose checks the file before anything deploys and lists every problem at once. Errors stop the import; warnings do not.</p>
        <div className="doc-table-wrap">
          <table className="doc-table">
            <thead><tr><th>Message</th><th>Fix</th></tr></thead>
            <tbody>
              <tr><td>not valid YAML</td><td>Usually indentation. Use spaces, not tabs, and two per level.</td></tr>
              <tr><td>version N is newer than this CronCompose understands</td><td>Update CronCompose, or set <code>version: 1</code>.</td></tr>
              <tr><td>process_manager must be one of …</td><td>Use <code>none</code>, <code>pm2</code>, <code>systemd</code> or <code>docker</code>.</td></tr>
              <tr><td>root … points outside the repo / must be relative</td><td>Write <code>apps/web</code>, not <code>/apps/web</code> or <code>../web</code>.</td></tr>
              <tr><td>two apps are named / use root …</td><td>Give each app its own <code>name</code> and <code>root</code>.</td></tr>
              <tr><td>clone_path must be an absolute path</td><td>Start it with <code>/</code>.</td></tr>
              <tr><td>health.path must start with /</td><td><code>/healthz</code>, not <code>healthz</code>.</td></tr>
              <tr><td>repo host … is not supported</td><td>Only github.com and gitlab.com URLs are recognised.</td></tr>
              <tr><td>unknown key ignored (warning)</td><td>Check the spelling against the tables above.</td></tr>
            </tbody>
          </table>
        </div>

        <h2 id="ci">Deploy from CI</h2>
        <p>
          With a GitHub or GitLab connection, CronCompose installs a webhook for the events in{" "}
          <code>redeploy_on</code> (branch push, tag push, and/or published release) and also commits a small CI job.
          To trigger a deploy yourself from any CI, call the project&apos;s run endpoint with the deploy token shown once
          after the first deploy:
        </p>
        <Code title="shell">{`curl -fsS -X POST "$CRONCOMPOSE_URL/api/deploys/$PROJECT_ID/runs" \\
  -H "Authorization: Bearer $CRONCOMPOSE_TOKEN" \\
  -H "content-type: application/json" \\
  -d '{"trigger":"api","branch":"main","commit":"'"$GIT_SHA"'"}'`}</Code>
        <p className="subtle">The same commit is never deployed twice when both the webhook and CI fire.</p>
      </article>
    </div>
  );
}

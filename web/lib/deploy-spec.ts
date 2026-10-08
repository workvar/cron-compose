// croncompose.yml <-> the import form. The control plane parses and validates the
// file (POST /deploys/spec/validate, GET /git/inspect); this module only maps the
// parsed result onto the form, and renders the form back out as YAML for "Export".
import type { DeployEnvVar, DeploySpec, Server } from "./types";
import { frameworkPreset } from "./frameworks";
import {
  defaultRunForLanguage,
  type ProjectBlock,
  type RedeployMode,
  DEFAULT_REDEPLOY_ON,
} from "./project-blocks";

export type AdvancedSettings = {
  healthPath: string;
  healthPort: string;
  healthTimeout: string;
  deployTimeout: string;
  autoRollback: boolean;
};

export const emptyAdvanced: AdvancedSettings = {
  healthPath: "",
  healthPort: "",
  healthTimeout: "",
  deployTimeout: "",
  autoRollback: false,
};

/** What a croncompose.yml fills in on the Configure form. */
export type SpecDraft = {
  name?: string;
  branch?: string;
  clonePath?: string;
  serverId?: string;
  blocks: ProjectBlock[];
  /** Top-level env shared by every process; per-app values override on the same key. */
  globalEnv: DeployEnvVar[];
  appEnv: Record<string, DeployEnvVar[]>;
  advanced: AdvancedSettings;
  redeployOn: RedeployMode[];
};

/** Plain map for POST/PATCH project.env (shared, non-secret). */
export function envVarsToRecord(vars: DeployEnvVar[]): Record<string, string> {
  const out: Record<string, string> = {};
  for (const v of vars) {
    const k = v.key.trim();
    if (!k || v.sensitive) continue;
    out[k] = v.value ?? "";
  }
  return out;
}

export function recordToEnvVars(m: Record<string, string> | undefined): DeployEnvVar[] {
  return Object.entries(m || {}).map(([key, value]) => ({
    key,
    value,
    sensitive: false,
  }));
}

function normalizeRedeployOn(raw?: string[]): RedeployMode[] {
  const allowed = new Set<RedeployMode>(["branch", "tag", "release"]);
  const out = (raw || []).filter((m): m is RedeployMode => allowed.has(m as RedeployMode));
  return out.length ? out : [...DEFAULT_REDEPLOY_ON];
}

function blockId(): string {
  return `b-${Math.random().toString(36).slice(2, 10)}`;
}

function lastSegment(p: string): string {
  return p.split("/").filter(Boolean).pop() || "";
}

/** Finds a server by id, then by name (case-insensitive). "" when none matches. */
export function matchServer(servers: Pick<Server, "id" | "name">[], ref?: string): string {
  const want = (ref || "").trim();
  if (!want) return "";
  const byId = servers.find((s) => s.id === want);
  if (byId) return byId.id;
  const lower = want.toLowerCase();
  return servers.find((s) => s.name.toLowerCase() === lower)?.id || "";
}

/**
 * Maps a parsed file onto the form. Like vercel.json, the file only overrides:
 * language and install fall back to what was detected from the repo.
 */
export function specToDraft(
  spec: DeploySpec,
  servers: Pick<Server, "id" | "name">[],
  detected?: { language?: string; install?: string },
): SpecDraft {
  const globalEnv = recordToEnvVars(spec.env);
  const repoName = lastSegment(spec.repo || "") || "app";
  const blocks: ProjectBlock[] = [];
  const appEnv: Record<string, DeployEnvVar[]> = {};

  const apps = spec.apps?.length
    ? spec.apps
    : [{
        name: spec.name || repoName,
        root: spec.root || ".",
        language: spec.language,
        install: spec.install,
        run: spec.run,
        cleanup: spec.cleanup,
        port: spec.port,
        process_manager: spec.process_manager,
        env: [] as DeployEnvVar[],
      }];

  // One app: a top-level port is obviously its port. Several apps cannot share one.
  const sharedPort = apps.length === 1 ? spec.port : undefined;
  for (const app of apps) {
    const root = app.root || ".";
    const port = app.port || sharedPort;
    const name = app.name || (root === "." ? spec.name || repoName : lastSegment(root)) || "app";
    const language = app.language || spec.language || detected?.language || "unknown";
    blocks.push({
      id: blockId(),
      name,
      root,
      language,
      install: app.install || spec.install || detected?.install || "",
      run: app.run || spec.run || defaultRunForLanguage(language),
      cleanup: app.cleanup || spec.cleanup || frameworkPreset(language)?.cleanup || "",
      port: port ? String(port) : "",
      processManager: app.process_manager || spec.process_manager || "none",
      // Came from an explicit croncompose.yml (or a pasted one): never let a later
      // root-folder change silently overwrite what the file said.
      autoDetect: false,
      healthPath: app.health?.path || "",
      healthPort: app.health?.port ? String(app.health.port) : "",
      healthTimeout: app.health?.timeout ? String(app.health.timeout) : "",
    });
    // Keep top-level env separate; per-app list is only this app's own vars.
    appEnv[name] = (app.env || []).map((v) => ({
      key: v.key,
      value: v.value ?? "",
      sensitive: false,
    }));
  }

  return {
    name: spec.name,
    branch: spec.branch,
    clonePath: spec.clone_path,
    serverId: matchServer(servers, spec.server) || undefined,
    blocks,
    globalEnv,
    appEnv,
    advanced: {
      healthPath: spec.health?.path || "",
      healthPort: spec.health?.port ? String(spec.health.port) : "",
      healthTimeout: spec.health?.timeout ? String(spec.health.timeout) : "",
      deployTimeout: spec.deploy_timeout ? String(spec.deploy_timeout) : "",
      autoRollback: !!spec.auto_rollback,
    },
    redeployOn: normalizeRedeployOn(spec.redeploy_on),
  };
}

/** Parses a pasted repo reference: owner/name, an https URL, or a git@ URL. */
export function parseRepoUrl(input: string): { provider: "github" | "gitlab"; fullName: string } | null {
  let ref = input.trim();
  if (!ref) return null;
  const ssh = ref.match(/^git@([^:]+):(.+)$/);
  if (ssh) ref = `https://${ssh[1]}/${ssh[2]}`;
  let provider: "github" | "gitlab" = "github";
  if (/^https?:\/\//i.test(ref)) {
    let u: URL;
    try {
      u = new URL(ref);
    } catch {
      return null;
    }
    const host = u.hostname.replace(/^www\./, "").toLowerCase();
    if (host === "gitlab.com") provider = "gitlab";
    else if (host !== "github.com") return null;
    ref = u.pathname;
  } else if (/^[a-z0-9.-]+\.[a-z]{2,}\//i.test(ref)) {
    // github.com/owner/name without a scheme
    return parseRepoUrl(`https://${ref}`);
  }
  let full = ref.replace(/^\/+|\/+$/g, "").replace(/\.git$/, "");
  const dash = full.indexOf("/-/");
  if (dash >= 0) full = full.slice(0, dash);
  const parts = full.split("/").filter(Boolean);
  if (provider === "github" && parts.length > 2) parts.length = 2;
  if (parts.length < 2 || !parts.every((p) => /^[A-Za-z0-9_.-]+$/.test(p))) return null;
  return { provider, fullName: parts.join("/") };
}

// ---- YAML export ----------------------------------------------------------

const YAML_WORDS = /^(true|false|yes|no|on|off|null|~|y|n)$/i;

/** Quotes a scalar only when YAML would misread it. JSON strings are valid YAML. */
export function yamlScalar(v: string): string {
  if (v === "" || YAML_WORDS.test(v) || /^[-+]?(\d|\.\d)/.test(v) || !/^[A-Za-z0-9_./][A-Za-z0-9_ ./@:+=-]*$/.test(v) || / $/.test(v) || /: /.test(v)) {
    return JSON.stringify(v);
  }
  return v;
}

export type ExportInput = {
  name: string;
  provider: string;
  repo: string;
  branch: string;
  server?: string;
  clonePath?: string;
  blocks: ProjectBlock[];
  globalEnv?: DeployEnvVar[];
  appEnv: Record<string, DeployEnvVar[]>;
  advanced: AdvancedSettings;
  redeployOn?: RedeployMode[];
};

/** Renders the form as a croncompose.yml. Sensitive values are never written. */
export function draftToYaml(d: ExportInput): string {
  const out: string[] = ["# croncompose.yml — docs: /docs", "version: 1"];
  const line = (key: string, value: string | number | boolean | undefined, indent = "") => {
    if (value === undefined || value === "" || value === 0) return;
    const v = typeof value === "string" ? yamlScalar(value) : String(value);
    out.push(`${indent}${key}: ${v}`);
  };
  line("name", d.name);
  line("provider", d.provider);
  line("repo", d.repo);
  line("branch", d.branch);
  line("server", d.server);
  line("clone_path", d.clonePath);

  const a = d.advanced;
  if (a.healthPath.trim()) {
    out.push("health:");
    line("path", a.healthPath.trim(), "  ");
    line("port", Number(a.healthPort) || 0, "  ");
    line("timeout", Number(a.healthTimeout) || 0, "  ");
  }
  line("deploy_timeout", Number(a.deployTimeout) || 0);
  if (a.autoRollback) line("auto_rollback", true);
  const redeploy = (d.redeployOn || DEFAULT_REDEPLOY_ON).filter(Boolean);
  if (redeploy.length && !(redeploy.length === 1 && redeploy[0] === "branch")) {
    out.push(`redeploy_on: [${redeploy.map((m) => yamlScalar(m)).join(", ")}]`);
  }

  const shared = (d.globalEnv || []).filter((v) => v.key.trim() && !v.sensitive);
  if (shared.length) {
    out.push("env:");
    for (const v of shared) out.push(`  ${v.key}: ${yamlScalar(v.value ?? "")}`);
  }

  let hidden = 0;
  out.push("apps:");
  for (const b of d.blocks) {
    out.push(`  - name: ${yamlScalar(b.name || "app")}`);
    line("root", b.root || ".", "    ");
    line("language", b.language === "unknown" ? "" : b.language, "    ");
    line("install", b.install, "    ");
    line("run", b.run, "    ");
    line("cleanup", b.cleanup, "    ");
    line("port", Number(b.port) || 0, "    ");
    line("process_manager", b.processManager === "none" ? "" : b.processManager, "    ");
    if (b.healthPath.trim()) {
      out.push("    health:");
      line("path", b.healthPath.trim(), "      ");
      line("port", Number(b.healthPort) || 0, "      ");
      line("timeout", Number(b.healthTimeout) || 0, "      ");
    }
    const env = (d.appEnv[b.name] || []).filter((v) => v.key.trim());
    const plain = env.filter((v) => !v.sensitive);
    hidden += env.length - plain.length;
    if (plain.length) {
      out.push("    env:");
      for (const v of plain) out.push(`      ${v.key}: ${yamlScalar(v.value ?? "")}`);
    }
  }
  if (hidden) {
    out.push(`# ${hidden} sensitive variable${hidden === 1 ? "" : "s"} left out; set them in CronCompose.`);
  }
  return out.join("\n") + "\n";
}

/** Starter file shown in the importer and on /docs. Keep it valid: specfile_test.go parses a copy. */
export const SPEC_TEMPLATE = `# croncompose.yml — full reference at /docs
version: 1
repo: https://github.com/acme/shop
branch: main
server: my-server            # server name or id in CronCompose

install: npm ci && npm run build
run: npm start               # cwd = activated folder under /opt/…
cleanup: rm -rf .git node_modules/.cache   # after build; frameworks set this
port: 3000
process_manager: pm2         # none | pm2 | systemd | docker
redeploy_on: [branch]        # branch | tag | release

env:
  NODE_ENV: production

health:
  path: /healthz
auto_rollback: true
`;

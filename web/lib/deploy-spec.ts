// croncompose.yml <-> the import form. The control plane parses and validates the
// file (POST /deploys/spec/validate, GET /git/inspect); this module only maps the
// parsed result onto the form, and renders the form back out as YAML for "Export".
import type { DeployEnvVar, DeploySpec, Server } from "./types";
import type { ProjectBlock } from "./project-blocks";

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
  appEnv: Record<string, DeployEnvVar[]>;
  advanced: AdvancedSettings;
};

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
  const topEnv: DeployEnvVar[] = Object.entries(spec.env || {}).map(([key, value]) => ({
    key,
    value,
    sensitive: false,
  }));
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
    blocks.push({
      id: blockId(),
      name,
      root,
      language: app.language || spec.language || detected?.language || "unknown",
      install: app.install || spec.install || detected?.install || "",
      port: port ? String(port) : "",
      processManager: app.process_manager || spec.process_manager || "none",
      // Came from an explicit croncompose.yml (or a pasted one): never let a later
      // root-folder change silently overwrite what the file said.
      autoDetect: false,
      healthPath: app.health?.path || "",
      healthPort: app.health?.port ? String(app.health.port) : "",
      healthTimeout: app.health?.timeout ? String(app.health.timeout) : "",
    });
    // Top-level env is shared by every app; an app's own value wins.
    const own = (app.env || []).map((v) => ({ key: v.key, value: v.value ?? "", sensitive: false }));
    const ownKeys = new Set(own.map((v) => v.key));
    appEnv[name] = [...topEnv.filter((v) => !ownKeys.has(v.key)), ...own];
  }

  return {
    name: spec.name,
    branch: spec.branch,
    clonePath: spec.clone_path,
    serverId: matchServer(servers, spec.server) || undefined,
    blocks,
    appEnv,
    advanced: {
      healthPath: spec.health?.path || "",
      healthPort: spec.health?.port ? String(spec.health.port) : "",
      healthTimeout: spec.health?.timeout ? String(spec.health.timeout) : "",
      deployTimeout: spec.deploy_timeout ? String(spec.deploy_timeout) : "",
      autoRollback: !!spec.auto_rollback,
    },
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
  appEnv: Record<string, DeployEnvVar[]>;
  advanced: AdvancedSettings;
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

  let hidden = 0;
  out.push("apps:");
  for (const b of d.blocks) {
    out.push(`  - name: ${yamlScalar(b.name || "app")}`);
    line("root", b.root || ".", "    ");
    line("language", b.language === "unknown" ? "" : b.language, "    ");
    line("install", b.install, "    ");
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
port: 3000
process_manager: pm2         # none | pm2 | systemd | docker

env:
  NODE_ENV: production

health:
  path: /healthz
auto_rollback: true
`;

import type {
  DeployApp,
  DeployEnvVar,
  DeployInspect,
  DeployProject,
  PM2AppHint,
} from "./types";
import { defaultRunForFramework, fieldsForFramework, frameworkPreset } from "./frameworks";

export type ProjectBlock = {
  id: string;
  name: string;
  root: string;
  language: string;
  /** Build / install script run after clone (deps, compile, migrate). */
  install: string;
  /**
   * Command that starts the process after the release is activated. Runs with
   * cwd set to the app folder under the deploy path (usually under /opt/…).
   * Example for a Go binary: `./server`.
   */
  run: string;
  /**
   * Shell run after build to drop source/caches from the release. Framework
   * presets fill this automatically.
   */
  cleanup: string;
  /**
   * One-shot shell after the release is activated and before the process
   * starts — migrations, seed data, cache warmers. Empty = skip.
   */
  preStart: string;
  port: string;
  processManager: string;
  /**
   * Explicit pm2/systemd name on the host. When set (typically from
   * ecosystem.config.js), deploys and process actions use it instead of
   * project-app.
   */
  processName: string;
  /**
   * True when processName was filled from an ecosystem apps[].name and the
   * operator has not edited it yet. Drives the “auto-populated” hint.
   */
  processNameAuto: boolean;
  /** Repo-relative ecosystem file path when processName came from pm2_apps. */
  ecosystemFile: string;
  /**
   * True while language/install still reflect an automatic detection for this
   * block's root, so a root change (see detectAt in git-detect.ts) is allowed to
   * refresh them. Picking a framework manually, or the block coming from a
   * pasted/repo croncompose.yml, turns this off so CronCompose never overwrites
   * a choice the person made.
   */
  autoDetect: boolean;
  /** Per-block health check, overriding the project's shared one. Empty path = unset. */
  healthPath: string;
  healthPort: string;
  healthTimeout: string;
  /**
   * Deploy wave index. Apps with the same group start together after the
   * release; lower groups finish before higher ones. 0 = first / all together.
   */
  startGroup: number;
};

/** Modes that can auto-redeploy a project when the Git provider fires a webhook. */
export type RedeployMode = "branch" | "tag" | "release";

export const DEFAULT_REDEPLOY_ON: RedeployMode[] = ["branch"];

export function newBlockId(): string {
  return `b-${Math.random().toString(36).slice(2, 10)}`;
}

export function normalizeBlockRoot(root: string): string {
  let trimmed = root.trim().replace(/\\/g, "/");
  trimmed = trimmed.replace(/^\/+/, "");
  trimmed = trimmed.replace(/^\.\//, "");
  while (trimmed.includes("//")) trimmed = trimmed.replaceAll("//", "/");
  trimmed = trimmed.replace(/\/+$/, "");
  if (trimmed === "" || trimmed === ".") return ".";
  return trimmed;
}

export function nameFromRoot(root: string, repoFullName: string): string {
  const normalized = normalizeBlockRoot(root);
  if (normalized === ".") {
    const repoName = repoFullName.split("/").filter(Boolean).pop();
    return repoName || "app";
  }
  const segment = normalized.split("/").filter(Boolean).pop();
  return segment || "app";
}

/** Build a start command from an ecosystem apps[] entry. */
export function runFromPM2App(app: PM2AppHint): string {
  const script = (app.script || "").trim();
  const args = (app.args || "").trim();
  if (!script) return "";
  return args ? `${script} ${args}` : script;
}

function pickPM2App(apps: PM2AppHint[] | undefined, root: string): PM2AppHint | undefined {
  if (!apps?.length) return undefined;
  const want = normalizeBlockRoot(root);
  const exact = apps.find((a) => normalizeBlockRoot(a.cwd || ".") === want);
  if (exact) return exact;
  if (want === "." || apps.length === 1) return apps[0];
  return undefined;
}

function ecosystemFileLabel(app: PM2AppHint): string {
  const file = (app.file || "").trim();
  if (!file) return "ecosystem.config.js";
  const base = file.split("/").pop();
  return base || file;
}

function applyPM2App(block: ProjectBlock, app: PM2AppHint | undefined): ProjectBlock {
  if (!app) return block;
  const run = runFromPM2App(app);
  const fromEco = Boolean(app.name);
  return {
    ...block,
    processManager: "pm2",
    processName: app.name || block.processName,
    processNameAuto: fromEco ? true : block.processNameAuto,
    ecosystemFile: fromEco ? ecosystemFileLabel(app) : block.ecosystemFile,
    run: run || block.run,
    port: app.port ? String(app.port) : block.port,
  };
}

export function seedBlockFromInspect(inspect: DeployInspect, repoFullName: string): ProjectBlock {
  const root = inspect.root_directory || ".";
  const language = inspect.language || "unknown";
  const preset = frameworkPreset(language);
  const pm =
    inspect.process_manager === "pm2" || inspect.has_pm2_ecosystem
      ? "pm2"
      : preset?.processManager || "none";
  const base: ProjectBlock = {
    id: newBlockId(),
    name: nameFromRoot(root, repoFullName),
    root,
    language,
    install: inspect.install_script || preset?.install || "",
    run: defaultRunForLanguage(language),
    cleanup: preset?.cleanup || "",
    preStart: "",
    port: preset?.port || "",
    processManager: pm,
    processName: "",
    processNameAuto: false,
    ecosystemFile: "",
    autoDetect: true,
    healthPath: "",
    healthPort: "",
    healthTimeout: "",
    startGroup: 0,
  };
  const app = pickPM2App(inspect.pm2_apps, root);
  const filled = applyPM2App(base, app);
  // Prefer a human folder name for the block label; keep ecosystem name as processName.
  if (app?.name && normalizeBlockRoot(root) === ".") {
    // Single-app at repo root: use ecosystem name as the block name too when
    // it is more specific than the repo slug.
    if (!filled.name || filled.name === nameFromRoot(".", repoFullName)) {
      return { ...filled, name: app.name };
    }
  }
  return filled;
}

/**
 * Seed one or more blocks from inspect. When the ecosystem lists several apps
 * with distinct cwd values, each becomes its own block.
 */
export function seedBlocksFromInspect(inspect: DeployInspect, repoFullName: string): ProjectBlock[] {
  const apps = inspect.pm2_apps || [];
  const distinctCwd = new Set(
    apps.map((a) => normalizeBlockRoot(a.cwd || ".")).filter(Boolean),
  );
  if (apps.length <= 1 || distinctCwd.size <= 1) {
    return [seedBlockFromInspect(inspect, repoFullName)];
  }
  const language = inspect.language || "unknown";
  const preset = frameworkPreset(language);
  return apps.map((app, i) => {
    const root = normalizeBlockRoot(app.cwd || ".");
    const run = runFromPM2App(app);
    return {
      id: newBlockId(),
      name: app.name || nameFromRoot(root, repoFullName),
      root,
      language,
      install: i === 0 ? inspect.install_script || preset?.install || "" : preset?.install || "",
      run: run || defaultRunForLanguage(language),
      cleanup: preset?.cleanup || "",
      preStart: "",
      port: app.port ? String(app.port) : preset?.port || "",
      processManager: "pm2",
      processName: app.name || "",
      processNameAuto: Boolean(app.name),
      ecosystemFile: app.name ? ecosystemFileLabel(app) : "",
      autoDetect: true,
      healthPath: "",
      healthPort: "",
      healthTimeout: "",
      startGroup: i,
    } satisfies ProjectBlock;
  });
}

/** Sensible start command when the process manager has nothing else to go on. */
export function defaultRunForLanguage(language: string): string {
  return defaultRunForFramework(language);
}

/** Apply a curated framework preset onto a block (install / run / port / pm). */
export function applyFramework(block: ProjectBlock, frameworkId: string): ProjectBlock {
  const fields = fieldsForFramework(frameworkId);
  if (!fields) {
    return {
      ...block,
      language: frameworkId,
      autoDetect: false,
      run: block.run || defaultRunForFramework(frameworkId),
    };
  }
  return {
    ...block,
    ...fields,
    processName: block.processName,
    processNameAuto: block.processNameAuto,
    ecosystemFile: block.ecosystemFile,
  };
}

/** Merge repo detection into a block (language / install / run / port / pm). */
export function applyDetection(
  block: ProjectBlock,
  det: {
    language?: string;
    install_script?: string;
    has_pm2_ecosystem?: boolean;
    pm2_apps?: PM2AppHint[];
  },
): ProjectBlock {
  const lang = det.language || "unknown";
  const preset = frameworkPreset(lang);
  const next: ProjectBlock = {
    ...block,
    language: lang,
    install: det.install_script || preset?.install || block.install,
    run: defaultRunForLanguage(lang) || block.run,
    cleanup: block.cleanup || preset?.cleanup || "",
    port: block.port || preset?.port || "",
    processManager:
      block.processManager !== "none"
        ? block.processManager
        : preset?.processManager || (det.has_pm2_ecosystem ? "pm2" : block.processManager),
  };
  const app = pickPM2App(det.pm2_apps, block.root);
  // Only fill empty fields from ecosystem so a manual edit is not wiped on re-detect.
  const filled = applyPM2App(
    {
      ...next,
      processName: block.processName,
      run: block.run || next.run,
      port: block.port || next.port,
    },
    app,
  );
  const processName = block.processName || filled.processName;
  const fromEco = !block.processName && Boolean(filled.processName);
  return {
    ...filled,
    processName,
    processNameAuto: fromEco ? true : block.processNameAuto,
    ecosystemFile: fromEco ? filled.ecosystemFile : block.ecosystemFile || filled.ecosystemFile,
    run: block.run || filled.run,
    port: block.port || filled.port,
  };
}

export function emptyBlock(): ProjectBlock {
  const preset = frameworkPreset("node")!;
  return {
    id: newBlockId(),
    name: "",
    root: "",
    language: preset.id,
    install: preset.install,
    run: preset.run,
    cleanup: preset.cleanup,
    preStart: "",
    port: preset.port,
    processManager: preset.processManager,
    processName: "",
    processNameAuto: false,
    ecosystemFile: "",
    autoDetect: true,
    healthPath: "",
    healthPort: "",
    healthTimeout: "",
    startGroup: 0,
  };
}

export function ensureUniqueBlockNames(blocks: ProjectBlock[]): ProjectBlock[] {
  const seen = new Map<string, number>();
  return blocks.map((block) => {
    const base = block.name.trim() || "app";
    const count = seen.get(base) ?? 0;
    seen.set(base, count + 1);
    if (count === 0) return block;
    return { ...block, name: `${base}-${count + 1}` };
  });
}

export function hasDuplicateRoots(blocks: ProjectBlock[]): boolean {
  const roots = new Set<string>();
  for (const block of blocks) {
    const root = normalizeBlockRoot(block.root);
    if (roots.has(root)) return true;
    roots.add(root);
  }
  return false;
}

export function blocksReady(blocks: ProjectBlock[]): boolean {
  if (blocks.length === 0) return false;
  return blocks.every((block) => block.root.trim() !== "");
}

export function blocksToDeployApps(
  blocks: ProjectBlock[],
  appEnv: Record<string, DeployEnvVar[]>,
): DeployApp[] {
  return blocks.map((block) => ({
    name: block.name,
    root: normalizeBlockRoot(block.root),
    language: block.language,
    install: block.install,
    run: block.run.trim() || undefined,
    cleanup: block.cleanup.trim() || undefined,
    pre_start: block.preStart.trim() || undefined,
    process_manager: block.processManager,
    process_name: block.processName.trim() || undefined,
    port: block.port ? Number(block.port) : undefined,
    env: appEnv[block.name] ?? [],
    start_group: block.startGroup > 0 ? block.startGroup : undefined,
    health: block.healthPath.trim()
      ? {
          path: block.healthPath.trim(),
          port: block.healthPort ? Number(block.healthPort) : undefined,
          timeout: block.healthTimeout ? Number(block.healthTimeout) : undefined,
        }
      : undefined,
  }));
}

/** Seed configure-tab blocks from an existing project (edit flow). */
export function blocksFromProject(project: DeployProject): ProjectBlock[] {
  const apps = project.apps?.length
    ? project.apps
    : [
        {
          name: project.name || nameFromRoot(project.root_directory || ".", project.repo_full_name),
          root: project.root_directory || ".",
          language: project.language,
          install: project.install_script,
          process_manager: project.process_manager,
          port: project.port || undefined,
          health: project.health_path
            ? {
                path: project.health_path,
                port: project.health_port || undefined,
                timeout: project.health_timeout_seconds || undefined,
              }
            : undefined,
        } satisfies DeployApp,
      ];

  return apps.map((app) => ({
    id: newBlockId(),
    name: app.name,
    root: app.root || ".",
    language: app.language || project.language || "unknown",
    install: app.install || project.install_script || "",
    run: app.run || "",
    cleanup: app.cleanup || "",
    preStart: app.pre_start || "",
    port: app.port ? String(app.port) : project.port ? String(project.port) : "",
    processManager: app.process_manager || project.process_manager || "none",
    processName: app.process_name || "",
    processNameAuto: false,
    ecosystemFile: app.process_manager === "pm2" || project.process_manager === "pm2"
      ? "ecosystem.config.js"
      : "",
    autoDetect: false,
    healthPath: app.health?.path || "",
    healthPort: app.health?.port ? String(app.health.port) : "",
    healthTimeout: app.health?.timeout ? String(app.health.timeout) : "",
    startGroup: app.start_group ?? 0,
  }));
}

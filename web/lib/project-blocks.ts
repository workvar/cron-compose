import type { DeployApp, DeployEnvVar, DeployInspect, DeployProject } from "./types";
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
  port: string;
  processManager: string;
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

export function seedBlockFromInspect(inspect: DeployInspect, repoFullName: string): ProjectBlock {
  const root = inspect.root_directory || ".";
  const language = inspect.language || "unknown";
  const preset = frameworkPreset(language);
  const pm =
    inspect.process_manager === "pm2"
      ? "pm2"
      : preset?.processManager || "none";
  return {
    id: newBlockId(),
    name: nameFromRoot(root, repoFullName),
    root,
    language,
    install: inspect.install_script || preset?.install || "",
    run: defaultRunForLanguage(language),
    cleanup: preset?.cleanup || "",
    port: preset?.port || "",
    processManager: pm,
    autoDetect: true,
    healthPath: "",
    healthPort: "",
    healthTimeout: "",
  };
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
  return { ...block, ...fields };
}

/** Merge repo detection into a block (language / install / run / port / pm). */
export function applyDetection(
  block: ProjectBlock,
  det: { language?: string; install_script?: string; has_pm2_ecosystem?: boolean },
): ProjectBlock {
  const lang = det.language || "unknown";
  const preset = frameworkPreset(lang);
  return {
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
    port: preset.port,
    processManager: preset.processManager,
    autoDetect: true,
    healthPath: "",
    healthPort: "",
    healthTimeout: "",
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
    process_manager: block.processManager,
    port: block.port ? Number(block.port) : undefined,
    env: appEnv[block.name] ?? [],
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
    port: app.port ? String(app.port) : project.port ? String(project.port) : "",
    processManager: app.process_manager || project.process_manager || "none",
    autoDetect: false,
    healthPath: app.health?.path || "",
    healthPort: app.health?.port ? String(app.health.port) : "",
    healthTimeout: app.health?.timeout ? String(app.health.timeout) : "",
  }));
}

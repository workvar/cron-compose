import type { DeployApp, DeployEnvVar, DeployInspect } from "./types";

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
  return {
    id: newBlockId(),
    name: nameFromRoot(root, repoFullName),
    root,
    language: inspect.language || "unknown",
    install: inspect.install_script,
    run: defaultRunForLanguage(inspect.language || ""),
    port: "",
    processManager: inspect.process_manager === "pm2" ? "pm2" : "none",
    autoDetect: true,
    healthPath: "",
    healthPort: "",
    healthTimeout: "",
  };
}

/** Sensible start command when the process manager has nothing else to go on. */
export function defaultRunForLanguage(language: string): string {
  switch ((language || "").toLowerCase()) {
    case "go":
    case "golang":
      return "./app";
    case "node":
    case "javascript":
    case "typescript":
      return "npm start";
    case "python":
      return "python3 -m app";
    default:
      return "";
  }
}

export function emptyBlock(): ProjectBlock {
  return {
    id: newBlockId(),
    name: "",
    root: "",
    language: "node",
    install: "",
    run: "npm start",
    port: "",
    processManager: "none",
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

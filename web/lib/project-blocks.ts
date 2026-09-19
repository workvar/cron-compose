import type { DeployApp, DeployEnvVar, DeployInspect } from "./types";

export type ProjectBlock = {
  id: string;
  name: string;
  root: string;
  language: string;
  install: string;
  port: string;
  processManager: string;
};

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
    port: "",
    processManager: inspect.process_manager === "pm2" ? "pm2" : "none",
  };
}

export function emptyBlock(): ProjectBlock {
  return {
    id: newBlockId(),
    name: "",
    root: "",
    language: "node",
    install: "",
    port: "",
    processManager: "none",
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
    process_manager: block.processManager,
    port: block.port ? Number(block.port) : undefined,
    env: appEnv[block.name] ?? [],
  }));
}

import { classifyInstallCommand, splitInstallCommands } from "./deploy-steps";
import type { ProjectBlock } from "./project-blocks";

/** One deploy step for one process. */
export type FlowPhase =
  | "install"
  | "build"
  | "cleanup"
  | "pre_start"
  | "start"
  | "health";

export type FlowTask = {
  /** Stable id for DnD keys. */
  id: string;
  blockId: string;
  phase: FlowPhase;
};

export type FlowWave = {
  /** Stable id so empty waves survive reorders. */
  id: string;
  tasks: FlowTask[];
};

export const FLOW_PHASES: { id: FlowPhase; label: string; hint: string }[] = [
  { id: "install", label: "Install", hint: "Deps / modules" },
  { id: "build", label: "Build", hint: "Compile / bundle" },
  { id: "cleanup", label: "Cleanup", hint: "Post-install trim" },
  { id: "pre_start", label: "Pre-start", hint: "Migrations" },
  { id: "start", label: "Start", hint: "Process manager" },
  { id: "health", label: "Health", hint: "HTTP probe" },
];

const PRE_RELEASE: FlowPhase[] = ["install", "build", "cleanup"];
const POST_RELEASE: FlowPhase[] = ["pre_start", "start", "health"];

function newId(prefix: string): string {
  return `${prefix}-${Math.random().toString(36).slice(2, 10)}`;
}

export function phaseLabel(phase: FlowPhase): string {
  return FLOW_PHASES.find((p) => p.id === phase)?.label || phase;
}

/** Split an install script into install vs build command strings. */
export function splitInstallBuild(
  script: string,
  language: string,
): { install: string; build: string } {
  const parts = splitInstallCommands(script);
  if (parts.length === 0) return { install: "", build: "" };
  const installParts: string[] = [];
  const buildParts: string[] = [];
  for (const cmd of parts) {
    const kind = classifyInstallCommand(cmd, language).kind;
    if (kind === "install") installParts.push(cmd);
    else buildParts.push(cmd);
  }
  // Unclassified-only script: treat the whole thing as install (agent runs it once).
  if (installParts.length === 0 && buildParts.length === 0) {
    return { install: script.trim(), build: "" };
  }
  // Build-only script (e.g. just `go build`): keep as build.
  if (installParts.length === 0) {
    return { install: "", build: buildParts.join(" && ") };
  }
  return {
    install: installParts.join(" && "),
    build: buildParts.join(" && "),
  };
}

/** Tasks this block contributes to a default flow. */
export function tasksForBlock(block: ProjectBlock): Omit<FlowTask, "id">[] {
  const out: Omit<FlowTask, "id">[] = [];
  const { install, build } = splitInstallBuild(block.install, block.language);
  if (install) out.push({ blockId: block.id, phase: "install" });
  if (build) out.push({ blockId: block.id, phase: "build" });
  // Install script that didn't split but is non-empty already covered; empty skip.
  if (!install && !build && block.install.trim()) {
    out.push({ blockId: block.id, phase: "install" });
  }
  if (block.cleanup.trim()) out.push({ blockId: block.id, phase: "cleanup" });
  if (block.preStart.trim()) out.push({ blockId: block.id, phase: "pre_start" });
  if (block.processManager === "pm2" || block.processManager === "systemd" || block.processManager === "docker") {
    out.push({ blockId: block.id, phase: "start" });
  }
  if (block.healthPath.trim()) out.push({ blockId: block.id, phase: "health" });
  return out;
}

function withIds(tasks: Omit<FlowTask, "id">[]): FlowTask[] {
  return tasks.map((t) => ({ ...t, id: newId("t") }));
}

/**
 * Default flow: install → build → cleanup (each phase one wave, all apps),
 * then start waves from startGroup, with pre_start/start/health per group.
 */
export function defaultFlow(blocks: ProjectBlock[]): FlowWave[] {
  if (blocks.length === 0) return [];
  const waves: FlowWave[] = [];

  const byPhase = (phase: FlowPhase) =>
    withIds(
      blocks.flatMap((b) => tasksForBlock(b).filter((t) => t.phase === phase)),
    );

  for (const phase of PRE_RELEASE) {
    const tasks = byPhase(phase);
    if (tasks.length) waves.push({ id: newId("w"), tasks });
  }

  const groups = new Map<number, ProjectBlock[]>();
  for (const b of blocks) {
    const g = Number.isFinite(b.startGroup) && b.startGroup > 0 ? Math.floor(b.startGroup) : 0;
    const list = groups.get(g) ?? [];
    list.push(b);
    groups.set(g, list);
  }
  const order = [...groups.keys()].sort((a, b) => a - b);
  for (const g of order) {
    const members = groups.get(g) || [];
    for (const phase of POST_RELEASE) {
      const tasks = withIds(
        members.flatMap((b) => tasksForBlock(b).filter((t) => t.phase === phase)),
      );
      if (tasks.length) waves.push({ id: newId("w"), tasks });
    }
  }

  return waves.length ? waves : [{ id: newId("w"), tasks: [] }];
}

/** Sync flow when blocks are added/removed: drop orphan tasks, add missing defaults. */
export function reconcileFlow(blocks: ProjectBlock[], flow: FlowWave[]): FlowWave[] {
  const blockIds = new Set(blocks.map((b) => b.id));
  const expected = new Map<string, FlowPhase>();
  for (const b of blocks) {
    for (const t of tasksForBlock(b)) {
      expected.set(`${t.blockId}:${t.phase}`, t.phase);
    }
  }

  const seen = new Set<string>();
  const next: FlowWave[] = flow.map((w) => ({
    id: w.id,
    tasks: w.tasks.filter((t) => {
      const key = `${t.blockId}:${t.phase}`;
      if (!blockIds.has(t.blockId) || !expected.has(key)) return false;
      if (seen.has(key)) return false;
      seen.add(key);
      return true;
    }),
  }));

  const missing: FlowTask[] = [];
  for (const [key, phase] of expected) {
    if (seen.has(key)) continue;
    const blockId = key.slice(0, key.lastIndexOf(":"));
    missing.push({ id: newId("t"), blockId, phase });
  }
  if (missing.length) {
    if (next.length === 0) next.push({ id: newId("w"), tasks: missing });
    else next[next.length - 1] = {
      ...next[next.length - 1],
      tasks: [...next[next.length - 1].tasks, ...missing],
    };
  }
  return next.length ? next : [{ id: newId("w"), tasks: [] }];
}

export function addEmptyWave(flow: FlowWave[]): FlowWave[] {
  return [...flow, { id: newId("w"), tasks: [] }];
}

export function removeWave(flow: FlowWave[], waveId: string): FlowWave[] {
  const next = flow.filter((w) => w.id !== waveId);
  return next.length ? next : [{ id: newId("w"), tasks: [] }];
}

export function moveTask(
  flow: FlowWave[],
  taskId: string,
  toWaveId: string,
  beforeTaskId?: string | null,
): FlowWave[] {
  let moved: FlowTask | null = null;
  const stripped = flow.map((w) => {
    const hit = w.tasks.find((t) => t.id === taskId);
    if (hit) moved = hit;
    return { ...w, tasks: w.tasks.filter((t) => t.id !== taskId) };
  });
  if (!moved) return flow;
  return stripped.map((w) => {
    if (w.id !== toWaveId) return w;
    const tasks = [...w.tasks];
    const at = beforeTaskId ? tasks.findIndex((t) => t.id === beforeTaskId) : -1;
    if (at >= 0) tasks.splice(at, 0, moved!);
    else tasks.push(moved!);
    return { ...w, tasks };
  });
}

export function hasEmptyWaves(flow: FlowWave[]): boolean {
  return flow.some((w) => w.tasks.length === 0);
}

/** Hard ordering: no install/build/cleanup after a pre_start/start/health. */
export function flowOrderError(flow: FlowWave[]): string | null {
  let sawPost = false;
  for (const w of flow) {
    for (const t of w.tasks) {
      const pre = PRE_RELEASE.includes(t.phase);
      const post = POST_RELEASE.includes(t.phase);
      if (post) sawPost = true;
      if (pre && sawPost) {
        return "Install / build / cleanup must finish before pre-start, start, or health.";
      }
    }
  }
  return null;
}

export function flowProblems(flow: FlowWave[]): string[] {
  const out: string[] = [];
  if (hasEmptyWaves(flow)) out.push("Fill or delete every empty wave");
  const order = flowOrderError(flow);
  if (order) out.push(order);
  return out;
}

/** Stable content signature so reconcile can no-op when nothing changed. */
export function flowSignature(flow: FlowWave[]): string {
  return flow
    .map((w) => w.tasks.map((t) => `${t.blockId}:${t.phase}`).join(","))
    .join("|");
}

/** Derive start_group on blocks from start-phase wave membership (API compat). */
export function blocksWithStartGroupsFromFlow(
  blocks: ProjectBlock[],
  flow: FlowWave[],
): ProjectBlock[] {
  const groupOf = new Map<string, number>();
  let startWave = 0;
  for (const w of flow) {
    const starts = w.tasks.filter((t) => t.phase === "start");
    if (starts.length === 0) continue;
    for (const t of starts) {
      if (!groupOf.has(t.blockId)) groupOf.set(t.blockId, startWave);
    }
    startWave++;
  }
  return blocks.map((b) => ({
    ...b,
    startGroup: groupOf.get(b.id) ?? 0,
  }));
}

/** Wire format for API / croncompose.yml. */
export type FlowSpecTask = { app: string; phase: string };
export type FlowSpecWave = FlowSpecTask[];

export function flowToSpec(blocks: ProjectBlock[], flow: FlowWave[]): FlowSpecWave[] {
  const nameOf = new Map(blocks.map((b) => [b.id, b.name || "app"]));
  return flow
    .filter((w) => w.tasks.length > 0)
    .map((w) =>
      w.tasks.map((t) => ({
        app: nameOf.get(t.blockId) || "app",
        phase: t.phase,
      })),
    );
}

export function flowFromSpec(
  blocks: ProjectBlock[],
  spec: FlowSpecWave[] | null | undefined,
): FlowWave[] | null {
  if (!spec || spec.length === 0) return null;
  const byName = new Map(blocks.map((b) => [b.name, b]));
  const waves: FlowWave[] = [];
  for (const wave of spec) {
    const tasks: FlowTask[] = [];
    for (const t of wave) {
      const block = byName.get(t.app);
      if (!block) continue;
      const phase = t.phase as FlowPhase;
      if (!FLOW_PHASES.some((p) => p.id === phase)) continue;
      tasks.push({ id: newId("t"), blockId: block.id, phase });
    }
    waves.push({ id: newId("w"), tasks });
  }
  return waves.length ? reconcileFlow(blocks, waves) : null;
}

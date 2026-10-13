import { phaseLabel, type FlowPhase } from "./deploy-flow";
import type { LiveProcess, LiveView, StepState } from "./deploy-progress";

export type WaveTaskLive = {
  id: string;
  app: string;
  phase: string;
  label: string;
  state: StepState;
  percent: number;
  /** Terminal pane id (process id or "shared"). */
  pane: string;
};

export type WaveLive = {
  index: number;
  total: number;
  state: StepState;
  percent: number;
  tasks: WaveTaskLive[];
};

const WAVE_RE = /flow wave\s+(\d+)\s*\/\s*(\d+)\s*:\s*(.*)$/i;

/** Parse the highest "flow wave N/M" line from the live log text. */
export function currentWaveFromLogs(logs: { chunk: string }[]): { index: number; total: number } | null {
  let hit: { index: number; total: number } | null = null;
  const text = [...logs]
    .map((l) => l.chunk)
    .join("");
  for (const line of text.split("\n")) {
    const m = line.replace(/^\[in[^\]]*\]\s*/, "").match(/^(?:install|pre_start|start):\s*(.*)$/i);
    const body = m ? m[1] : line;
    const w = body.match(WAVE_RE);
    if (!w) continue;
    hit = { index: Number(w[1]), total: Number(w[2]) };
  }
  return hit;
}

function stepFor(proc: LiveProcess | undefined, phase: string) {
  if (!proc) return null;
  const kind = phase === "pre_start" ? "pre_start" : phase;
  return proc.steps.find((s) => s.kind === kind) ?? null;
}

function rollupTasks(tasks: WaveTaskLive[]): { state: StepState; percent: number } {
  if (tasks.length === 0) return { state: "pending", percent: 0 };
  if (tasks.some((t) => t.state === "failed")) return { state: "failed", percent: Math.round(tasks.reduce((n, t) => n + t.percent, 0) / tasks.length) };
  if (tasks.every((t) => t.state === "done" || t.state === "skipped")) {
    return { state: tasks.some((t) => t.state === "done") ? "done" : "skipped", percent: 100 };
  }
  if (tasks.some((t) => t.state === "active")) {
    return { state: "active", percent: Math.round(tasks.reduce((n, t) => n + t.percent, 0) / tasks.length) };
  }
  return { state: "pending", percent: Math.round(tasks.reduce((n, t) => n + t.percent, 0) / tasks.length) };
}

/**
 * Live wave board from the project's saved flow and the progress rail.
 * When flow is empty, synthesizes one wave per process from its steps.
 */
export function buildDeployWaves(input: {
  flow?: Array<Array<{ app: string; phase: string }>> | null;
  view: LiveView;
  logs: { chunk: string }[];
  status: string;
}): WaveLive[] {
  const { view, logs, status } = input;
  const byName = new Map(view.processes.map((p) => [p.name, p]));
  const cursor = currentWaveFromLogs(logs);

  let waves: WaveLive[] = [];
  const flow = input.flow?.filter((w) => w.length > 0) ?? [];

  if (flow.length > 0) {
    waves = flow.map((tasks, i) => {
      const liveTasks: WaveTaskLive[] = tasks.map((t, j) => {
        const proc = byName.get(t.app);
        const step = stepFor(proc, t.phase);
        const phase = t.phase as FlowPhase;
        return {
          id: `w${i}-${t.app}-${t.phase}-${j}`,
          app: t.app,
          phase: t.phase,
          label: `${t.app} · ${phaseLabel(phase)}`,
          state: step?.state ?? "pending",
          percent: step?.percent ?? 0,
          pane: proc?.id ?? "shared",
        };
      });
      const roll = rollupTasks(liveTasks);
      return {
        index: i + 1,
        total: flow.length,
        state: roll.state,
        percent: roll.percent,
        tasks: liveTasks,
      };
    });
  } else {
    // Default path: one wave of shared prepare, then one wave per process rail.
    const sharedTasks: WaveTaskLive[] = view.shared.map((s) => ({
      id: s.id,
      app: "Server",
      phase: s.kind,
      label: s.label,
      state: s.state,
      percent: s.percent,
      pane: "shared",
    }));
    if (sharedTasks.length) {
      const roll = rollupTasks(sharedTasks);
      waves.push({ index: 1, total: 1 + view.processes.length, state: roll.state, percent: roll.percent, tasks: sharedTasks });
    }
    view.processes.forEach((p, i) => {
      const liveTasks: WaveTaskLive[] = p.steps.map((s) => ({
        id: s.id,
        app: p.name,
        phase: s.kind,
        label: s.label,
        state: s.state,
        percent: s.percent,
        pane: p.id,
      }));
      const roll = rollupTasks(liveTasks);
      waves.push({
        index: waves.length + 1,
        total: (sharedTasks.length ? 1 : 0) + view.processes.length,
        state: roll.state,
        percent: roll.percent,
        tasks: liveTasks,
      });
      void i;
    });
  }

  // Align "current" wave: mark later waves pending if they have not started and
  // earlier ones are still active. Use log cursor when present.
  if (cursor && waves.length > 0) {
    const cur = Math.min(Math.max(cursor.index, 1), waves.length) - 1;
    for (let i = 0; i < waves.length; i++) {
      if (i > cur && waves[i].state === "pending") continue;
      if (i < cur && waves[i].state === "pending" && status === "running") {
        // Log already advanced past this wave — treat unfinished as done when
        // the run moved on (e.g. empty optional phase).
        const allPending = waves[i].tasks.every((t) => t.state === "pending");
        if (allPending) {
          waves[i] = {
            ...waves[i],
            state: "done",
            percent: 100,
            tasks: waves[i].tasks.map((t) => ({ ...t, state: "skipped", percent: 100 })),
          };
        }
      }
    }
  }

  if (status === "succeeded") {
    waves = waves.map((w) => ({
      ...w,
      state: w.state === "failed" ? w.state : "done",
      percent: w.state === "failed" ? w.percent : 100,
      tasks: w.tasks.map((t) =>
        t.state === "failed" ? t : { ...t, state: t.state === "skipped" ? "skipped" : "done", percent: 100 },
      ),
    }));
  }

  return waves;
}

export function activeWaveIndex(waves: WaveLive[]): number {
  const active = waves.findIndex((w) => w.state === "active");
  if (active >= 0) return active;
  const pending = waves.findIndex((w) => w.state === "pending");
  if (pending >= 0) return Math.max(0, pending - 1);
  return Math.max(0, waves.length - 1);
}

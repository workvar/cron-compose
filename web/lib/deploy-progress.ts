// Live deploy progress from the agent log. The agent emits phase lines
// (`install: npm run build (in /path)`) and then the raw installer output.
// This walks that text into one rail per process plus a shared prepare/release
// rail, and splits the log into a pane per process so the run screen can show
// them side by side.

import { classifyInstallCommand, splitInstallCommands } from "./deploy-steps";
import { stripAnsi } from "./strip-ansi";

export type StepState = "pending" | "active" | "done" | "failed" | "skipped";

export type LiveStep = {
  id: string;
  label: string;
  /** Shell command, when this step is one. */
  detail?: string;
  kind: string;
  state: StepState;
  /** 0–100 for this step alone. */
  percent: number;
};

export type LiveProcess = {
  id: string;
  name: string;
  state: StepState;
  percent: number;
  steps: LiveStep[];
};

export type LiveLogLine = { pane: string; text: string };

export type LiveView = {
  percent: number;
  headline: string;
  shared: LiveStep[];
  processes: LiveProcess[];
  lines: LiveLogLine[];
};

export type ProgressApp = {
  name: string;
  root?: string;
  language?: string;
  install?: string;
  cleanup?: string;
  processManager?: string;
  healthPath?: string;
};

export type LiveInput = {
  logs: { seq: number; chunk: string }[];
  apps: ProgressApp[];
  projectName?: string;
  status: string;
};

type Kind =
  | "preflight"
  | "clone"
  | "install"
  | "build"
  | "command"
  | "cleanup"
  | "release"
  | "start"
  | "health";

type Mutable = {
  id: string;
  label: string;
  detail?: string;
  kind: Kind;
  state: StepState;
  ticks: number;
};

type Proc = {
  id: string;
  name: string;
  root: string;
  language: string;
  plannedScript: string;
  boundDir?: string;
  commandSteps: Mutable[];
  cleanup?: Mutable;
  start?: Mutable;
  health?: Mutable;
  steps: Mutable[];
};

const PHASE_RE = /^(preflight|clone|install|release|start|health):\s*(.*)$/;
const START_RE = /^(pm2|systemd|docker compose|systemctl|process_manager=none)\b/;
/** Parallel multi-app PTY lines are tagged `[in <workDir>] …` for pane routing. */
const IN_PREFIX_RE = /^\[in\s+([^\]]+)\]\s?(.*)$/;

function labelFor(kind: string, name: string, cmd: string): string {
  switch (kind) {
    case "install":
      return `Installing ${name}`;
    case "build":
      return `Building ${name}`;
    case "cleanup":
      return `Cleaning ${name}`;
    case "start":
      return `Starting ${name}`;
    case "health":
      return `Checking ${name}`;
    default:
      return cmd.trim() || name;
  }
}

function makeStep(id: string, label: string, kind: Kind, detail?: string): Mutable {
  return { id, label, detail, kind, state: "pending", ticks: 0 };
}

function refresh(p: Proc) {
  p.steps = [...p.commandSteps];
  if (p.cleanup) p.steps.push(p.cleanup);
  if (p.start) p.steps.push(p.start);
  if (p.health) p.steps.push(p.health);
}

function commandsFromScript(script: string, language: string, name: string, id: string): Mutable[] {
  const parts = splitInstallCommands(script);
  const steps = parts.map((cmd, i) => {
    const classified = classifyInstallCommand(cmd, language);
    return makeStep(`${id}-cmd-${i}`, labelFor(classified.kind, name, cmd), classified.kind as Kind, cmd);
  });
  const seen = new Map<string, number>();
  for (const s of steps) seen.set(s.label, (seen.get(s.label) || 0) + 1);
  for (const s of steps) {
    if ((seen.get(s.label) || 0) > 1 && s.detail) s.label = `${s.label} · ${s.detail}`;
  }
  return steps;
}

function makeProcess(app: ProgressApp, index: number): Proc {
  const name = (app.name || "app").trim() || "app";
  const id = `${name.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "") || "app"}-${index}`;
  const pm = (app.processManager || "").trim();
  const proc: Proc = {
    id,
    name,
    root: (app.root || "").trim(),
    language: app.language || "",
    plannedScript: "",
    commandSteps: [],
    steps: [],
  };
  if ((app.install || "").trim()) {
    proc.plannedScript = app.install!.trim();
    proc.commandSteps = commandsFromScript(proc.plannedScript, proc.language, name, id);
  }
  if ((app.cleanup || "").trim()) {
    proc.cleanup = makeStep(`${id}-cleanup`, `Cleaning ${name}`, "cleanup", app.cleanup);
  }
  if (pm === "pm2" || pm === "systemd" || pm === "docker") {
    proc.start = makeStep(`${id}-start`, `Starting ${name}`, "start");
  }
  if ((app.healthPath || "").trim()) {
    proc.health = makeStep(`${id}-health`, `Checking ${name}`, "health", app.healthPath);
  }
  refresh(proc);
  return proc;
}

function pathMatches(p: Proc, dir: string): boolean {
  const root = p.root.replace(/^\.\//, "").replace(/\/$/, "");
  if (root && root !== "." && (dir.endsWith("/" + root) || dir.endsWith(root))) return true;
  if (p.name && (dir.endsWith("/" + p.name) || dir.includes("/" + p.name + "/"))) return true;
  if (p.boundDir && p.boundDir === dir) return true;
  return false;
}

function lineStartsCommand(line: string, cmd: string, kind: string): boolean {
  const raw = line.trim();
  const low = raw.toLowerCase();
  const c = cmd.trim().toLowerCase();
  if (!c) return false;
  if (low.includes(c)) return true;
  if (kind === "build") {
    if (/^>\s+\S+@\S+\s+build\b/i.test(raw)) return true;
    if (/^>\s+(next|nuxt|vite|astro|tsc|webpack|ng|remix|nest)\b/i.test(raw)) return true;
    if (/creating an optimized production build/i.test(low)) return true;
    if (/compiled successfully/i.test(low)) return true;
  }
  const head = c.split(/\s+/)[0];
  if (kind === "command" && head && /^>\s+/.test(raw) && low.includes(head)) return true;
  return false;
}

function stepPercent(s: Mutable): number {
  if (s.state === "done") return 100;
  if (s.state === "pending" || s.state === "skipped") return 0;
  return Math.min(92, Math.round((0.18 + s.ticks * 0.02) * 100));
}

function rollup(steps: Mutable[]): number {
  if (steps.length === 0) return 0;
  let n = 0;
  for (const s of steps) {
    if (s.state === "done") n += 1;
    else if (s.state === "active" || s.state === "failed") n += stepPercent(s) / 100;
  }
  return Math.round((n / steps.length) * 100);
}

function rollupState(steps: Mutable[]): StepState {
  if (steps.some((s) => s.state === "failed")) return "failed";
  if (steps.some((s) => s.state === "active")) return "active";
  if (steps.length > 0 && steps.every((s) => s.state === "done" || s.state === "skipped")) {
    return steps.some((s) => s.state === "done") ? "done" : "skipped";
  }
  if (steps.some((s) => s.state === "done")) return "active";
  return "pending";
}

function freeze(s: Mutable): LiveStep {
  return {
    id: s.id,
    label: s.label,
    detail: s.detail,
    kind: s.kind,
    state: s.state,
    percent: stepPercent(s),
  };
}

export function buildDeployLive(input: LiveInput): LiveView {
  const singleApp = input.apps.length <= 1;
  const processes: Proc[] =
    input.apps.length > 0
      ? input.apps.map((app, i) => makeProcess(app, i))
      : [makeProcess({ name: input.projectName || "App", install: "" }, 0)];

  const shared = {
    preflight: makeStep("preflight", "Preflight", "preflight"),
    clone: makeStep("clone", "Clone", "clone"),
    release: makeStep("release", "Activate release", "release"),
  };

  function ordered(): Mutable[] {
    const out: Mutable[] = [shared.preflight, shared.clone];
    for (const p of processes) out.push(...p.commandSteps);
    for (const p of processes) if (p.cleanup) out.push(p.cleanup);
    out.push(shared.release);
    for (const p of processes) if (p.start) out.push(p.start);
    for (const p of processes) if (p.health) out.push(p.health);
    return out;
  }

  // A holder so control-flow analysis does not narrow this back to null
  // after the helper functions assign it.
  const cur: { step: Mutable | null } = { step: null };
  let failed = false;
  let pane = "shared";
  const lines: LiveLogLine[] = [];

  function ownerOf(step: Mutable): Proc | undefined {
    return processes.find(
      (p) =>
        p.commandSteps.includes(step) ||
        p.cleanup === step ||
        p.start === step ||
        p.health === step,
    );
  }

  function isShared(step: Mutable): boolean {
    return step === shared.preflight || step === shared.clone || step === shared.release;
  }

  function activate(step: Mutable | undefined) {
    if (!step || failed) return;
    if (step.state === "done" || step.state === "skipped" || step.state === "failed") return;
    // Parallel installs keep multiple apps active. Auto-complete the previous step
    // only within one process, or when leaving shared prepare steps for an app.
    if (cur.step && cur.step !== step && cur.step.state === "active") {
      const prevOwner = ownerOf(cur.step);
      const nextOwner = ownerOf(step);
      const sameProc = Boolean(prevOwner && nextOwner && prevOwner === nextOwner);
      const leaveShared = isShared(cur.step) && !isShared(step);
      const withinShared = isShared(cur.step) && isShared(step);
      if (sameProc || leaveShared || withinShared) cur.step.state = "done";
    }
    if (step.state === "pending") step.state = "active";
    cur.step = step;
  }

  function completeCommands(p: Proc) {
    if (failed) return;
    // A process that has not started yet must stay pending. Calling this when the
    // next app's install begins was marking every later app done.
    const started = p.commandSteps.some((s) => s.state === "active" || s.state === "done");
    if (!started) return;
    for (const s of p.commandSteps) {
      if (s.state === "active" || s.state === "pending") s.state = "done";
    }
    if (cur.step && p.commandSteps.includes(cur.step)) cur.step = null;
  }

  function failHere() {
    const all = ordered();
    let idx = cur.step ? all.indexOf(cur.step) : all.findIndex((s) => s.state === "active");
    if (idx < 0) {
      let lastDone = -1;
      all.forEach((s, i) => {
        if (s.state === "done") lastDone = i;
      });
      if (lastDone >= 0 && lastDone + 1 < all.length && all[lastDone + 1].state !== "done") idx = lastDone + 1;
      else idx = all.findIndex((s) => s.state === "pending" || s.state === "active");
    }
    if (idx >= 0 && all[idx].state !== "done") {
      all[idx].state = "failed";
      cur.step = all[idx];
    }
    const at = cur.step ? all.indexOf(cur.step) : idx;
    all.forEach((s, i) => {
      if (at < 0 || s.state === "failed") return;
      if (i < at && s.state !== "done") s.state = "done";
      if (i > at && (s.state === "pending" || s.state === "active")) s.state = "skipped";
    });
    failed = true;
  }

  function match(dir: string): Proc | undefined {
    for (const p of processes) if (pathMatches(p, dir)) return p;
    if (processes.length === 1 && singleApp && !processes[0].boundDir) {
      processes[0].boundDir = dir;
      return processes[0];
    }
    return undefined;
  }

  function ensure(dir: string): Proc {
    const found = match(dir);
    if (found) return found;
    const base = dir.split("/").filter(Boolean).pop() || "app";
    const created = makeProcess({ name: base, root: dir }, processes.length);
    created.boundDir = dir;
    processes.push(created);
    return created;
  }

  function setCommands(p: Proc, script: string) {
    const trimmed = script.trim();
    if (!trimmed) return;
    if (p.plannedScript === trimmed && p.commandSteps.length > 0) return;
    if (p.commandSteps.some((s) => s.state !== "pending")) return;
    p.plannedScript = trimmed;
    p.commandSteps = commandsFromScript(trimmed, p.language, p.name, p.id);
    refresh(p);
  }

  function maybeAdvance(p: Proc, line: string) {
    if (failed) return;
    const idx = p.commandSteps.findIndex((s) => s.state === "active");
    if (idx < 0) return;
    const next = p.commandSteps[idx + 1];
    if (next && lineStartsCommand(line, next.detail || "", next.kind)) {
      activate(next);
      return;
    }
    p.commandSteps[idx].ticks++;
  }

  function paneFor(step: Mutable): string {
    if (step === shared.preflight || step === shared.clone || step === shared.release) return "shared";
    return ownerOf(step)?.id || pane;
  }

  function claimStart(line: string): Proc | undefined {
    const named = line.match(/--name\s+(\S+)/);
    if (named) {
      const pmName = named[1];
      const hit = processes.find((p) => {
        if (!p.start) return false;
        if (p.name === pmName) return true;
        // Project-prefixed pm2 names: "shop-web" matches app "web".
        if (pmName.endsWith("-" + p.name)) return true;
        return false;
      });
      if (hit) return hit;
    }
    const unit = line.match(/enable --now\s+(\S+)/);
    if (unit) {
      const u = unit[1].replace(/\.service$/, "");
      const hit = processes.find((p) => {
        if (!p.start) return false;
        if (u === p.name || unit[1] === `${p.name}.service` || unit[1].startsWith(`${p.name}.`)) return true;
        if (u.endsWith("-" + p.name)) return true;
        return false;
      });
      if (hit) return hit;
    }
    return processes.find((p) => p.start && p.start.state === "pending");
  }

  const text = stripAnsi(
    [...input.logs]
      .sort((a, b) => a.seq - b.seq)
      .map((l) => l.chunk)
      .join(""),
  );
  const parts = text.split("\n");
  if (parts.length > 0 && parts[parts.length - 1] === "") parts.pop();

  for (const raw of parts) {
    let line = raw.trim();
    if (!line) {
      lines.push({ pane, text: "" });
      continue;
    }

    const tagged = line.match(IN_PREFIX_RE);
    if (tagged) {
      const dir = tagged[1].trim();
      const body = tagged[2] ?? "";
      const proc = ensure(dir);
      pane = proc.id;
      line = body.trim();
      if (!line) {
        lines.push({ pane, text: "" });
        continue;
      }
    }

    const failedLine = line.startsWith("FAILED —") || line.startsWith("FAILED -");
    const phase = line.match(PHASE_RE);

    if (failedLine) {
      lines.push({ pane: cur.step ? paneFor(cur.step) : pane, text: line });
      failHere();
      continue;
    }

    if (phase) {
      const kind = phase[1];
      const msg = phase[2].trim();
      if (kind === "clone" && msg.startsWith("removing tmp clone")) {
        pane = "shared";
        lines.push({ pane, text: line });
        continue;
      }
      if (kind === "preflight") {
        pane = "shared";
        activate(shared.preflight);
        shared.preflight.ticks++;
      } else if (kind === "clone") {
        pane = "shared";
        activate(shared.clone);
        if (/tmp clone ready|release /.test(msg)) {
          shared.clone.state = "done";
          if (cur.step === shared.clone) cur.step = null;
        } else {
          shared.clone.ticks++;
        }
      } else if (kind === "install") {
        const work = msg.match(/^(cleanup:\s+)?(.*)\s+\(in\s+([^)]+)\)\s*$/);
        if (work) {
          const isCleanup = Boolean(work[1]);
          const script = work[2].trim();
          const dir = work[3].trim();
          const proc = ensure(dir);
          if (isCleanup) {
            completeCommands(proc);
            if (!proc.cleanup) {
              proc.cleanup = makeStep(`${proc.id}-cleanup`, `Cleaning ${proc.name}`, "cleanup", script);
              refresh(proc);
            }
            activate(proc.cleanup);
          } else if (script === "done") {
            completeCommands(proc);
          } else {
            // Do not complete sibling apps: installs run in parallel.
            setCommands(proc, script);
            if (proc.commandSteps[0]) activate(proc.commandSteps[0]);
          }
          pane = proc.id;
        } else {
          pane = "shared";
        }
      } else if (kind === "release") {
        pane = "shared";
        if (msg.startsWith("current ->")) {
          for (const p of processes) completeCommands(p);
          if (shared.release.state !== "done") {
            activate(shared.release);
            shared.release.state = "done";
            if (cur.step === shared.release) cur.step = null;
          }
        } else if (!msg.startsWith("pruned") && shared.release.state !== "done") {
          for (const p of processes) completeCommands(p);
          activate(shared.release);
          shared.release.ticks++;
        }
      } else if (kind === "health") {
        const healths = processes.map((p) => p.health).filter((s): s is Mutable => Boolean(s));
        const open = healths.find((s) => s.state === "pending" || s.state === "active");
        if (/^healthy:/.test(msg) && open) {
          activate(open);
          open.state = "done";
          if (cur.step === open) cur.step = null;
          const owner = processes.find((p) => p.health === open);
          pane = owner?.id || "shared";
        } else if (/^skipped:/.test(msg) && open) {
          open.state = "skipped";
          pane = "shared";
        } else if (open) {
          activate(open);
          open.ticks++;
          const owner = processes.find((p) => p.health === open);
          pane = owner?.id || "shared";
        } else {
          pane = "shared";
        }
      } else if (kind === "start") {
        pane = "shared";
      }
      lines.push({ pane, text: line });
      continue;
    }

    if (/^pm2 (save|startup)\b/.test(line)) {
      if (cur.step && cur.step.kind === "start") cur.step.ticks++;
      lines.push({ pane, text: line });
      continue;
    }

    if (START_RE.test(line)) {
      const proc = claimStart(line);
      if (proc?.start) {
        if (shared.release.state === "active") {
          shared.release.state = "done";
          if (cur.step === shared.release) cur.step = null;
        }
        if (line.startsWith("process_manager=none")) {
          proc.start.state = "skipped";
        } else {
          activate(proc.start);
          proc.start.ticks++;
        }
        pane = proc.id;
      }
      lines.push({ pane, text: line });
      continue;
    }

    const owner = processes.find((p) => p.id === pane);
    if (owner) maybeAdvance(owner, line);
    else if (cur.step && !failed) cur.step.ticks++;
    lines.push({ pane, text: line });
  }

  if (input.status === "succeeded") {
    for (const s of ordered()) {
      if (s.state === "pending" || s.state === "active") s.state = "done";
    }
  } else if (input.status === "failed" || input.status === "canceled" || input.status === "agent_offline") {
    failHere();
  }

  const all = ordered();
  let percent = rollup(all);
  if (input.status === "succeeded") percent = 100;
  else if ((input.status === "running" || input.status === "pending") && percent >= 100) percent = 99;

  const activeLabels = all.filter((s) => s.state === "active").map((s) => s.label);
  const failedStep = all.find((s) => s.state === "failed");
  let headline = "Waiting for the agent";
  if (activeLabels.length > 0) headline = activeLabels.join(" · ");
  else if (failedStep) headline = failedStep.label;
  else if (input.status === "succeeded") headline = "Finished";
  else if (input.status === "canceled") headline = "Canceled";
  else if (all.some((s) => s.state === "done")) headline = "Working";

  return {
    percent,
    headline,
    shared: [shared.preflight, shared.clone, shared.release].map(freeze),
    processes: processes.map((p) => ({
      id: p.id,
      name: p.name,
      state: rollupState(p.steps),
      percent: input.status === "succeeded" ? 100 : rollup(p.steps),
      steps: p.steps.map(freeze),
    })),
    lines,
  };
}

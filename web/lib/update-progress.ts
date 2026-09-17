export type UpdatePhase =
  | "offered"
  | "fetching"
  | "cloning"
  | "downloading"
  | "building"
  | "installing"
  | "migrating"
  | "stopping"
  | "restarting"
  | "verifying"
  | "done"
  | "failed"
  | "timeout";

export type StepState = "done" | "current" | "pending" | "error";

export type PipelineStep = {
  id: UpdatePhase;
  label: string;
  hint: string;
};

const STACK: PipelineStep[] = [
  { id: "offered", label: "Sending update", hint: "Telling the agent to start the upgrade" },
  { id: "fetching", label: "Fetching the release", hint: "Checking out the new git tag" },
  { id: "downloading", label: "Downloading images and files", hint: "Docker images, npm packages, and source files" },
  { id: "building", label: "Building the new version", hint: "Compiling binaries and the web UI" },
  { id: "migrating", label: "Applying migrations", hint: "Updating the database schema" },
  { id: "stopping", label: "Stopping the server", hint: "Shutting down running services" },
  { id: "restarting", label: "Restarting services", hint: "Bringing the stack back up" },
  { id: "verifying", label: "Verifying", hint: "Confirming the new version is running" },
];

const AGENT: PipelineStep[] = [
  { id: "offered", label: "Sending update", hint: "Telling the agent to start the upgrade" },
  { id: "cloning", label: "Cloning the release", hint: "Downloading the tagged source" },
  { id: "building", label: "Building from source", hint: "Compiling the new agent" },
  { id: "installing", label: "Installing the binary", hint: "Replacing the running agent" },
  { id: "restarting", label: "Restarting the agent", hint: "Supervisor bringing the new process up" },
  { id: "verifying", label: "Verifying", hint: "Confirming the new version is running" },
];

const ORDER: UpdatePhase[] = [
  "offered",
  "fetching",
  "cloning",
  "downloading",
  "building",
  "installing",
  "migrating",
  "stopping",
  "restarting",
  "verifying",
  "done",
];

export function pipelineFor(stack: boolean): PipelineStep[] {
  return stack ? STACK : AGENT;
}

function rank(phase: UpdatePhase): number {
  const i = ORDER.indexOf(phase);
  return i < 0 ? 0 : i;
}

/** Map a live agent phase onto the pipeline this surface actually shows. */
export function effectivePhase(phase: UpdatePhase, stack: boolean): UpdatePhase {
  if (phase === "timeout" || phase === "failed" || phase === "done") return phase;
  if (stack) {
    if (phase === "cloning") return "fetching";
    if (phase === "installing") return "building";
    return phase;
  }
  if (phase === "fetching" || phase === "downloading") return "cloning";
  if (phase === "migrating" || phase === "stopping") return "restarting";
  return phase;
}

export function stepState(stepId: UpdatePhase, current: UpdatePhase): StepState {
  if (current === "failed" || current === "timeout") {
    const s = rank(stepId);
    const c = rank(current === "failed" || current === "timeout" ? "restarting" : current);
    if (s < c) return "done";
    if (s === c) return "error";
    return "pending";
  }
  if (current === "done") return "done";
  const s = rank(stepId);
  const c = rank(current);
  if (s < c) return "done";
  if (s === c) return "current";
  return "pending";
}

export function percentForPhase(phase: UpdatePhase, reported?: number): number {
  if (typeof reported === "number" && reported > 0) {
    return Math.max(0, Math.min(100, reported));
  }
  if (phase === "done") return 100;
  const i = rank(phase);
  if (i <= 0) return 5;
  return Math.min(95, Math.round((i / (ORDER.length - 1)) * 100));
}

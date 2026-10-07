// Builds the human-readable deploy plan shown on the Review step. The plan is
// grouped into main blocks (prepare, one block per app directory, release swap,
// policies) with sub-steps that mirror what the agent will do: install packages,
// build, stage under the clone path, apply env, start the process manager, health.
import type { AdvancedSettings } from "./deploy-spec";
import type { ProjectBlock } from "./project-blocks";
import type { DeployEnvVar } from "./types";

export type DeployStepKind =
  | "preflight"
  | "clone"
  | "install"
  | "build"
  | "command"
  | "stage"
  | "env"
  | "start"
  | "health"
  | "rollback"
  | "timeout";

export type DeployStep = {
  id: string;
  title: string;
  detail?: string;
  kind?: DeployStepKind;
};

/** One main block on the review screen (prepare, an app directory, release, …). */
export type DeployPlanBlock = {
  id: string;
  title: string;
  /** e.g. working directory for an app block. */
  subtitle?: string;
  kind: "setup" | "app" | "release" | "policy";
  steps: DeployStep[];
};

export type DeployStepsInput = {
  repo: string;
  branch: string;
  serverName: string;
  clonePath: string;
  blocks: ProjectBlock[];
  advanced: AdvancedSettings;
  /** Per-app env from the configure form, keyed by app name. */
  appEnv?: Record<string, DeployEnvVar[]>;
};

function pmLabel(pm: string): string {
  switch (pm) {
    case "pm2":
      return "PM2";
    case "systemd":
      return "systemd";
    case "docker":
      return "Docker";
    default:
      return "no process manager";
  }
}

function langInstallLabel(language: string): string {
  const lang = (language || "").toLowerCase();
  if (lang.includes("node") || lang.includes("javascript") || lang.includes("typescript")) {
    return "Install npm packages";
  }
  if (lang.includes("go") || lang === "golang") return "Download Go modules";
  if (lang.includes("python") || lang.includes("django") || lang.includes("flask")) {
    return "Install Python packages";
  }
  if (lang.includes("rust")) return "Fetch Rust crates";
  if (lang.includes("ruby")) return "Install Ruby gems";
  return "Install dependencies";
}

function langBuildLabel(language: string): string {
  const lang = (language || "").toLowerCase();
  if (lang.includes("go") || lang === "golang") return "Build Go binary";
  if (lang.includes("rust")) return "Build Rust release";
  return "Build";
}

/** Split an install script into discrete commands (&&-chained). */
export function splitInstallCommands(script: string): string[] {
  const trimmed = script.trim();
  if (!trimmed) return [];
  // Prefer && chains (the common detected form). Fall back to newlines.
  const parts = trimmed.includes("&&")
    ? trimmed.split(/\s*&&\s*/)
    : trimmed.split(/\n+/);
  return parts.map((p) => p.trim()).filter(Boolean);
}

type Classified = { kind: DeployStepKind; title: string };

/** Classify one shell command for the review sub-step title. */
export function classifyInstallCommand(cmd: string, language: string): Classified {
  const c = cmd.trim().toLowerCase();
  if (
    /^(npm|pnpm|yarn|bun)\s+(ci|install|i)\b/.test(c) ||
    /^go\s+mod\s+(download|tidy|vendor)\b/.test(c) ||
    /^(pip3?|pipenv|poetry|uv)\s+(install|sync|add)\b/.test(c) ||
    /^bundle\s+install\b/.test(c) ||
    /^cargo\s+fetch\b/.test(c)
  ) {
    return { kind: "install", title: langInstallLabel(language) };
  }
  if (
    /\b(build|compile)\b/.test(c) ||
    /^go\s+build\b/.test(c) ||
    /^cargo\s+build\b/.test(c) ||
    /^(tsc|vite|webpack|next|nuxt|astro)\b/.test(c) ||
    /^npm\s+run\s+build\b/.test(c) ||
    /^pnpm\s+(run\s+)?build\b/.test(c) ||
    /^yarn\s+(run\s+)?build\b/.test(c)
  ) {
    return { kind: "build", title: langBuildLabel(language) };
  }
  return { kind: "command", title: "Run command" };
}

function envCount(vars: DeployEnvVar[] | undefined): number {
  return (vars || []).filter((v) => v.key.trim()).length;
}

function rootLabel(root: string): string {
  const r = (root || ".").trim() || ".";
  return r === "." ? "repository root" : r;
}

/**
 * Hierarchical plan for the confirmation screen. Main blocks are prepare, each
 * app directory (install → build → … → start), the atomic release swap, then
 * optional policies.
 */
export function buildDeployPlan(input: DeployStepsInput): DeployPlanBlock[] {
  const branch = input.branch.trim() || "main";
  const clone = input.clonePath.trim() || "(language default path)";
  const server = input.serverName.trim() || "selected server";
  const blocks = input.blocks.length ? input.blocks : [];
  const sections: DeployPlanBlock[] = [];

  sections.push({
    id: "setup",
    title: "Prepare on the server",
    subtitle: server,
    kind: "setup",
    steps: [
      {
        id: "preflight",
        kind: "preflight",
        title: "Preflight checks",
        detail: `Confirm ${server} can clone the repo and run install / process-manager steps.`,
      },
      {
        id: "clone",
        kind: "clone",
        title: "Clone the repository",
        detail: `Check out ${input.repo} @ ${branch} into a new release under ${clone}.`,
      },
    ],
  });

  for (const b of blocks) {
    const root = b.root || ".";
    const work = rootLabel(root);
    const name = (b.name || root || "app").trim() || "app";
    const buildSteps: DeployStep[] = [];
    const commands = splitInstallCommands(b.install);

    if (commands.length === 0) {
      buildSteps.push({
        id: `install-${b.id}-skip`,
        kind: "install",
        title: "Install & build",
        detail: "No install script — skip if the release is already built.",
      });
    } else {
      commands.forEach((cmd, i) => {
        const classified = classifyInstallCommand(cmd, b.language);
        buildSteps.push({
          id: `install-${b.id}-${i}`,
          kind: classified.kind,
          title: classified.title,
          detail: cmd,
        });
      });
    }

    buildSteps.push({
      id: `stage-${b.id}`,
      kind: "stage",
      title: "Keep artifacts in this release",
      detail: `Output stays under ${clone} until the release is activated for every app.`,
    });

    sections.push({
      id: `build-${b.id}`,
      title: `Build ${name}`,
      subtitle: `Working directory: ${work}`,
      kind: "app",
      steps: buildSteps,
    });
  }

  sections.push({
    id: "release",
    title: "Activate the release",
    subtitle: clone,
    kind: "release",
    steps: [
      {
        id: "release-swap",
        kind: "stage",
        title: "Atomic symlink swap",
        detail: `Point ${clone}/current at the new release once every app install succeeds — failed installs leave the previous release live.`,
      },
    ],
  });

  for (const b of blocks) {
    const root = b.root || ".";
    const work = rootLabel(root);
    const name = (b.name || root || "app").trim() || "app";
    const runSteps: DeployStep[] = [];

    const nEnv = envCount(input.appEnv?.[b.name]);
    runSteps.push({
      id: `env-${b.id}`,
      kind: "env",
      title: "Apply environment variables",
      detail:
        nEnv > 0
          ? `Inject ${nEnv} configured variable${nEnv === 1 ? "" : "s"} (plus PORT when set) when the process starts.`
          : Number(b.port) > 0
            ? `No custom env yet — set PORT=${b.port} when starting.`
            : "No custom env configured for this app.",
    });

    const pm = b.processManager || "none";
    const port = Number(b.port) || 0;
    const runCmd = (b.run || "").trim();
    runSteps.push({
      id: `start-${b.id}`,
      kind: "start",
      title: pm === "none" ? "Leave as install result" : `Start with ${pmLabel(pm)}`,
      detail:
        pm === "none"
          ? "No process manager — the install output is the deploy result."
          : runCmd
            ? `Run \`${runCmd}\` via ${pmLabel(pm)}${port > 0 ? ` (PORT=${port})` : ""} in the activated deploy folder.`
            : port > 0
              ? `Start via ${pmLabel(pm)} on port ${port}.`
              : `Start via ${pmLabel(pm)}.`,
    });

    const healthPath = b.healthPath.trim() || (blocks.length === 1 ? input.advanced.healthPath.trim() : "");
    if (healthPath) {
      const healthPort =
        Number(b.healthPort) ||
        Number(b.port) ||
        Number(input.advanced.healthPort) ||
        0;
      const timeout =
        Number(b.healthTimeout) || Number(input.advanced.healthTimeout) || 60;
      runSteps.push({
        id: `health-${b.id}`,
        kind: "health",
        title: "Health check",
        detail: `Wait up to ${timeout}s for 127.0.0.1:${healthPort || "…"}${healthPath}.`,
      });
    }

    sections.push({
      id: `run-${b.id}`,
      title: `Run ${name}`,
      subtitle: `Working directory: ${work}`,
      kind: "app",
      steps: runSteps,
    });
  }

  const policySteps: DeployStep[] = [];
  if (input.advanced.autoRollback) {
    policySteps.push({
      id: "rollback",
      kind: "rollback",
      title: "Auto-rollback on failure",
      detail: "If a later step fails, redeploy the last commit that succeeded.",
    });
  }
  const budget = Number(input.advanced.deployTimeout) || 0;
  if (budget > 0) {
    policySteps.push({
      id: "timeout",
      kind: "timeout",
      title: "Deploy time budget",
      detail: `Whole run must finish within ${budget} seconds.`,
    });
  }
  // Project-level health when no per-app path was set and there are multiple apps.
  const projectHealth = input.advanced.healthPath.trim();
  if (projectHealth && blocks.length > 1 && !blocks.some((b) => b.healthPath.trim())) {
    const port = Number(input.advanced.healthPort) || Number(blocks[0]?.port) || 0;
    const timeout = Number(input.advanced.healthTimeout) || 60;
    policySteps.push({
      id: "health",
      kind: "health",
      title: "Shared health check",
      detail: `Wait up to ${timeout}s for 127.0.0.1:${port || "…"}${projectHealth}.`,
    });
  }
  if (policySteps.length > 0) {
    sections.push({
      id: "policy",
      title: "Policies",
      kind: "policy",
      steps: policySteps,
    });
  }

  return sections;
}

/** Flat list of every sub-step (tests and any caller that wants a linear view). */
export function buildDeploySteps(input: DeployStepsInput): DeployStep[] {
  return buildDeployPlan(input).flatMap((block) => block.steps);
}

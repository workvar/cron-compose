// Builds the human-readable deploy plan shown on the Review step. Steps mirror
// the agent's real phases (preflight → clone → install → release → start → health)
// so the confirmation screen matches what the run log will look like.
import type { AdvancedSettings } from "./deploy-spec";
import type { ProjectBlock } from "./project-blocks";

export type DeployStep = {
  id: string;
  title: string;
  detail?: string;
};

export type DeployStepsInput = {
  repo: string;
  branch: string;
  serverName: string;
  clonePath: string;
  blocks: ProjectBlock[];
  advanced: AdvancedSettings;
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

/** Ordered steps the agent will take for this form / croncompose.yml. */
export function buildDeploySteps(input: DeployStepsInput): DeployStep[] {
  const branch = input.branch.trim() || "main";
  const clone = input.clonePath.trim() || "(language default path)";
  const server = input.serverName.trim() || "selected server";
  const blocks = input.blocks.length ? input.blocks : [];
  const steps: DeployStep[] = [];

  steps.push({
    id: "preflight",
    title: "Preflight on the server",
    detail: `Check ${server} can clone the repo and run the install / ${pmLabel(blocks[0]?.processManager || "none")} steps.`,
  });

  steps.push({
    id: "clone",
    title: "Clone the repository",
    detail: `Check out ${input.repo} @ ${branch} into ${clone}.`,
  });

  for (const b of blocks) {
    const root = b.root || ".";
    const install = b.install.trim();
    steps.push({
      id: `install-${b.id}`,
      title: blocks.length > 1 ? `Install ${b.name || root}` : "Install dependencies & build",
      detail: install
        ? `In ${root}: ${install}`
        : `In ${root}: no install script (skip if the release is already built).`,
    });
  }

  steps.push({
    id: "release",
    title: "Activate the release",
    detail: "Point the live checkout at the new release once every install succeeds.",
  });

  for (const b of blocks) {
    const pm = b.processManager || "none";
    const port = Number(b.port) || 0;
    const portBit = port > 0 ? ` on port ${port}` : "";
    steps.push({
      id: `start-${b.id}`,
      title: blocks.length > 1 ? `Start ${b.name || b.root || "app"}` : "Start the app",
      detail:
        pm === "none"
          ? "No process manager — leave the install output as the deploy result."
          : `Start with ${pmLabel(pm)}${portBit}.`,
    });
  }

  const healthApps = blocks.filter((b) => b.healthPath.trim());
  const projectHealth = input.advanced.healthPath.trim();
  if (healthApps.length > 0) {
    for (const b of healthApps) {
      const port = Number(b.healthPort) || Number(b.port) || 0;
      const timeout = Number(b.healthTimeout) || 60;
      steps.push({
        id: `health-${b.id}`,
        title: blocks.length > 1 ? `Health check ${b.name || b.root}` : "Health check",
        detail: `Wait up to ${timeout}s for 127.0.0.1:${port || "…"}${b.healthPath.trim()}.`,
      });
    }
  } else if (projectHealth) {
    const port = Number(input.advanced.healthPort) || Number(blocks[0]?.port) || 0;
    const timeout = Number(input.advanced.healthTimeout) || 60;
    steps.push({
      id: "health",
      title: "Health check",
      detail: `Wait up to ${timeout}s for 127.0.0.1:${port || "…"}${projectHealth}.`,
    });
  }

  if (input.advanced.autoRollback) {
    steps.push({
      id: "rollback",
      title: "Auto-rollback on failure",
      detail: "If a later step fails, redeploy the last commit that succeeded.",
    });
  }

  const budget = Number(input.advanced.deployTimeout) || 0;
  if (budget > 0) {
    steps.push({
      id: "timeout",
      title: "Deploy time budget",
      detail: `Whole run must finish within ${budget} seconds.`,
    });
  }

  return steps;
}

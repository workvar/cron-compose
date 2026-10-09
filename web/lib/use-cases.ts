export type UseCase = {
  slug: string;
  badge: string;
  title: string;
  summary: string;
  problem: string;
  solution: string;
  outcomes: string[];
};

export const USE_CASES: UseCase[] = [
  {
    slug: "fleet",
    badge: "Fleet ops",
    title: "One pane for every Linux host",
    summary: "Enroll agents, watch health, and jump into terminals without juggling SSH configs.",
    problem:
      "SSH configs, bastions, and one-off scripts scatter across laptops. Nobody has a single answer for “which hosts are online and what are they running?”",
    solution:
      "CronCompose agents report status to the control plane. You enroll once, then browse the fleet, open terminals, and act on processes from one UI.",
    outcomes: [
      "Live online / offline status per host",
      "Audited terminal sessions",
      "Labels and selectors for targeting jobs and deploys",
    ],
  },
  {
    slug: "deploys",
    badge: "Deploys",
    title: "Import a repo. Ship a process.",
    summary: "Build once, deploy to matching hosts with PM2, systemd, or Docker.",
    problem:
      "Every service has a different deploy script, and rollbacks mean guessing which tarball was last live.",
    solution:
      "A croncompose.yml (or the Deploy wizard) describes install, run, env, and process manager. Redeploy from the UI or on git events.",
    outcomes: [
      "Reproducible builds per release",
      "PM2, systemd, and Docker support",
      "Redeploy on branch, tag, or release",
    ],
  },
  {
    slug: "jobs",
    badge: "Jobs",
    title: "Schedules that keep their promises",
    summary: "Define cron once. Agents keep firing offline-first and sync when they reconnect.",
    problem:
      "Crontab files drift per machine. When a box goes offline, schedules silently stop and nobody notices until customers do.",
    solution:
      "Jobs are defined in the control plane and synced to agents. Each agent runs locally even without connectivity, then reports results when it can.",
    outcomes: [
      "Central definitions, local execution",
      "Run history and logs in one place",
      "Offline-safe schedules",
    ],
  },
  {
    slug: "connectors",
    badge: "Connectors",
    title: "Act on running processes",
    summary: "Restart, inspect logs, and manage PM2, systemd, or Docker objects without leaving CronCompose.",
    problem:
      "Day-two ops still means SSH and remembering which tool owns which process on which host.",
    solution:
      "Connectors expose process managers as first-class objects: list, restart, read logs, and keep an audit trail.",
    outcomes: [
      "Unified process actions",
      "Log streaming and history",
      "Audited operator actions",
    ],
  },
];

export function getUseCase(slug: string): UseCase | undefined {
  return USE_CASES.find((c) => c.slug === slug);
}

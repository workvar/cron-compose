// Shared state + helpers for the job-creation wizard.

export type TargetKind = "server" | "labels";

export type JobDraft = {
  targetKind: TargetKind;
  targetLabels: string; // raw "k=v, k2=v2"
  name: string;
  description: string;
  interpreter: string;
  scriptBody: string;
  scheduleCron: string;
  timezone: string;
  catchupPolicy: "skip" | "once" | "all";
  // advanced
  timeoutSeconds: number;
  concurrencyPolicy: "skip" | "allow" | "queue";
  maxRetries: number;
  cpuPct: number;
  memMB: number;
  tasksMax: number;
  ioWeight: number;
  workingDir: string;
  runAsUser: string;
  secretRefs: string[];
};

export function initialDraft(): JobDraft {
  return {
    targetKind: "server",
    targetLabels: "",
    name: "",
    description: "",
    interpreter: "bash",
    scriptBody: '#!/usr/bin/env bash\nset -euo pipefail\n\necho "hello from cron"\n',
    scheduleCron: "0 */6 * * *",
    timezone: "UTC",
    catchupPolicy: "skip",
    timeoutSeconds: 3600,
    concurrencyPolicy: "skip",
    maxRetries: 0,
    cpuPct: 0,
    memMB: 0,
    tasksMax: 0,
    ioWeight: 0,
    workingDir: "",
    runAsUser: "",
    secretRefs: [],
  };
}

export type Patch = Partial<JobDraft>;

export function parseLabels(input: string): Record<string, string> {
  const out: Record<string, string> = {};
  for (const part of input.split(",")) {
    const trimmed = part.trim();
    if (!trimmed) continue;
    const eq = trimmed.indexOf("=");
    if (eq < 0) continue;
    out[trimmed.slice(0, eq).trim()] = trimmed.slice(eq + 1).trim();
  }
  return out;
}

export const CRON_PRESETS: { label: string; cron: string }[] = [
  { label: "Every 15 min", cron: "*/15 * * * *" },
  { label: "Hourly", cron: "0 * * * *" },
  { label: "Every 6 hours", cron: "0 */6 * * *" },
  { label: "Daily · midnight", cron: "0 0 * * *" },
  { label: "Daily · 9am", cron: "0 9 * * *" },
  { label: "Weekly · Mon", cron: "0 0 * * 1" },
  { label: "Monthly · 1st", cron: "0 0 1 * *" },
];

export const INTERPRETERS = ["bash", "sh", "python3", "node"];

export const COMMON_TZ = [
  "UTC", "America/New_York", "America/Los_Angeles", "America/Chicago",
  "Europe/London", "Europe/Berlin", "Asia/Kolkata", "Asia/Tokyo", "Australia/Sydney",
];

// A friendly description for the known presets; otherwise echo the raw expression.
export function describeCron(cron: string): string {
  const hit = CRON_PRESETS.find((p) => p.cron === cron.trim());
  if (hit) return hit.label.replace(" · ", " at ");
  return `Custom: ${cron.trim() || "(empty)"}`;
}

// Loose validity check: cron must have 5 whitespace-separated fields.
export function isValidCron(cron: string): boolean {
  return cron.trim().split(/\s+/).length === 5;
}

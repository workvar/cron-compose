import type { DeployHealthState } from "@/lib/types";

// The project's standing, which is not the same as its last run's status: a project
// whose newest run failed and then rolled back is running fine, on older code, and
// both halves of that matter. "unknown" renders nothing rather than a badge saying
// nothing is known, so a project that has never deployed stays quiet.
const label: Record<DeployHealthState, string> = {
  unknown: "",
  healthy: "healthy",
  degraded: "degraded",
  rolled_back: "rolled back",
};

const tone: Record<DeployHealthState, string> = {
  unknown: "neutral",
  healthy: "ok",
  degraded: "danger",
  rolled_back: "warn",
};

const hint: Record<DeployHealthState, string> = {
  unknown: "",
  healthy: "The last deploy succeeded.",
  degraded: "The last deploy failed and nothing has recovered it: the failed deploy is what is live.",
  rolled_back: "Running the previous commit after an automatic rollback, not the newest one.",
};

export function HealthBadge({ state }: { state?: DeployHealthState }) {
  if (!state || state === "unknown") return null;
  return (
    <span className={`status ${tone[state]}`} title={hint[state]}>
      {label[state]}
    </span>
  );
}

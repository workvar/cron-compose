import type { UpdateServerStatus, UpdateStatus } from "./types";

export type UpdatePanelSummary = {
  latest?: string;
  releaseUrl?: string;
  repo?: string;
  checkError?: string;
  pending: UpdateServerStatus[];
  updatable: UpdateServerStatus[];
  current: UpdateServerStatus[];
  headline: string;
};

export function summarizeUpdates(status: UpdateStatus | null): UpdatePanelSummary {
  if (!status) {
    return {
      pending: [],
      updatable: [],
      current: [],
      checkError: "Could not reach the control plane.",
      headline: "Update status is unavailable.",
    };
  }

  const pending = status.items.filter((s) => s.update_available);
  const updatable = pending.filter((s) => s.can_update);
  const current = status.items.filter((s) => !s.update_available);

  let headline: string;
  if (status.check_error && !status.latest_version) {
    headline = "Could not check GitHub for a newer release.";
  } else if (status.latest_version && pending.length === 0 && status.items.length > 0) {
    const unversioned = status.items.filter((s) => !s.current_version).length;
    if (unversioned === status.items.length) {
      headline = `Latest release is ${status.latest_version}. Hosts have not reported a version yet.`;
    } else {
      headline = `Everything is on ${status.latest_version}.`;
    }
  } else if (pending.length > 0) {
    const stacks = pending.filter((s) => s.stack).length;
    if (stacks > 0) {
      headline = `${pending.length} host${pending.length === 1 ? "" : "s"} can build ${status.latest_version} from source.`;
    } else {
      headline = `${pending.length} agent${pending.length === 1 ? "" : "s"} can build ${status.latest_version} from source.`;
    }
  } else if (status.latest_version) {
    headline = `Latest release is ${status.latest_version}.`;
  } else {
    headline = "No GitHub release found yet.";
  }

  return {
    latest: status.latest_version,
    releaseUrl: status.release_url,
    repo: status.repo,
    checkError: status.check_error,
    pending,
    updatable,
    current,
    headline,
  };
}

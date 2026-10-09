import { processStateLabel, processStateTone } from "@/lib/process-state";

export function ProcessStateBadge({ state }: { state?: string | null }) {
  const label = processStateLabel(state);
  return (
    <span className={`status ${processStateTone(state)}`} title={label}>
      {label}
    </span>
  );
}

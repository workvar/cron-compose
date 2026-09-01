import type { JobDraft } from "./types";
import { describeCron, parseLabels } from "./types";

function Item({ label, value }: { label: string; value: string }) {
  return (
    <div className="review-item">
      <div className="ri-label">{label}</div>
      <div className="ri-value">{value}</div>
    </div>
  );
}

export function StepReview({ draft, serverName }: { draft: JobDraft; serverName?: string }) {
  const target =
    draft.targetKind === "server"
      ? serverName ?? "This server"
      : Object.entries(parseLabels(draft.targetLabels)).map(([k, v]) => `${k}=${v}`).join(", ") || "(no labels)";

  const limits =
    [
      draft.timeoutSeconds ? `${draft.timeoutSeconds}s timeout` : null,
      draft.cpuPct ? `${draft.cpuPct}% CPU` : null,
      draft.memMB ? `${draft.memMB} MB` : null,
      draft.tasksMax ? `${draft.tasksMax} procs` : null,
      draft.ioWeight ? `IO weight ${draft.ioWeight}` : null,
      draft.maxRetries ? `${draft.maxRetries} retries` : null,
    ].filter(Boolean).join(" · ") || "defaults";

  return (
    <div>
      <h2 className="step-h">Review &amp; create</h2>
      <p className="step-lead">Double-check the details, then create the job.</p>

      <div className="review-grid">
        <Item label="Name" value={draft.name || "(unnamed)"} />
        <Item label="Target" value={target} />
        <Item label="Schedule" value={describeCron(draft.scheduleCron)} />
        <Item label="Timezone" value={draft.timezone || "UTC"} />
        <Item label="Interpreter" value={draft.interpreter} />
        <Item label="Concurrency / catch-up" value={`${draft.concurrencyPolicy} / ${draft.catchupPolicy}`} />
        <Item label="Limits" value={limits} />
        <Item label="Secrets" value={draft.secretRefs.length ? draft.secretRefs.join(", ") : "none"} />
      </div>

      {draft.description && <p className="subtle" style={{ marginTop: 14 }}>{draft.description}</p>}

      <div className="review-script">{draft.scriptBody}</div>
    </div>
  );
}

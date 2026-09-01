import type { ConnectorStep } from "@/lib/types";

/**
 * The per-stage report from an operation (backup, validate, write, activate, health,
 * rollback). Showing every stage, not just the failing one, is what makes a rollback
 * legible: the operator can see that the write happened and was then undone.
 */
export function StepList({ steps }: { steps: ConnectorStep[] }) {
  if (!steps || steps.length === 0) return null;
  return (
    <ol className="step-list">
      {steps.map((s, i) => (
        <li key={`${s.name}-${i}`} className={s.ok ? "step ok" : "step bad"}>
          <span className="step-name">{s.name}</span>
          {s.output && <pre className="step-output">{s.output}</pre>}
        </li>
      ))}
    </ol>
  );
}

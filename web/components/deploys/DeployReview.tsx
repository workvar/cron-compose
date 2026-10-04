"use client";

import type { DeployStep } from "@/lib/deploy-steps";

type Props = {
  repo: string;
  branch: string;
  serverName: string;
  specPath?: string | null;
  steps: DeployStep[];
  issues?: { level: "error" | "warning"; field?: string; message: string }[];
};

/** Confirmation screen: what the croncompose.yml (or form) will do on the agent. */
export function DeployReview({ repo, branch, serverName, specPath, steps, issues }: Props) {
  const warnings = (issues || []).filter((i) => i.level === "warning");
  return (
    <div className="panel config-card">
      <div className="card-title">Review deploy plan</div>
      <p className="subtle" style={{ marginTop: 6 }}>
        These are the steps the agent will run on <strong>{serverName || "the selected server"}</strong>
        {specPath ? (
          <>
            {" "}
            from <code>{specPath}</code>
          </>
        ) : null}
        .
      </p>
      <div className="cluster" style={{ marginTop: 12, marginBottom: 4 }}>
        <span className="pill">{repo}</span>
        <span className="pill">{branch || "main"}</span>
        {serverName && <span className="pill">{serverName}</span>}
      </div>

      {warnings.length > 0 && (
        <ul className="issue-list" style={{ marginBottom: 14 }}>
          {warnings.map((is, i) => (
            <li key={i} className="issue warning">
              {is.field && !is.message.includes(is.field) && <code>{is.field}</code>} {is.message}
            </li>
          ))}
        </ul>
      )}

      <ol className="deploy-steps">
        {steps.map((s, i) => (
          <li key={s.id} className="deploy-step">
            <span className="deploy-step-n" aria-hidden>
              {i + 1}
            </span>
            <div>
              <div className="deploy-step-title">{s.title}</div>
              {s.detail && <div className="deploy-step-detail">{s.detail}</div>}
            </div>
          </li>
        ))}
      </ol>
    </div>
  );
}

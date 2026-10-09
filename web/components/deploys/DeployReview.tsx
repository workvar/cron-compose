"use client";

import type { DeployPlanBlock, DeployStepKind } from "@/lib/deploy-steps";

type Props = {
  repo: string;
  branch: string;
  serverName: string;
  specPath?: string | null;
  plan: DeployPlanBlock[];
  issues?: { level: "error" | "warning"; field?: string; message: string }[];
};

function kindClass(kind?: DeployStepKind): string {
  if (!kind) return "";
  return ` deploy-substep-${kind}`;
}

/** Confirmation screen: hierarchical plan from croncompose.yml / the form. */
export function DeployReview({ repo, branch, serverName, specPath, plan, issues }: Props) {
  const warnings = (issues || []).filter((i) => i.level === "warning");
  let globalIndex = 0;

  return (
    <div className="panel config-card">
      <div className="card-title">Review deploy plan</div>
      <p className="subtle" style={{ marginTop: 6 }}>
        Steps the agent will run on <strong>{serverName || "the selected server"}</strong>
        {specPath ? (
          <>
            {" "}
            from <code>{specPath}</code>
          </>
        ) : null}
        . Each app directory is its own block: installs build in parallel, then after the
        release is activated, env → start → health.
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

      <div className="deploy-plan">
        {plan.map((block) => (
          <section key={block.id} className={`deploy-plan-block deploy-plan-block-${block.kind}`}>
            <header className="deploy-plan-block-head">
              <div>
                <div className="deploy-plan-block-title">{block.title}</div>
                {block.subtitle && <div className="deploy-plan-block-sub">{block.subtitle}</div>}
              </div>
              {block.kind === "app" && <span className="pill">app</span>}
            </header>
            <ol className="deploy-substeps">
              {block.steps.map((s) => {
                globalIndex += 1;
                return (
                  <li key={s.id} className={`deploy-substep${kindClass(s.kind)}`}>
                    <span className="deploy-substep-n" aria-hidden>
                      {globalIndex}
                    </span>
                    <div className="deploy-substep-body">
                      <div className="deploy-substep-title">{s.title}</div>
                      {s.detail && (
                        <div className="deploy-substep-detail">
                          {s.kind === "install" || s.kind === "build" || s.kind === "command" ? (
                            <code>{s.detail}</code>
                          ) : (
                            s.detail
                          )}
                        </div>
                      )}
                    </div>
                  </li>
                );
              })}
            </ol>
          </section>
        ))}
      </div>
    </div>
  );
}

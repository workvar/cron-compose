"use client";

import Link from "next/link";
import { blocksFromProject } from "@/lib/project-blocks";
import type { DeployProject } from "@/lib/types";

/** Collapsible deployment settings (build / pre-start / run) — Vercel-style accordion. */
export function DeploymentSettings({ project }: { project: DeployProject }) {
  const blocks = blocksFromProject(project);

  return (
    <details className="panel deploy-settings" style={{ marginTop: 18 }}>
      <summary className="deploy-settings-summary">
        <span className="card-title" style={{ margin: 0 }}>Deployment settings</span>
        <span className="subtle small">
          Build, cleanup, pre-start, and run for each process ·{" "}
          <Link href={`/deploys/${project.id}/edit`} onClick={(e) => e.stopPropagation()}>
            Edit
          </Link>
        </span>
      </summary>
      <div className="deploy-settings-body">
        {blocks.map((b) => (
          <div key={b.id} className="deploy-settings-app">
            <div className="cluster" style={{ gap: 8, marginBottom: 10 }}>
              <strong>{b.name || "Process"}</strong>
              <span className="pill">{b.root || "."}</span>
              {b.processManager !== "none" && <span className="pill">{b.processManager}</span>}
            </div>
            <dl className="deploy-settings-dl">
              <div>
                <dt>Build</dt>
                <dd><pre>{b.install.trim() || "(none)"}</pre></dd>
              </div>
              {b.cleanup.trim() ? (
                <div>
                  <dt>Cleanup</dt>
                  <dd><pre>{b.cleanup}</pre></dd>
                </div>
              ) : null}
              <div>
                <dt>Pre-start</dt>
                <dd>
                  <pre>{b.preStart.trim() || "(none — migrations / one-shot after each deploy)"}</pre>
                </dd>
              </div>
              <div>
                <dt>Run</dt>
                <dd><pre>{b.run.trim() || "(none)"}</pre></dd>
              </div>
            </dl>
          </div>
        ))}
      </div>
    </details>
  );
}

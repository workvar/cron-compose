"use client";

import { useState } from "react";
import type { DeployApp, DeployProject } from "@/lib/types";
import { AppEnvEditor } from "./AppEnvEditor";
import { RedeployButton } from "./RedeployButton";

export function ProjectEnvPanel({ project }: { project: DeployProject }) {
  const initialApps: DeployApp[] =
    project.apps?.length > 0
      ? project.apps
      : [
          {
            name: project.name.split("/").pop() || project.name,
            root: project.root_directory || ".",
            language: project.language,
            install: project.install_script,
            port: project.port,
            process_manager: project.process_manager,
            env: Object.entries(project.env || {}).map(([key, value]) => ({
              key,
              value,
              sensitive: false,
              has_value: true,
            })),
          },
        ];

  const [apps, setApps] = useState(initialApps);
  const [needsRedeploy, setNeedsRedeploy] = useState(false);

  async function autosave(next: DeployApp[]) {
    const res = await fetch(`/api/deploys/${project.id}`, {
      method: "PATCH",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ apps: next }),
    });
    if (!res.ok) throw new Error(await res.text());
    const data = (await res.json()) as { project: DeployProject };
    if (data.project.apps?.length) setApps(data.project.apps);
    setNeedsRedeploy(true);
  }

  return (
    <div style={{ marginTop: 18 }}>
      <AppEnvEditor apps={apps} onChange={setApps} onAutosave={autosave} />
      {needsRedeploy && (
        <div className="env-redeploy-bar">
          <div>
            <div style={{ fontWeight: 700, color: "var(--text)" }}>Redeploy to apply env changes</div>
            <p className="subtle" style={{ margin: "4px 0 0", fontSize: 13 }}>
              Variables are saved on the control plane. Redeploy so the agent picks them up.
            </p>
          </div>
          <RedeployButton projectId={project.id} />
        </div>
      )}
    </div>
  );
}

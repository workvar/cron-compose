"use client";

import Link from "next/link";
import { useEffect, useMemo } from "react";
import CopyButton from "@/components/CopyButton";
import { IconDownload, IconX } from "@/components/icons";
import { draftToYaml, downloadYaml, projectToExportInput } from "@/lib/deploy-spec";
import type { DeployProject } from "@/lib/types";

type Props = {
  project: DeployProject;
  serverName?: string;
  onClose: () => void;
};

/** Vercel-style Share sheet: download YAML or copy the project onto another server. */
export function ShareConfigModal({ project, serverName, onClose }: Props) {
  const yaml = useMemo(
    () => draftToYaml(projectToExportInput(project, { serverName })),
    [project, serverName],
  );
  const filename = useMemo(() => {
    const safe = (project.name || "deploy").replace(/[^a-zA-Z0-9._-]+/g, "-").replace(/^-|-$/g, "") || "deploy";
    return `croncompose-${safe}.yml`;
  }, [project.name]);

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  return (
    <div className="modal-backdrop" role="presentation" onClick={onClose}>
      <div
        className="modal-panel share-config-modal"
        role="dialog"
        aria-labelledby="share-config-title"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="row" style={{ alignItems: "flex-start" }}>
          <div>
            <h2 id="share-config-title" style={{ margin: 0, fontSize: 18 }}>Share configuration</h2>
            <p className="subtle" style={{ margin: "6px 0 0" }}>
              Export everything set up on this project — processes, scripts, pre-start, env — as a{" "}
              <code>.yml</code> you can import on another server.
            </p>
          </div>
          <button type="button" className="button ghost sm" onClick={onClose} aria-label="Close">
            <IconX />
          </button>
        </div>

        <div className="share-config-actions">
          <button
            type="button"
            className="button"
            onClick={() => downloadYaml(filename, yaml)}
          >
            <IconDownload /> Download {filename}
          </button>
          <Link
            href={`/deploys/new?from=${encodeURIComponent(project.id)}`}
            className="button secondary"
            onClick={onClose}
          >
            Copy to another server
          </Link>
        </div>

        <p className="field-hint" style={{ marginTop: 14 }}>
          On the next server: Deploy → Import shared croncompose.yml → edit → deploy. Sensitive env
          values are left out of the file; Copy to server on this CronCompose keeps them.
        </p>

        <div className="row" style={{ marginTop: 16, marginBottom: 8 }}>
          <div className="card-title">croncompose.yml</div>
          <CopyButton value={yaml} className="light" />
        </div>
        <pre className="spec-code share-config-preview">{yaml}</pre>
      </div>
    </div>
  );
}

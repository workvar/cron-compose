"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { IconMore, IconShare } from "@/components/icons";
import { draftToYaml, downloadYaml, projectToExportInput } from "@/lib/deploy-spec";
import type { DeployProject } from "@/lib/types";
import { ShareConfigModal } from "./ShareConfigModal";

/** Row/card overflow menu: Share, export YAML, copy to server. */
export function ProjectCardMenu({
  project,
  serverName,
}: {
  project: DeployProject;
  serverName?: string;
}) {
  const [open, setOpen] = useState(false);
  const [shareOpen, setShareOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    function onDoc(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    }
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  }, [open]);

  function exportYaml() {
    const yaml = draftToYaml(projectToExportInput(project, { serverName }));
    const safe = (project.name || "deploy").replace(/[^a-zA-Z0-9._-]+/g, "-").replace(/^-|-$/g, "") || "deploy";
    downloadYaml(`croncompose-${safe}.yml`, yaml);
    setOpen(false);
  }

  return (
    <>
      <div className="action-menu" ref={ref} onClick={(e) => e.preventDefault()}>
        <button
          type="button"
          className="button ghost sm icon-only"
          aria-label={`Actions for ${project.name}`}
          aria-expanded={open}
          onClick={(e) => {
            e.preventDefault();
            e.stopPropagation();
            setOpen((v) => !v);
          }}
        >
          <IconMore />
        </button>
        {open && (
          <div className="action-menu-pop" role="menu">
            <button
              type="button"
              role="menuitem"
              onClick={(e) => {
                e.preventDefault();
                e.stopPropagation();
                setOpen(false);
                setShareOpen(true);
              }}
            >
              <IconShare /> Share config
            </button>
            <button
              type="button"
              role="menuitem"
              onClick={(e) => {
                e.preventDefault();
                e.stopPropagation();
                exportYaml();
              }}
            >
              Export croncompose.yml
            </button>
            <Link
              href={`/deploys/new?from=${encodeURIComponent(project.id)}`}
              role="menuitem"
              onClick={(e) => e.stopPropagation()}
            >
              Copy to another server
            </Link>
            <Link
              href={`/deploys/${project.id}/edit`}
              role="menuitem"
              onClick={(e) => e.stopPropagation()}
            >
              Edit
            </Link>
          </div>
        )}
      </div>
      {shareOpen && (
        <ShareConfigModal
          project={project}
          serverName={serverName}
          onClose={() => setShareOpen(false)}
        />
      )}
    </>
  );
}

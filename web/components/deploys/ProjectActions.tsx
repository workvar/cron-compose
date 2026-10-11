"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { IconMore, IconShare } from "@/components/icons";
import { draftToYaml, downloadYaml, projectToExportInput } from "@/lib/deploy-spec";
import type { DeployProject } from "@/lib/types";
import { ShareConfigModal } from "./ShareConfigModal";

export function ProjectActions({
  project,
  serverName,
}: {
  project: DeployProject;
  serverName?: string;
}) {
  const router = useRouter();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [shareOpen, setShareOpen] = useState(false);
  const [menuOpen, setMenuOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!menuOpen) return;
    function onDoc(e: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setMenuOpen(false);
      }
    }
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") setMenuOpen(false);
    }
    document.addEventListener("mousedown", onDoc);
    window.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDoc);
      window.removeEventListener("keydown", onKey);
    };
  }, [menuOpen]);

  function exportYaml() {
    const yaml = draftToYaml(projectToExportInput(project, { serverName }));
    const safe = (project.name || "deploy").replace(/[^a-zA-Z0-9._-]+/g, "-").replace(/^-|-$/g, "") || "deploy";
    downloadYaml(`croncompose-${safe}.yml`, yaml);
    setMenuOpen(false);
  }

  async function remove() {
    setMenuOpen(false);
    if (!confirm(`Delete deploy ${project.name}?`)) return;
    setBusy(true);
    setError(null);
    try {
      const res = await fetch(`/api/deploys/${project.id}`, { method: "DELETE" });
      if (!res.ok && res.status !== 204) throw new Error(await res.text());
      router.push("/deploys");
      router.refresh();
    } catch (e) {
      setError((e as Error).message);
      setBusy(false);
    }
  }

  return (
    <>
      <Link href={`/deploys/${project.id}/edit`} className="button secondary">
        Edit
      </Link>
      <button
        type="button"
        className="button secondary"
        onClick={() => setShareOpen(true)}
        title="Share configuration as croncompose.yml"
      >
        <IconShare /> Share
      </button>
      <div className="action-menu" ref={menuRef}>
        <button
          type="button"
          className="button secondary icon-only"
          aria-label="More actions"
          aria-expanded={menuOpen}
          aria-haspopup="menu"
          onClick={() => setMenuOpen((v) => !v)}
          disabled={busy}
        >
          <IconMore />
        </button>
        {menuOpen && (
          <div className="action-menu-pop" role="menu">
            <button type="button" role="menuitem" onClick={exportYaml}>
              Export croncompose.yml
            </button>
            <Link
              href={`/deploys/new?from=${encodeURIComponent(project.id)}`}
              role="menuitem"
              onClick={() => setMenuOpen(false)}
            >
              Copy to another server
            </Link>
            <Link
              href={`/deploys/${project.id}/edit`}
              role="menuitem"
              onClick={() => setMenuOpen(false)}
            >
              Edit configuration
            </Link>
            <button type="button" role="menuitem" className="danger" onClick={() => void remove()}>
              Delete project
            </button>
          </div>
        )}
      </div>
      {error && <span className="form-error" style={{ marginLeft: 8 }}>{error}</span>}
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

"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import type { DeployProject } from "@/lib/types";

export function ProjectActions({ project }: { project: DeployProject }) {
  const router = useRouter();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function remove() {
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
      <button type="button" className="button secondary" onClick={remove} disabled={busy}>
        Delete
      </button>
      {error && <span className="form-error" style={{ marginLeft: 8 }}>{error}</span>}
    </>
  );
}

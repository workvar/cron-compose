"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { IconRestart } from "@/components/icons";
import { shortCommit } from "@/lib/deploy-duration";

type UpdateCheck = {
  branch: string;
  deployed_sha?: string;
  remote_sha?: string;
  update_available: boolean;
  ahead_by?: number;
  behind_by?: number;
  deployed_at?: string;
  busy?: boolean;
  message?: string;
};

type Props = {
  projectId: string;
  branch: string;
};

export function BranchUpdateBanner({ projectId, branch }: Props) {
  const router = useRouter();
  const [status, setStatus] = useState<UpdateCheck | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const refresh = useCallback(async () => {
    try {
      const q = new URLSearchParams({ branch });
      const res = await fetch(`/api/deploys/${projectId}/update-check?${q}`);
      if (!res.ok) {
        const text = await res.text();
        throw new Error(text || `HTTP ${res.status}`);
      }
      setStatus((await res.json()) as UpdateCheck);
      setError(null);
    } catch (e) {
      setError((e as Error).message);
      setStatus(null);
    }
  }, [projectId, branch]);

  useEffect(() => {
    void refresh();
    const id = window.setInterval(() => void refresh(), 60_000);
    return () => window.clearInterval(id);
  }, [refresh]);

  async function updateDeploy() {
    setBusy(true);
    setError(null);
    try {
      const res = await fetch(`/api/deploys/${projectId}/runs`, {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ trigger: "manual", branch }),
      });
      if (!res.ok) throw new Error(await res.text());
      const run = (await res.json()) as { id: string };
      router.push(`/deploys/runs/${run.id}`);
    } catch (e) {
      setError((e as Error).message);
      setBusy(false);
    }
  }

  if (error && !status) {
    return (
      <div className="branch-update-banner is-muted" role="status">
        <p className="subtle" style={{ margin: 0 }}>
          Could not check {branch} for updates: {error}
        </p>
        <button type="button" className="button secondary sm" onClick={() => void refresh()}>
          Retry
        </button>
      </div>
    );
  }

  if (!status?.update_available) return null;

  const remote = shortCommit(status.remote_sha);
  const live = shortCommit(status.deployed_sha);
  const ahead =
    status.ahead_by && status.ahead_by > 0
      ? `${status.ahead_by} new commit${status.ahead_by === 1 ? "" : "s"}`
      : "New commits";

  return (
    <div className="branch-update-banner" role="status">
      <div className="branch-update-banner-main">
        <strong>Update available on {status.branch}</strong>
        <p>
          {status.message || `${ahead} on the connected branch.`}
          {live && remote ? ` Live ${live} → ${remote}.` : remote ? ` Tip ${remote}.` : ""}
        </p>
      </div>
      <div className="branch-update-banner-actions">
        <button
          type="button"
          className="button secondary sm"
          disabled={busy}
          onClick={() => void refresh()}
        >
          Check again
        </button>
        <button
          type="button"
          className="button sm"
          disabled={busy || status.busy}
          onClick={() => void updateDeploy()}
        >
          <IconRestart /> {busy ? "Starting…" : "Update deploy"}
        </button>
      </div>
      {error && <p className="form-error" style={{ margin: 0, flexBasis: "100%" }}>{error}</p>}
    </div>
  );
}

"use client";

import { useCallback, useEffect, useState } from "react";
import type { ConnectorResource } from "@/lib/types";
import { StepList } from "./StepList";
import { useConnectorCommand } from "./useConnectorCommand";

/**
 * Read, check, and apply one config file.
 *
 * The checksum captured at read time travels back with the apply, so the agent can
 * refuse a write when the file changed underneath the editor. That is why the editor
 * always loads before it lets you save, and why switching files clears the buffer
 * rather than keeping your text against a new baseline.
 */
export function ConfigEditor({
  connectorId,
  files,
  canEdit,
}: {
  connectorId: string;
  files: ConnectorResource[];
  canEdit: boolean;
}) {
  const [path, setPath] = useState(files[0]?.ref ?? "");
  const [content, setContent] = useState("");
  const [baseChecksum, setBaseChecksum] = useState("");
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [dirty, setDirty] = useState(false);
  const { busy, error, result, send } = useConnectorCommand();

  const load = useCallback(async (p: string) => {
    if (!p) return;
    setLoading(true);
    setLoadError(null);
    setDirty(false);
    try {
      const res = await fetch(`/api/connectors/${connectorId}/config?path=${encodeURIComponent(p)}`);
      const body = await res.json().catch(() => null);
      if (!res.ok) throw new Error(body?.error?.message ?? `HTTP ${res.status}`);
      setContent(body.content ?? "");
      setBaseChecksum(body.checksum ?? "");
    } catch (e) {
      setContent("");
      setBaseChecksum("");
      setLoadError((e as Error).message);
    } finally {
      setLoading(false);
    }
  }, [connectorId]);

  useEffect(() => {
    void load(path);
  }, [path, load]);

  async function apply(dryRun: boolean) {
    const body = await send(`/api/connectors/${connectorId}/config`, {
      method: "POST",
      body: JSON.stringify({
        path,
        content,
        base_checksum: baseChecksum,
        dry_run: dryRun,
      }),
    });
    // A real apply moves the baseline forward, so a second save in the same sitting
    // is not rejected as stale.
    if (body?.status === "succeeded" && !dryRun) {
      setBaseChecksum(body.checksum ?? "");
      setDirty(false);
    }
  }

  if (files.length === 0) return null;

  return (
    <div className="panel stack" style={{ gap: 12 }}>
      <div className="row">
        <select
          className="input"
          value={path}
          onChange={(e) => setPath(e.target.value)}
          disabled={loading || busy}
          style={{ maxWidth: 420 }}
        >
          {files.map((f) => (
            <option key={f.id} value={f.ref}>{f.ref}</option>
          ))}
        </select>
        <button type="button" className="button secondary sm" onClick={() => void load(path)} disabled={loading || busy}>
          Reload
        </button>
      </div>

      {loadError && <p className="form-error" style={{ margin: 0 }}>{loadError}</p>}

      <textarea
        className="input mono"
        rows={22}
        spellCheck={false}
        value={loading ? "loading..." : content}
        readOnly={!canEdit || loading}
        onChange={(e) => { setContent(e.target.value); setDirty(true); }}
        style={{ fontSize: 12, lineHeight: 1.5 }}
      />

      <div className="row">
        <div className="cluster" style={{ gap: 8 }}>
          <button type="button" className="button secondary" onClick={() => void apply(true)} disabled={busy || loading || !path}>
            Check only
          </button>
          <button type="button" className="button" onClick={() => void apply(false)} disabled={!canEdit || busy || loading || !dirty}>
            {busy ? "Applying..." : "Apply and reload"}
          </button>
        </div>
        <span className="faint" style={{ fontSize: 12 }}>
          {dirty ? "unsaved changes" : baseChecksum ? `checksum ${baseChecksum.slice(0, 12)}` : ""}
        </span>
      </div>

      {!canEdit && (
        <p className="faint" style={{ fontSize: 12, margin: 0 }}>
          Read-only: the agent reported it cannot write this connector&apos;s config. Grant it
          write access to the files, or a passwordless sudo entry, to enable editing.
        </p>
      )}

      {error && <p className="form-error" style={{ margin: 0 }}>{error}</p>}
      {result && (
        <div className={result.status === "succeeded" ? "subtle" : "form-error"}>
          <strong>{result.status}</strong>{result.message ? `: ${result.message}` : ""}
          <StepList steps={result.steps ?? []} />
        </div>
      )}
    </div>
  );
}

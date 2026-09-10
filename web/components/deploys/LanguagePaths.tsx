"use client";

import { useState } from "react";
import type { DeploySettings } from "@/lib/types";

export function LanguagePaths({ initial }: { initial: DeploySettings }) {
  const [paths, setPaths] = useState(initial.language_paths);
  const [busy, setBusy] = useState(false);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function save(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    setSaved(false);
    try {
      const res = await fetch("/api/deploy-settings", {
        method: "PUT",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ language_paths: paths }),
      });
      if (!res.ok) {
        const txt = await res.text();
        throw new Error(txt || `HTTP ${res.status}`);
      }
      const next = (await res.json()) as DeploySettings;
      setPaths(next.language_paths);
      setSaved(true);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={save} className="panel" style={{ maxWidth: 640 }}>
      <p className="subtle" style={{ marginTop: 0 }}>
        Repos clone into <code>{"{path}/{repo}"}</code> on the selected agent. Override per deploy if needed.
      </p>
      {Object.entries(paths).map(([lang, path]) => (
        <div className="field" key={lang}>
          <label htmlFor={`path-${lang}`}>{lang}</label>
          <input
            id={`path-${lang}`}
            value={path}
            onChange={(e) => setPaths((p) => ({ ...p, [lang]: e.target.value }))}
          />
        </div>
      ))}
      {error && <p className="form-error">{error}</p>}
      <button type="submit" className="button" disabled={busy}>
        {busy ? "Saving…" : saved ? "Saved" : "Save paths"}
      </button>
    </form>
  );
}

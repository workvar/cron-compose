"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { stepUpWithPasskey } from "@/lib/webauthn";
import { apiErrorMessage } from "@/lib/api-error";
import type { ConnectorResource, ProcessInspectDetail } from "@/lib/types";
import { MaskedEnvField } from "./MaskedEnvField";

type StepUp = {
  challenge_id: string;
  credential: Record<string, unknown>;
};

type Props = {
  serverId: string;
  kind: string;
  resource: ConnectorResource;
  onClose: () => void;
};

export function ProcessImportPanel({ serverId, kind, resource, onClose }: Props) {
  const router = useRouter();
  const attrs = resource.attributes || {};
  const connectorId = attrs.connector_id || resource.connector_id;
  const [name, setName] = useState(resource.name);
  const [command, setCommand] = useState(attrs.command || attrs.exec || "");
  const [cwd, setCwd] = useState(attrs.cwd || "");
  const [provider, setProvider] = useState("github");
  const [repo, setRepo] = useState("");
  const [branch, setBranch] = useState("main");
  const [cloneURL, setCloneURL] = useState("");
  const [env, setEnv] = useState<Record<string, string>>({});
  const [detail, setDetail] = useState<ProcessInspectDetail | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const envKeys = (attrs.env_keys || "").split(",").map((s) => s.trim()).filter(Boolean);

  function applyDetail(d: ProcessInspectDetail) {
    setDetail(d);
    if (d.name) setName(d.name);
    if (d.command) setCommand((d.command + (d.args ? ` ${d.args}` : "")).trim());
    if (d.cwd) setCwd(d.cwd);
    if (d.env) setEnv(d.env);
    if (d.git?.provider) setProvider(d.git.provider);
    if (d.git?.repo_full_name) setRepo(d.git.repo_full_name);
    if (d.git?.branch) setBranch(d.git.branch);
    if (d.git?.clone_url) setCloneURL(d.git.clone_url);
  }

  async function fetchDetail(step: StepUp): Promise<ProcessInspectDetail> {
    if (!connectorId) throw new Error("Missing connector id");
    const res = await fetch(
      `/api/connectors/${encodeURIComponent(connectorId)}/objects/${encodeURIComponent(resource.ref)}/inspect`,
      {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify(step),
      },
    );
    if (!res.ok) throw new Error(await apiErrorMessage(res, "Could not inspect process"));
    const data = (await res.json()) as { detail: ProcessInspectDetail };
    applyDetail(data.detail);
    return data.detail;
  }

  async function importProcess() {
    setBusy(true);
    setError(null);
    try {
      let d = detail;
      if (!d) {
        d = await fetchDetail(await stepUpWithPasskey());
      }
      const finalName = name.trim() || d.name || resource.name;
      const finalCommand = command.trim() || d.command || "";
      const finalCwd = cwd.trim() || d.cwd || "";
      const finalRepo = repo.trim() || d.git?.repo_full_name || "";
      const finalProvider = provider || d.git?.provider || "github";
      const finalBranch = branch.trim() || d.git?.branch || "main";
      const finalClone = cloneURL.trim() || d.git?.clone_url || "";
      const finalEnv = Object.keys(env).length ? env : (d.env || {});

      if (!finalRepo) {
        throw new Error("Repository (owner/name) is required");
      }

      const res = await fetch(`/api/servers/${encodeURIComponent(serverId)}/deploys/import-process`, {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          kind,
          ref: resource.ref,
          name: finalName,
          command: finalCommand,
          cwd: finalCwd,
          provider: finalProvider,
          repo_full_name: finalRepo,
          clone_url: finalClone || undefined,
          default_branch: finalBranch,
          process_manager: kind === "docker" ? "docker" : kind,
          env: finalEnv,
        }),
      });
      if (!res.ok) throw new Error(await apiErrorMessage(res, "Import failed"));
      const data = (await res.json()) as { project: { id: string } };
      router.push(`/deploys/${data.project.id}`);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="panel process-import">
      <div className="card-head">
        <div className="card-title">Import {kind} process</div>
        <button type="button" className="button secondary sm" onClick={onClose}>Close</button>
      </div>
      <p className="subtle" style={{ marginTop: 0 }}>
        Creates a Deploy Project from the running process without restarting it.
        Link a git repo so future deploys know where to pull from.
      </p>

      <div className="form-grid" style={{ marginTop: 14 }}>
        <label>
          Name
          <input value={name} onChange={(e) => setName(e.target.value)} />
        </label>
        <label>
          Command
          <input value={command} onChange={(e) => setCommand(e.target.value)} />
        </label>
        <label>
          Working directory
          <input value={cwd} onChange={(e) => setCwd(e.target.value)} placeholder="/path/on/server" />
        </label>
        <label>
          Provider
          <select value={provider} onChange={(e) => setProvider(e.target.value)}>
            <option value="github">GitHub</option>
            <option value="gitlab">GitLab</option>
          </select>
        </label>
        <label>
          Repository (owner/name)
          <input value={repo} onChange={(e) => setRepo(e.target.value)} placeholder="acme/api" />
        </label>
        <label>
          Branch
          <input value={branch} onChange={(e) => setBranch(e.target.value)} />
        </label>
      </div>

      <div style={{ marginTop: 16 }}>
        <div className="card-title" style={{ marginBottom: 8 }}>Environment</div>
        <MaskedEnvField
          keys={envKeys.length ? envKeys : Object.keys(env)}
          env={env}
          onStepUpReveal={async (step) => {
            const d = await fetchDetail(step);
            return d.env || {};
          }}
        />
      </div>

      {(attrs.mode || attrs.unit_path || attrs.image || attrs.compose_project) && (
        <div className="subtle" style={{ marginTop: 14, fontSize: 12 }}>
          {attrs.mode && <div>Mode: {attrs.mode}</div>}
          {attrs.unit_path && <div>Unit: {attrs.unit_path}</div>}
          {attrs.image && <div>Image: {attrs.image}</div>}
          {attrs.compose_project && (
            <div>Compose: {attrs.compose_project}/{attrs.compose_service || "?"}</div>
          )}
        </div>
      )}

      {error && <p className="form-error">{error}</p>}

      <div className="cluster" style={{ marginTop: 18 }}>
        <button type="button" className="button" disabled={busy} onClick={importProcess}>
          {busy ? "Importing…" : "Import as project"}
        </button>
        <button type="button" className="button secondary" onClick={onClose}>Cancel</button>
      </div>
    </div>
  );
}

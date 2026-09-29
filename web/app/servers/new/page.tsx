"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import type { CreateServerResponse } from "@/lib/types";
import { IconChevronLeft, IconCheck } from "@/components/icons";
import CopyButton from "@/components/CopyButton";
import TradeoffList from "@/components/TradeoffList";

export default function NewServerPage() {
  const router = useRouter();
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState<CreateServerResponse | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const res = await fetch("/api/servers", {
        method: "POST",
        credentials: "include",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ name, description }),
      });
      if (!res.ok) {
        const body = (await res.json().catch(() => null)) as { error?: { message?: string } } | null;
        throw new Error(body?.error?.message ?? `HTTP ${res.status}`);
      }
      setResult((await res.json()) as CreateServerResponse);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  if (result) {
    return (
      <>
        <Link href="/servers" className="back-link"><IconChevronLeft /> Servers</Link>
        <div className="page-head">
          <div>
            <h1>Server created</h1>
            <p className="subtle">Run one install command on <strong>{result.server.name}</strong>. The token is shown once.</p>
          </div>
        </div>

        <div className="panel" style={{ maxWidth: 640, marginBottom: 16 }}>
          <label>Install as a dedicated user (recommended)</label>
          <div className="code-block" style={{ marginTop: 6 }}>
            <pre className="review-script" style={{ marginTop: 0 }}>{result.install_command}</pre>
            <CopyButton value={result.install_command} label="Copy" />
          </div>
          <TradeoffList
            good={[
              "Runs as the unprivileged croncompose system user; a compromised job can't touch the rest of the box",
              "Matches least-privilege practice for anything internet-reachable",
              "Root access can still be turned on later per-server, on demand, with a passkey",
            ]}
            bad={[
              "The web terminal can only open as the croncompose user until you enable Agent root access",
              "Installing system packages or editing files outside its home needs sudo rules you add yourself",
            ]}
          />
        </div>

        <div className="panel" style={{ maxWidth: 640 }}>
          <label>Install as root</label>
          <div className="code-block" style={{ marginTop: 6 }}>
            <pre className="review-script" style={{ marginTop: 0 }}>{result.install_command_root}</pre>
            <CopyButton value={result.install_command_root} label="Copy" />
          </div>
          <TradeoffList
            good={[
              "The web terminal can open as any OS user immediately, nothing else to enable",
              "No separate Agent root access step, no passkey step-up needed for this",
              "Simplest choice on a box you already treat as fully trusted (your own homelab, a throwaway VM)",
            ]}
            bad={[
              "Every job, connector op and terminal session runs with full root from the start",
              "A bug or malicious job on this server has the whole machine, not just its own account",
              "Harder to demote later: you'd need to reinstall as a dedicated user rather than flip a switch",
            ]}
          />
        </div>

        <button className="button" style={{ marginTop: 16 }} onClick={() => router.push("/servers")}>
          <IconCheck /> Done
        </button>
      </>
    );
  }

  return (
    <>
      <Link href="/servers" className="back-link"><IconChevronLeft /> Servers</Link>
      <div className="page-head">
        <div>
          <h1>Add server</h1>
          <p className="subtle">Give it a name. You&apos;ll get an install command next.</p>
        </div>
      </div>

      <form onSubmit={submit} className="panel" style={{ maxWidth: 520 }}>
        <div className="field">
          <label htmlFor="name">Name</label>
          <input id="name" value={name} onChange={(e) => setName(e.target.value)} placeholder="kitchen-pi" required />
        </div>
        <div className="field">
          <label htmlFor="description">Description (optional)</label>
          <input id="description" value={description} onChange={(e) => setDescription(e.target.value)} placeholder="Raspberry Pi behind the kitchen TV" />
        </div>
        {error && <div className="form-error" style={{ marginBottom: 14 }}>{error}</div>}
        <button type="submit" className="button" disabled={busy || !name}>
          {busy ? "Creating…" : "Create server"}
        </button>
      </form>
    </>
  );
}

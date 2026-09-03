"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import type { CreateServerResponse } from "@/lib/types";
import { IconChevronLeft, IconCheck } from "@/components/icons";

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
            <p className="subtle">Run the install command on <strong>{result.server.name}</strong>. The token is shown once.</p>
          </div>
        </div>
        <div className="panel" style={{ maxWidth: 640 }}>
          <label>Install command</label>
          <pre className="review-script" style={{ marginTop: 6 }}>{result.install_command}</pre>
          <button className="button" style={{ marginTop: 16 }} onClick={() => router.push("/servers")}>
            <IconCheck /> Done
          </button>
        </div>
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

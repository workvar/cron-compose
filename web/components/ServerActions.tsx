"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import type { Server } from "@/lib/types";
import { IconTrash } from "@/components/icons";

// Delete is the only destructive action here, so this stays a single button rather
// than growing into a full edit form like ProjectActions. Gated to admin/owner by
// the caller, matching the control-plane's role requirement on DELETE /servers/:id.
export function ServerActions({ server }: { server: Server }) {
  const router = useRouter();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function remove() {
    if (!confirm(`Delete server "${server.name}"? Its jobs and run history go with it.`)) return;
    setBusy(true);
    setError(null);
    try {
      const res = await fetch(`/api/servers/${server.id}`, { method: "DELETE" });
      if (!res.ok && res.status !== 204) throw new Error(await res.text());
      router.push("/servers");
      router.refresh();
    } catch (e) {
      setError((e as Error).message);
      setBusy(false);
    }
  }

  return (
    <div className="stack" style={{ gap: 6, alignItems: "flex-end" }}>
      <button
        type="button"
        className="button icon-danger"
        onClick={remove}
        disabled={busy}
        aria-label={busy ? "Deleting server" : `Delete server ${server.name}`}
        title="Delete server"
      >
        <IconTrash />
      </button>
      {error && <p className="form-error" style={{ margin: 0 }}>{error}</p>}
    </div>
  );
}

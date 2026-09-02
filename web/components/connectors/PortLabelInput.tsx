"use client";

import { useEffect, useState } from "react";
import type { ConnectorPort } from "@/lib/types";

export function PortLabelInput({
  serverId,
  row,
  disabled,
  onSaved,
}: {
  serverId: string;
  row: ConnectorPort;
  disabled?: boolean;
  onSaved?: (label: string) => void;
}) {
  const [value, setValue] = useState(row.label ?? "");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setValue(row.label ?? "");
  }, [row.label]);

  async function save(next: string) {
    const trimmed = next.trim();
    if (trimmed === (row.label ?? "").trim()) return;
    setBusy(true);
    setError(null);
    try {
      const res = await fetch("/api/port-labels", {
        method: "PUT",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          server_id: serverId,
          proto: row.proto,
          address: row.address,
          port: row.port,
          label: trimmed,
        }),
      });
      const body = await res.json().catch(() => null);
      if (!res.ok) throw new Error(body?.error?.message ?? `HTTP ${res.status}`);
      onSaved?.(trimmed);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div>
      <input
        className="port-label-input"
        value={value}
        disabled={disabled || busy}
        placeholder="Label this port"
        aria-label={`Label for port ${row.port}`}
        onChange={(e) => setValue(e.target.value)}
        onBlur={() => void save(value)}
        onKeyDown={(e) => {
          if (e.key === "Enter") {
            e.preventDefault();
            (e.target as HTMLInputElement).blur();
          }
        }}
      />
      {error && <div className="form-error" style={{ marginTop: 6, padding: "6px 8px" }}>{error}</div>}
    </div>
  );
}

"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { IconPlay } from "@/components/icons";

export function RedeployButton({ projectId }: { projectId: string }) {
  const router = useRouter();
  const [busy, setBusy] = useState(false);

  async function run() {
    setBusy(true);
    try {
      const res = await fetch(`/api/deploys/${projectId}/runs`, {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ trigger: "manual" }),
      });
      if (!res.ok) throw new Error(await res.text());
      const run = (await res.json()) as { id: string };
      router.push(`/deploys/runs/${run.id}`);
    } finally {
      setBusy(false);
    }
  }

  return (
    <button type="button" className="button" disabled={busy} onClick={run}>
      <IconPlay /> {busy ? "Starting…" : "Deploy now"}
    </button>
  );
}

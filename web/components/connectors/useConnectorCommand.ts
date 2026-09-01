"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import type { ConnectorCommandResponse } from "@/lib/types";

/**
 * Shared state machine for every connector mutation: one request in flight at a time,
 * the last result kept so the step list stays on screen, and a router refresh on
 * success so the server-rendered inventory catches up with the change.
 *
 * A connector command can come back "succeeded" at the HTTP level and still have
 * failed on the box (status: invalid, unauthorized, ...), so the caller has to look at
 * result.status, not just the absence of an error. That distinction is the reason this
 * returns both.
 */
export function useConnectorCommand() {
  const router = useRouter();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<ConnectorCommandResponse | null>(null);

  async function send(path: string, init: RequestInit): Promise<ConnectorCommandResponse | null> {
    setBusy(true);
    setError(null);
    setResult(null);
    try {
      const res = await fetch(path, {
        ...init,
        headers: { "content-type": "application/json", ...(init.headers ?? {}) },
      });
      const body = (await res.json().catch(() => null)) as
        | (ConnectorCommandResponse & { error?: { message?: string } })
        | null;

      if (!res.ok) {
        throw new Error(body?.error?.message ?? httpMessage(res.status));
      }
      setResult(body);
      if (body?.status === "succeeded") router.refresh();
      return body;
    } catch (e) {
      setError((e as Error).message);
      return null;
    } finally {
      setBusy(false);
    }
  }

  return { busy, error, result, send, clear: () => setResult(null) };
}

function httpMessage(status: number): string {
  if (status === 503) return "The agent for this server is offline";
  if (status === 504) return "The agent did not answer in time; the change may still have applied";
  if (status === 403) return "You do not have permission to do that";
  if (status === 409) return "The agent reported it cannot manage this connector";
  return `HTTP ${status}`;
}

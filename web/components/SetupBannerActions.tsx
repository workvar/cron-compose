"use client";

import { useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import type { HealthStatus } from "@/lib/health";

type BootstrapState = "idle" | "working" | "done" | "error";

type SetupStatus = {
  db: "ok" | "down";
  can_bootstrap: boolean;
};

async function fetchSetupStatus(): Promise<SetupStatus | null> {
  try {
    const res = await fetch("/api/setup/status", { cache: "no-store" });
    if (!res.ok) return null;
    return res.json() as Promise<SetupStatus>;
  } catch {
    return null;
  }
}

async function runBootstrap(): Promise<{ ok: boolean; error?: string }> {
  try {
    const res = await fetch("/api/setup/bootstrap", { method: "POST" });
    const body = (await res.json().catch(() => null)) as { status?: string; error?: string } | null;
    if (res.ok && body?.status === "ok") return { ok: true };
    return { ok: false, error: body?.error ?? `setup failed (${res.status})` };
  } catch (err) {
    return { ok: false, error: (err as Error).message };
  }
}

async function waitForDatabase(): Promise<boolean> {
  for (let i = 0; i < 30; i++) {
    const status = await fetchSetupStatus();
    if (status?.db === "ok") return true;
    await new Promise((r) => setTimeout(r, 1000));
  }
  return false;
}

export function SetupBannerActions({ status }: { status: HealthStatus }) {
  const router = useRouter();
  const [state, setState] = useState<BootstrapState>("idle");
  const [message, setMessage] = useState("");
  const started = useRef(false);

  useEffect(() => {
    if (status !== "db_down" || started.current) return;
    started.current = true;
    void bootstrap();
  }, [status]);

  async function bootstrap() {
    setState("working");
    setMessage("Preparing local Postgres and applying migrations…");

    const result = await runBootstrap();
    if (!result.ok) {
      setState("error");
      setMessage(result.error ?? "Setup failed");
      return;
    }

    setMessage("Waiting for database…");
    const ready = await waitForDatabase();
    if (!ready) {
      setState("error");
      setMessage("Database did not become ready in time. Try again.");
      return;
    }

    setState("done");
    setMessage("Database is ready.");
    router.refresh();
  }

  if (status === "unreachable") {
    return (
      <div className="setup-banner-actions">
        <button type="button" className="button sm" onClick={() => router.refresh()}>
          Retry
        </button>
      </div>
    );
  }

  return (
    <div className="setup-banner-actions">
      {state === "working" && <p className="setup-banner-progress">{message}</p>}
      {state === "error" && (
        <>
          <p className="setup-banner-error">{message}</p>
          <button type="button" className="button sm" onClick={() => void bootstrap()}>
            Try again
          </button>
        </>
      )}
      {state === "idle" && (
        <button type="button" className="button sm" onClick={() => void bootstrap()}>
          Set up database
        </button>
      )}
    </div>
  );
}

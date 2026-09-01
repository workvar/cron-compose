"use client";

import { useEffect, useState, use } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import type { Job, Server } from "@/lib/types";
import { Stepper, type StepDef } from "@/components/jobwizard/Stepper";
import { StepTarget } from "@/components/jobwizard/StepTarget";
import { StepScript } from "@/components/jobwizard/StepScript";
import { StepSchedule } from "@/components/jobwizard/StepSchedule";
import { StepReview } from "@/components/jobwizard/StepReview";
import { initialDraft, parseLabels, isValidCron, type JobDraft, type Patch } from "@/components/jobwizard/types";
import { IconChevronLeft, IconChevronRight, IconCheck } from "@/components/icons";

const STEPS: StepDef[] = [
  { title: "Target", desc: "Where it runs" },
  { title: "Script", desc: "What it does" },
  { title: "Schedule", desc: "When it runs" },
  { title: "Review", desc: "Create the job" },
];

function canAdvance(step: number, d: JobDraft): boolean {
  if (step === 0) return d.targetKind === "server" || Object.keys(parseLabels(d.targetLabels)).length > 0;
  if (step === 1) return d.name.trim() !== "" && d.scriptBody.trim() !== "" && d.interpreter.trim() !== "";
  if (step === 2) return isValidCron(d.scheduleCron) && d.timezone.trim() !== "";
  return true;
}

export default function NewJobPage({ params }: { params: Promise<{ id: string }> }) {
  const { id: serverID } = use(params);
  const router = useRouter();

  const [draft, setDraft] = useState<JobDraft>(initialDraft);
  const [step, setStep] = useState(0);
  const [serverName, setServerName] = useState<string | undefined>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const set = (p: Patch) => setDraft((d) => ({ ...d, ...p }));

  useEffect(() => {
    fetch(`/api/servers/${serverID}`)
      .then((r) => (r.ok ? (r.json() as Promise<Server>) : Promise.reject()))
      .then((s) => setServerName(s.name))
      .catch(() => setServerName(undefined));
  }, [serverID]);

  const canSubmit = [0, 1, 2].every((s) => canAdvance(s, draft));

  async function submit() {
    setBusy(true);
    setError(null);
    try {
      const body: Record<string, unknown> = {
        target_kind: draft.targetKind,
        name: draft.name.trim(),
        schedule_cron: draft.scheduleCron.trim(),
        timezone: draft.timezone.trim(),
        interpreter: draft.interpreter.trim(),
        script_body: draft.scriptBody,
        timeout_seconds: draft.timeoutSeconds,
        cpu_quota_percent: draft.cpuPct,
        memory_max_mb: draft.memMB,
        tasks_max: draft.tasksMax,
        io_weight: draft.ioWeight,
        concurrency_policy: draft.concurrencyPolicy,
        catchup_policy: draft.catchupPolicy,
        max_retries: draft.maxRetries,
      };
      if (draft.description.trim()) body.description = draft.description.trim();
      if (draft.workingDir.trim()) body.working_dir = draft.workingDir.trim();
      if (draft.runAsUser.trim()) body.run_as_user = draft.runAsUser.trim();
      if (draft.targetKind === "server") body.server_id = serverID;
      else body.target_labels = parseLabels(draft.targetLabels);
      if (draft.secretRefs.length > 0) body.secret_refs = draft.secretRefs;

      const res = await fetch("/api/jobs", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify(body),
      });
      if (!res.ok) {
        const txt = await res.text().catch(() => "");
        throw new Error(`HTTP ${res.status}: ${txt}`);
      }
      const job = (await res.json()) as Job;
      router.push(`/jobs/${job.id}`);
    } catch (e) {
      setError((e as Error).message);
      setBusy(false);
    }
  }

  return (
    <>
      <Link href={`/servers/${serverID}`} className="back-link"><IconChevronLeft /> Back to server</Link>
      <div className="page-head">
        <div>
          <h1>New job</h1>
          <p className="subtle">Four quick steps: target, script, schedule, review.</p>
        </div>
      </div>

      <div className="wizard">
        <Stepper steps={STEPS} current={step} />

        <div className="wizard-body">
          {step === 0 && <StepTarget draft={draft} set={set} serverName={serverName} />}
          {step === 1 && <StepScript draft={draft} set={set} />}
          {step === 2 && <StepSchedule draft={draft} set={set} />}
          {step === 3 && <StepReview draft={draft} serverName={serverName} />}

          {error && <div className="form-error" style={{ marginTop: 18 }}>{error}</div>}

          <div className="wizard-foot">
            <button type="button" className="button secondary" onClick={() => setStep((s) => Math.max(0, s - 1))} disabled={step === 0 || busy}>
              <IconChevronLeft /> Back
            </button>
            {step < STEPS.length - 1 ? (
              <button type="button" className="button" onClick={() => setStep((s) => s + 1)} disabled={!canAdvance(step, draft)}>
                Continue <IconChevronRight />
              </button>
            ) : (
              <button type="button" className="button" onClick={submit} disabled={busy || !canSubmit}>
                <IconCheck /> {busy ? "Creating…" : "Create job"}
              </button>
            )}
          </div>
        </div>
      </div>
    </>
  );
}

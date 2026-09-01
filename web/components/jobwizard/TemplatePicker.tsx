"use client";

import { useEffect, useState } from "react";
import type { JobTemplate } from "@/lib/types";
import type { Patch } from "./types";

/**
 * Start-from-a-template control at the top of the script step.
 *
 * It applies the template's script, interpreter, and schedule into the draft and then
 * gets out of the way. There is no live link back to the template afterwards, so
 * editing the script here can never surprise anyone else, and the picker collapses
 * once used to make clear that the draft is now the user's own.
 */
export function TemplatePicker({ set }: { set: (p: Patch) => void }) {
  const [templates, setTemplates] = useState<JobTemplate[]>([]);
  const [open, setOpen] = useState(false);
  const [applied, setApplied] = useState<string | null>(null);

  useEffect(() => {
    let live = true;
    fetch("/api/job-templates")
      .then((r) => (r.ok ? r.json() : { items: [] }))
      .then((b) => { if (live) setTemplates(b.items ?? []); })
      .catch(() => { /* the picker is optional; a failure just hides it */ });
    return () => { live = false; };
  }, []);

  if (templates.length === 0) return null;

  function apply(t: JobTemplate) {
    set({
      name: t.name.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, ""),
      description: t.description,
      interpreter: t.interpreter,
      scriptBody: t.script_body,
      scheduleCron: t.schedule_cron,
      timezone: t.timezone,
    });
    setApplied(t.name);
    setOpen(false);
  }

  const byCategory = groupBy(templates, (t) => t.category);

  return (
    <div className="field">
      <div className="row">
        <label style={{ margin: 0 }}>
          {applied ? `Started from "${applied}"` : "Start from a template"}
        </label>
        <button type="button" className="button secondary sm" onClick={() => setOpen((v) => !v)}>
          {open ? "Close" : applied ? "Pick another" : "Browse templates"}
        </button>
      </div>

      {open && (
        <div className="stack" style={{ gap: 14, marginTop: 10 }}>
          {Object.entries(byCategory).map(([category, items]) => (
            <div key={category}>
              <div className="faint" style={{ fontSize: 11, textTransform: "uppercase", letterSpacing: 0.4 }}>
                {category}
              </div>
              <div className="chips" style={{ marginTop: 6 }}>
                {items.map((t) => (
                  <button key={t.id} type="button" className="chip" title={t.description} onClick={() => apply(t)}>
                    {t.name}
                    {!t.builtin && <span className="faint"> (saved)</span>}
                  </button>
                ))}
              </div>
            </div>
          ))}
          <p className="field-hint">
            Applying a template replaces the script, interpreter, and schedule below. It is a
            copy: editing it later does not affect the template or any other job.
          </p>
        </div>
      )}
    </div>
  );
}

function groupBy<T>(items: T[], key: (t: T) => string): Record<string, T[]> {
  const out: Record<string, T[]> = {};
  for (const item of items) {
    const k = key(item);
    (out[k] ??= []).push(item);
  }
  return out;
}

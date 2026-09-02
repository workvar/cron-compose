import type { JobDraft, Patch } from "./types";
import { CRON_PRESETS, COMMON_TZ, describeCron, isValidCron } from "./types";
import { IconCalendarClock } from "@/components/icons";
import { SearchableSelect } from "@/components/SearchableSelect";

export function StepSchedule({ draft, set }: { draft: JobDraft; set: (p: Patch) => void }) {
  const valid = isValidCron(draft.scheduleCron);

  return (
    <div>
      <h2 className="step-h">When should it run?</h2>
      <p className="step-lead">Pick a preset or write a cron expression. Times use the job&apos;s timezone.</p>

      <div className="field">
        <label>Presets</label>
        <div className="chips">
          {CRON_PRESETS.map((p) => (
            <button type="button" key={p.cron} className={`chip${draft.scheduleCron === p.cron ? " selected" : ""}`} onClick={() => set({ scheduleCron: p.cron })}>
              {p.label}
            </button>
          ))}
        </div>
      </div>

      <div className="grid-2">
        <div className="field">
          <label>Cron expression</label>
          <input className="code-editor" value={draft.scheduleCron} onChange={(e) => set({ scheduleCron: e.target.value })} placeholder="0 */6 * * *" />
          {!valid && <p className="field-hint" style={{ color: "var(--danger)" }}>A cron expression has 5 fields (min hour day month weekday).</p>}
        </div>
        <div className="field">
          <label htmlFor="job-tz">Timezone</label>
          <SearchableSelect
            id="job-tz"
            value={draft.timezone}
            onChange={(v) => set({ timezone: v })}
            allowCustom
            options={COMMON_TZ.map((tz) => ({ value: tz, label: tz }))}
            placeholder="UTC"
          />
        </div>
      </div>

      <div className="panel" style={{ background: "var(--surface-2)", display: "flex", gap: 12, alignItems: "center", padding: "14px 16px" }}>
        <span className="mini-icon"><IconCalendarClock /></span>
        <div>
          <div style={{ fontWeight: 700, color: "var(--text)" }}>{describeCron(draft.scheduleCron)}</div>
          <div className="subtle" style={{ fontSize: 12 }}>Evaluated in {draft.timezone || "UTC"}.</div>
        </div>
      </div>

      <div className="field" style={{ marginTop: 18 }}>
        <label htmlFor="job-catchup">Catch-up policy (missed runs while a server was offline)</label>
        <SearchableSelect
          id="job-catchup"
          value={draft.catchupPolicy}
          onChange={(v) => set({ catchupPolicy: v as JobDraft["catchupPolicy"] })}
          options={[
            { value: "skip", label: "skip — don't run missed schedules" },
            { value: "once", label: "once — run a single catch-up" },
            { value: "all", label: "all — run every missed schedule" },
          ]}
        />
      </div>
    </div>
  );
}

import type { JobDraft, Patch } from "./types";
import { INTERPRETERS } from "./types";
import { SecretRefsField } from "./SecretRefsField";
import { TemplatePicker } from "./TemplatePicker";
import { IconChevronRight } from "@/components/icons";

export function StepScript({ draft, set }: { draft: JobDraft; set: (p: Patch) => void }) {
  return (
    <div>
      <h2 className="step-h">What should it do?</h2>
      <p className="step-lead">Name the job, pick an interpreter, and write the script.</p>

      <TemplatePicker set={set} />

      <div className="grid-2">
        <div className="field">
          <label>Name</label>
          <input value={draft.name} onChange={(e) => set({ name: e.target.value })} placeholder="backup-photos" autoFocus />
        </div>
        <div className="field">
          <label>Interpreter</label>
          <div className="chips" style={{ marginBottom: 8 }}>
            {INTERPRETERS.map((it) => (
              <button type="button" key={it} className={`chip${draft.interpreter === it ? " selected" : ""}`} onClick={() => set({ interpreter: it })}>
                {it}
              </button>
            ))}
          </div>
          <input value={draft.interpreter} onChange={(e) => set({ interpreter: e.target.value })} placeholder="bash" />
        </div>
      </div>

      <div className="field">
        <label>Description (optional)</label>
        <input value={draft.description} onChange={(e) => set({ description: e.target.value })} placeholder="Nightly backup of the photo library" />
      </div>

      <div className="field">
        <label>Script</label>
        <textarea
          className="code-editor"
          value={draft.scriptBody}
          onChange={(e) => set({ scriptBody: e.target.value })}
          rows={13}
          spellCheck={false}
        />
      </div>

      <details className="advanced">
        <summary><span className="chev"><IconChevronRight /></span> Advanced execution options</summary>
        <div className="adv-body">
          <div className="grid-3">
            <div className="field">
              <label>Timeout (seconds)</label>
              <input type="number" min={0} value={draft.timeoutSeconds} onChange={(e) => set({ timeoutSeconds: Number(e.target.value) })} />
            </div>
            <div className="field">
              <label>Concurrency</label>
              <select value={draft.concurrencyPolicy} onChange={(e) => set({ concurrencyPolicy: e.target.value as JobDraft["concurrencyPolicy"] })}>
                <option value="skip">skip if running</option>
                <option value="allow">allow overlap</option>
                <option value="queue">queue</option>
              </select>
            </div>
            <div className="field">
              <label>Max retries</label>
              <input type="number" min={0} value={draft.maxRetries} onChange={(e) => set({ maxRetries: Number(e.target.value) })} />
            </div>
            <div className="field">
              <label>CPU quota (% of one core, 0 = unlimited)</label>
              <input type="number" min={0} value={draft.cpuPct} onChange={(e) => set({ cpuPct: Number(e.target.value) })} />
            </div>
            <div className="field">
              <label>Memory limit (MB, 0 = unlimited)</label>
              <input type="number" min={0} value={draft.memMB} onChange={(e) => set({ memMB: Number(e.target.value) })} />
            </div>
            <div className="field">
              <label>Max processes (0 = unlimited)</label>
              <input type="number" min={0} value={draft.tasksMax} onChange={(e) => set({ tasksMax: Number(e.target.value) })} />
            </div>
            <div className="field">
              <label>IO weight (1-10000, 0 = default)</label>
              <input type="number" min={0} max={10000} value={draft.ioWeight} onChange={(e) => set({ ioWeight: Number(e.target.value) })} />
            </div>
            <div className="field">
              <label>Run as user (optional)</label>
              <input value={draft.runAsUser} onChange={(e) => set({ runAsUser: e.target.value })} placeholder="root" />
            </div>
          </div>
          <div className="field">
            <label>Working directory (optional)</label>
            <input value={draft.workingDir} onChange={(e) => set({ workingDir: e.target.value })} placeholder="/opt/app" />
          </div>
          <div className="field">
            <label>Secrets</label>
            <SecretRefsField value={draft.secretRefs} onChange={(next) => set({ secretRefs: next })} />
          </div>
        </div>
      </details>
    </div>
  );
}

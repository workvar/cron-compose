"use client";

import { useState } from "react";
import { stepUpWithPasskey } from "@/lib/webauthn";

type StepUp = {
  challenge_id: string;
  credential: Record<string, unknown>;
};

type Props = {
  /** Keys to show when values are not yet loaded (from env_keys attribute). */
  keys?: string[];
  /** Plaintext env when already known (still masked until reveal). */
  env?: Record<string, string>;
  /** Fetch plaintext after passkey; receives the step-up assertion. */
  onStepUpReveal?: (step: StepUp) => Promise<Record<string, string>>;
};

/** Shows env keys with dotted values until passkey step-up reveals plaintext. */
export function MaskedEnvField({ keys, env, onStepUpReveal }: Props) {
  const [open, setOpen] = useState(false);
  const [values, setValues] = useState<Record<string, string>>(env || {});
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const displayKeys = (keys && keys.length > 0
    ? keys
    : Object.keys(values)
  ).filter(Boolean).sort();

  async function reveal() {
    setBusy(true);
    setError(null);
    try {
      const step = await stepUpWithPasskey();
      if (onStepUpReveal) {
        const next = await onStepUpReveal(step);
        setValues(next);
      } else if (!env || Object.keys(env).length === 0) {
        throw new Error("Nothing to reveal");
      }
      setOpen(true);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  if (displayKeys.length === 0) {
    return <p className="subtle" style={{ margin: 0 }}>No environment variables.</p>;
  }

  return (
    <div className="masked-env">
      <div className="masked-env-head">
        <span className="subtle" style={{ fontSize: 12 }}>{displayKeys.length} variables</span>
        {!open ? (
          <button type="button" className="button secondary sm" disabled={busy} onClick={reveal}>
            {busy ? "Verifying…" : "Reveal with passkey"}
          </button>
        ) : (
          <button type="button" className="button secondary sm" onClick={() => setOpen(false)}>
            Hide
          </button>
        )}
      </div>
      {error && <p className="form-error">{error}</p>}
      <div className="masked-env-list">
        {displayKeys.map((k) => (
          <div key={k} className="masked-env-row">
            <code className="masked-env-key">{k}</code>
            <span className="masked-env-val">
              {open ? (values[k] ?? "") : "••••••••••••"}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}

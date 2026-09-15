"use client";

// The health check is opt-in, so this form is the only way to turn it on. It is split
// out of ProjectActions to keep that component about the project's basics.

export type HealthCheckValues = {
  path: string;
  port: string;
  timeout: string;
  deployTimeout: string;
};

type Props = {
  value: HealthCheckValues;
  onChange: (next: HealthCheckValues) => void;
  /** The app's port, used as the probe's default target. */
  appPort: number;
};

export function HealthCheckFields({ value, onChange, appPort }: Props) {
  const set = (patch: Partial<HealthCheckValues>) => onChange({ ...value, ...patch });
  const on = value.path.trim() !== "";

  return (
    <>
      <div className="field">
        <label htmlFor="health-path">Health check path</label>
        <input
          id="health-path"
          placeholder="/healthz"
          value={value.path}
          onChange={(e) => set({ path: e.target.value })}
        />
        <p className="field-hint">
          {on
            ? "A deploy is not successful until the app answers here. Leave empty to go back to treating the install script's exit code as the whole verdict."
            : "Empty: a deploy counts as successful as soon as the install script exits 0, even if the app then crashes on boot. Set a path to make each deploy prove the app came up, and to let auto-rollback catch that case."}
        </p>
      </div>
      {on && (
        <div className="grid-2">
          <div className="field">
            <label htmlFor="health-port">Health check port</label>
            <input
              id="health-port"
              placeholder={appPort > 0 ? String(appPort) : "same as PORT"}
              value={value.port}
              onChange={(e) => set({ port: e.target.value })}
            />
            <p className="field-hint">Probed on 127.0.0.1. Defaults to the app&apos;s PORT.</p>
          </div>
          <div className="field">
            <label htmlFor="health-timeout">Seconds to become healthy</label>
            <input
              id="health-timeout"
              placeholder="60"
              value={value.timeout}
              onChange={(e) => set({ timeout: e.target.value })}
            />
            <p className="field-hint">Polled every 3s until this runs out, then the run fails.</p>
          </div>
        </div>
      )}
      <div className="field">
        <label htmlFor="deploy-timeout">Deploy timeout (seconds)</label>
        <input
          id="deploy-timeout"
          placeholder="900"
          value={value.deployTimeout}
          onChange={(e) => set({ deployTimeout: e.target.value })}
        />
        <p className="field-hint">
          Whole-run budget, so a wedged installer cannot hold the project forever. Empty uses the
          agent&apos;s default of 15 minutes; the agent caps it at 2 hours.
        </p>
      </div>
    </>
  );
}

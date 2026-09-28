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
  /** Makes every field id unique when this form appears more than once on a page
   * (the project-wide Advanced section, plus one per app block). */
  idPrefix?: string;
  /** Omit the deploy-timeout field: it's a whole-run budget, not per-app. */
  hideDeployTimeout?: boolean;
};

export function HealthCheckFields({ value, onChange, appPort, idPrefix = "", hideDeployTimeout }: Props) {
  const set = (patch: Partial<HealthCheckValues>) => onChange({ ...value, ...patch });
  const on = value.path.trim() !== "";
  const id = (name: string) => `${idPrefix}${name}`;

  return (
    <>
      <div className="field">
        <label htmlFor={id("health-path")}>Health check path</label>
        <input
          id={id("health-path")}
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
            <label htmlFor={id("health-port")}>Health check port</label>
            <input
              id={id("health-port")}
              placeholder={appPort > 0 ? String(appPort) : "same as PORT"}
              value={value.port}
              onChange={(e) => set({ port: e.target.value })}
            />
            <p className="field-hint">Probed on 127.0.0.1. Defaults to the app&apos;s PORT.</p>
          </div>
          <div className="field">
            <label htmlFor={id("health-timeout")}>Seconds to become healthy</label>
            <input
              id={id("health-timeout")}
              placeholder="60"
              value={value.timeout}
              onChange={(e) => set({ timeout: e.target.value })}
            />
            <p className="field-hint">Polled every 3s until this runs out, then the run fails.</p>
          </div>
        </div>
      )}
      {!hideDeployTimeout && (
        <div className="field">
          <label htmlFor={id("deploy-timeout")}>Deploy timeout (seconds)</label>
          <input
            id={id("deploy-timeout")}
            placeholder="900"
            value={value.deployTimeout}
            onChange={(e) => set({ deployTimeout: e.target.value })}
          />
          <p className="field-hint">
            Whole-run budget, so a wedged installer cannot hold the project forever. Empty uses the
            agent&apos;s default of 15 minutes; the agent caps it at 2 hours.
          </p>
        </div>
      )}
    </>
  );
}

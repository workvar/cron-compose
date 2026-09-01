import type { HealthStatus } from "@/lib/health";
import { SetupBannerActions } from "./SetupBannerActions";

// Shown above page content whenever the control plane can't reach Postgres (or
// isn't reachable at all). In dev, database setup runs automatically; a retry
// button appears if that fails.
export function SetupBanner({ status }: { status: HealthStatus }) {
  if (status === "ok") return null;

  const dbDown = status === "db_down";

  return (
    <div className="setup-banner" role="alert">
      <div className="setup-banner-head">
        <span className="setup-banner-dot" />
        <div>
          <strong>{dbDown ? "Postgres isn't connected" : "Control plane isn't reachable"}</strong>
          <p>
            {dbDown
              ? "Preparing your local Postgres database and applying migrations. This usually takes a few seconds."
              : "Couldn't reach the control plane API. Make sure it's running, then retry."}
          </p>
          <SetupBannerActions status={status} />
        </div>
      </div>
    </div>
  );
}

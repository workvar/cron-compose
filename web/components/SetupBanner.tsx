import type { HealthStatus } from "@/lib/health";
import { IconChevronRight } from "./icons";

// Shown above page content whenever the control plane can't reach Postgres (or
// isn't reachable at all). Pure server component: the expand/collapse is a
// native <details> disclosure, no client JS needed.
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
              ? "The control plane is running but can't reach the database, so servers, jobs, and runs won't load until it's connected."
              : "Couldn't reach the control plane API at all. Make sure it's running, then reload this page."}
          </p>
        </div>
      </div>

      <details className="setup-banner-details">
        <summary>
          <IconChevronRight />
          Show setup steps
        </summary>
        <ol>
          <li>
            Start Postgres:
            <pre><code>make db-up</code></pre>
          </li>
          <li>
            Apply migrations:
            <pre><code>make migrate</code></pre>
          </li>
          <li>
            Restart the control plane:
            <pre><code>{"make control-plane && ./control-plane/bin/control-plane"}</code></pre>
          </li>
        </ol>
        <p className="subtle">
          Full setup in <code>DEVELOPMENT.md</code> under &ldquo;First run&rdquo;.
        </p>
      </details>
    </div>
  );
}

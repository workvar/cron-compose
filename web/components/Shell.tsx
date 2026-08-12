import { cookies } from "next/headers";
import { apiGet, ApiError } from "@/lib/api";
import { getHealth } from "@/lib/health";
import type { Me } from "@/lib/types";
import { Sidebar } from "./Sidebar";
import { Topbar } from "./Topbar";
import { SetupBanner } from "./SetupBanner";

// Placeholder identity for the case where the session cookie is present but the
// control plane can't confirm it. The frame renders, each page shows its own
// degraded state, and the setup banner explains why.
const UNKNOWN_ME: Me = { id: "", email: "", name: "Signed in", role: "viewer" };

// Wraps every route. The bare centered shell is for genuinely signed-out visitors
// (/login); everyone else gets the full-width sidebar + topbar frame. A failed /me
// is deliberately NOT treated as signed out: when the control plane is unreachable
// the whole app would otherwise collapse into a 410px auth column.
export async function Shell({ children }: { children: React.ReactNode }) {
  const [meResult, health, cookieJar] = await Promise.all([
    apiGet<Me>("/me").then(
      (me) => ({ me, error: null as ApiError | null }),
      (error: unknown) => ({ me: null, error: error as ApiError }),
    ),
    getHealth(),
    cookies(),
  ]);

  const signedOut =
    !meResult.me && meResult.error instanceof ApiError && meResult.error.unauthorized;
  const hasSessionCookie = cookieJar.has("cc_session");

  if (signedOut || (!meResult.me && !hasSessionCookie)) {
    return (
      <div className="auth-shell">
        <div className="auth-shell-inner">
          <SetupBanner status={health} />
          {children}
        </div>
      </div>
    );
  }

  return (
    <div className="app-shell">
      <Sidebar me={meResult.me ?? UNKNOWN_ME} />
      <div className="app-main">
        <Topbar me={meResult.me ?? UNKNOWN_ME} />
        <main className="content">
          <SetupBanner status={health} />
          {children}
        </main>
      </div>
    </div>
  );
}

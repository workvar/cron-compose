import { cookies, headers } from "next/headers";
import { redirect } from "next/navigation";
import { apiGet, ApiError } from "@/lib/api";
import { getHealth } from "@/lib/health";
import type { ListResponse, Me, Server } from "@/lib/types";
import { Sidebar } from "./Sidebar";
import { Topbar } from "./Topbar";
import { SetupBanner } from "./SetupBanner";

// Placeholder identity for the case where the session cookie is present but the
// control plane can't confirm it (for example the plane is unreachable). The
// frame still renders so the setup banner can explain why.
const UNKNOWN_ME: Me = { id: "", email: "", name: "Signed in", role: "viewer" };

// Wraps every route. The bare centered shell is for /login. Everyone else gets
// the full-width sidebar + topbar frame. A failed /me is NOT treated as signed
// out when the control plane is unreachable — otherwise the whole app collapses
// into a 410px auth column. A 401/403 is a dead session: send them to login.
export async function Shell({ children }: { children: React.ReactNode }) {
  const [meResult, health, cookieJar, serverCount, hdrs] = await Promise.all([
    apiGet<Me>("/me").then(
      (me) => ({ me, error: null as ApiError | null }),
      (error: unknown) => ({ me: null, error: error as ApiError }),
    ),
    getHealth(),
    cookies(),
    apiGet<ListResponse<Server>>("/servers")
      .then((r) => r.items.length)
      .catch(() => 0),
    headers(),
  ]);

  const signedOut =
    !meResult.me && meResult.error instanceof ApiError && meResult.error.unauthorized;
  const hasSessionCookie = cookieJar.has("cc_session");
  const pathname = hdrs.get("x-cc-pathname") || "";

  if (signedOut && pathname !== "/login") {
    redirect("/login");
  }

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
      <Sidebar me={meResult.me ?? UNKNOWN_ME} serverCount={serverCount} />
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

import { cookies, headers } from "next/headers";
import { redirect } from "next/navigation";
import { apiGet, ApiError } from "@/lib/api";
import { getHealth } from "@/lib/health";
import { isPublicPath, publicActiveNav } from "@/lib/public-routes";
import type { ListResponse, Me, Server } from "@/lib/types";
import { AppShell } from "./AppShell";
import { PublicChrome } from "./public/PublicChrome";
import { Sidebar } from "./Sidebar";
import { Topbar } from "./Topbar";
import { SetupBanner } from "./SetupBanner";

// Placeholder identity for the case where the session cookie is present but the
// control plane can't confirm it (for example the plane is unreachable). The
// frame still renders so the setup banner can explain why.
const UNKNOWN_ME: Me = { id: "", email: "", name: "Signed in", role: "viewer" };

async function loadMe(): Promise<{ me: Me | null; error: ApiError | null }> {
  return apiGet<Me>("/me").then(
    (me) => ({ me, error: null as ApiError | null }),
    (error: unknown) => ({ me: null, error: error as ApiError }),
  );
}

// Wraps every route. Login is full-bleed. Public marketing pages use PublicChrome
// for anonymous visitors. Authenticated app routes get the sidebar shell.
// Never call /me when there is no session cookie — that 401 "missing session"
// was showing up every time someone opened /login.
export async function Shell({ children }: { children: React.ReactNode }) {
  const cookieJar = await cookies();
  const hdrs = await headers();
  const pathname = hdrs.get("x-cc-pathname") || "";
  const hasSessionCookie = cookieJar.has("cc_session");
  const publicPage = isPublicPath(pathname);
  const isLogin = pathname === "/login" || pathname.startsWith("/login/");
  const isLanding = pathname === "/landing" || pathname.startsWith("/landing/");
  const isTermFullscreen = /^\/servers\/[^/]+\/terminal\/full\/?$/.test(pathname);

  if (isLogin) {
    const health = await getHealth();
    return (
      <div className="auth-shell auth-shell-split">
        {health !== "ok" ? (
          <div className="auth-setup-overlay">
            <SetupBanner status={health} />
          </div>
        ) : null}
        {children}
      </div>
    );
  }

  if (publicPage) {
    // Only probe /me when a cookie exists — avoids 401 "missing session" noise.
    const meResult = hasSessionCookie
      ? await loadMe()
      : { me: null, error: null as ApiError | null };
    const signedOut =
      !meResult.me && meResult.error instanceof ApiError && meResult.error.unauthorized;
    const anonymous = !hasSessionCookie || signedOut || !meResult.me;

    // Landing is always the marketing chrome (even if signed in).
    if (anonymous || isLanding) {
      return (
        <PublicChrome active={publicActiveNav(pathname)}>
          {children}
        </PublicChrome>
      );
    }

    const [health, serverCount] = await Promise.all([
      getHealth(),
      apiGet<ListResponse<Server>>("/servers")
        .then((r) => r.items.length)
        .catch(() => 0),
    ]);
    return (
      <AppShell>
        <Sidebar me={meResult.me ?? UNKNOWN_ME} serverCount={serverCount} />
        <div className="app-main">
          <Topbar me={meResult.me ?? UNKNOWN_ME} />
          <main className="content">
            <SetupBanner status={health} />
            {children}
          </main>
        </div>
      </AppShell>
    );
  }

  // Private app routes
  if (!hasSessionCookie) {
    const dest = pathname && pathname !== "/" ? `/login?next=${encodeURIComponent(pathname)}` : "/login";
    redirect(dest);
  }

  const [meResult, health, serverCount] = await Promise.all([
    loadMe(),
    getHealth(),
    apiGet<ListResponse<Server>>("/servers")
      .then((r) => r.items.length)
      .catch(() => 0),
  ]);

  const signedOut =
    !meResult.me && meResult.error instanceof ApiError && meResult.error.unauthorized;

  if (signedOut) {
    const dest = pathname && pathname !== "/" ? `/login?next=${encodeURIComponent(pathname)}` : "/login";
    redirect(dest);
  }

  if (!meResult.me) {
    return (
      <div className="auth-shell">
        <div className="auth-shell-inner">
          <SetupBanner status={health} />
          {children}
        </div>
      </div>
    );
  }

  if (isTermFullscreen) {
    return <div className="term-fullscreen-shell">{children}</div>;
  }

  return (
    <AppShell>
      <Sidebar me={meResult.me} serverCount={serverCount} />
      <div className="app-main">
        <Topbar me={meResult.me} />
        <main className="content">
          <SetupBanner status={health} />
          {children}
        </main>
      </div>
    </AppShell>
  );
}

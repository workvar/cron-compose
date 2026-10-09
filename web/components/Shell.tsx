import { cookies, headers } from "next/headers";
import { redirect } from "next/navigation";
import { apiGet, ApiError } from "@/lib/api";
import { getHealth } from "@/lib/health";
import { isPublicPath } from "@/lib/public-routes";
import type { ListResponse, Me, Server } from "@/lib/types";
import { ShellFrame } from "./ShellFrame";

async function loadMe(): Promise<{ me: Me | null; error: ApiError | null }> {
  return apiGet<Me>("/me").then(
    (me) => ({ me, error: null as ApiError | null }),
    (error: unknown) => ({ me: null, error: error as ApiError }),
  );
}

// Server gate + data load. Chrome (login / marketing / app) is chosen in
// ShellFrame from the client pathname so soft navigations update immediately.
export async function Shell({ children }: { children: React.ReactNode }) {
  const cookieJar = await cookies();
  const hdrs = await headers();
  const pathname = hdrs.get("x-cc-pathname") || "";
  const hasSessionCookie = cookieJar.has("cc_session");
  const publicPage = isPublicPath(pathname);
  const isLogin = pathname === "/login" || pathname.startsWith("/login/");

  // Private routes without a cookie → login (proxy.ts does the same for navigations).
  if (!isLogin && !publicPage && !hasSessionCookie) {
    const dest =
      pathname && pathname !== "/"
        ? `/login?next=${encodeURIComponent(pathname)}`
        : "/login";
    redirect(dest);
  }

  let me: Me | null = null;
  let meError: ApiError | null = null;
  // Never probe /me without a cookie — that 401 "missing session" is noise on login.
  if (hasSessionCookie) {
    const result = await loadMe();
    me = result.me;
    meError = result.error;
  }

  const signedOut =
    hasSessionCookie &&
    !me &&
    meError instanceof ApiError &&
    meError.unauthorized;

  if (!isLogin && !publicPage && (signedOut || (!me && meError?.unauthorized))) {
    const dest =
      pathname && pathname !== "/"
        ? `/login?next=${encodeURIComponent(pathname)}`
        : "/login";
    redirect(dest);
  }

  const health = await getHealth();
  let serverCount = 0;
  if (me && !isLogin) {
    serverCount = await apiGet<ListResponse<Server>>("/servers")
      .then((r) => r.items.length)
      .catch(() => 0);
  }

  return (
    <ShellFrame me={me} health={health} serverCount={serverCount}>
      {children}
    </ShellFrame>
  );
}

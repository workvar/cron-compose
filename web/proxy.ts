// Request proxy: bounce unauthenticated requests to /login. The real auth check
// happens server-side; this just keeps the UI from rendering forbidden pages.
import { NextResponse, type NextRequest } from "next/server";

const PUBLIC_PATHS = ["/login"];

function next(req: NextRequest) {
  const headers = new Headers(req.headers);
  headers.set("x-cc-pathname", req.nextUrl.pathname);
  return NextResponse.next({ request: { headers } });
}

export function proxy(req: NextRequest) {
  const { pathname } = req.nextUrl;

  // Always allow next-internal, static assets, and /api. /api reaches the control
  // plane via a Next rewrite (dev/standalone) or the control plane directly (prod,
  // where it is the single entry point); the control plane enforces auth there.
  if (
    pathname.startsWith("/_next") ||
    pathname.startsWith("/api") ||
    pathname.startsWith("/icon") ||
    pathname.startsWith("/apple-icon") ||
    pathname.includes(".")
  ) {
    return next(req);
  }

  const hasSession = req.cookies.has("cc_session");
  if (!hasSession && !PUBLIC_PATHS.includes(pathname)) {
    const url = req.nextUrl.clone();
    url.pathname = "/login";
    url.searchParams.set("next", pathname);
    return NextResponse.redirect(url);
  }
  // Do not bounce /login just because a cookie exists: after a restart the cookie
  // is often stale (bad HMAC). Forcing /login → / would loop and POST /servers
  // would keep returning 401. The login page redirects itself once /me succeeds.
  return next(req);
}

export const config = {
  matcher: ["/((?!_next/static|_next/image|favicon.ico).*)"],
};

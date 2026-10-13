// Request proxy: bounce unauthenticated requests to /login. The real auth check
// happens server-side; this just keeps the UI from rendering forbidden pages.
import { NextResponse, type NextRequest } from "next/server";

const PUBLIC_PATHS = ["/login", "/landing"];
// Marketing + docs are readable signed out.
const PUBLIC_PREFIXES = ["/docs", "/landing", "/use-cases"];

function isPublic(pathname: string) {
  return (
    PUBLIC_PATHS.includes(pathname) ||
    PUBLIC_PREFIXES.some((p) => pathname === p || pathname.startsWith(p + "/"))
  );
}

function next(req: NextRequest) {
  const headers = new Headers(req.headers);
  headers.set("x-cc-pathname", req.nextUrl.pathname);
  return NextResponse.next({ request: { headers } });
}

export function proxy(req: NextRequest) {
  const { pathname } = req.nextUrl;
  // Full URL path before basePath stripping. Dashboard is also pathname "/" after
  // /app is removed — must not treat that as the bare site root.
  const rawPath = new URL(req.url).pathname;

  // Always allow next-internal, static assets, and /api. /api reaches the control
  // plane via a Next rewrite (dev/standalone) or the control plane directly (prod,
  // where it is the single entry point); the control plane enforces auth there.
  if (
    pathname.startsWith("/_next") ||
    pathname.startsWith("/api") ||
    pathname.startsWith("/icon") ||
    pathname.startsWith("/apple-icon") ||
    pathname.startsWith("/opengraph-image") ||
    pathname.startsWith("/twitter-image") ||
    pathname.includes(".")
  ) {
    return next(req);
  }

  const hasSession = req.cookies.has("cc_session");

  // Bare host root only (outside basePath). Control plane does the same when it
  // fronts the site; this covers direct hits on the Next port (dev / tunnel).
  // Use /app not /app/ — Next's default trailingSlash:false 308s /app/ → /app,
  // and bouncing the dashboard root here used to 404 soft navigations to "/".
  if (rawPath === "/") {
    return NextResponse.redirect(
      new URL(hasSession ? "/app" : "/app/landing", req.url),
    );
  }

  if (!hasSession && !isPublic(pathname)) {
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
  // Include "/" so the dashboard root (basePath-stripped) still gets x-cc-pathname.
  matcher: [
    "/",
    "/((?!_next/static|_next/image|favicon.ico|.*\\.(?:svg|png|ico|webp)$).*)",
  ],
};

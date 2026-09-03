import type { NextConfig } from "next";

// Only the /api rewrite below needs a build-time value; server code reads API_BASE
// at request time via lib/apiBase.ts so a bundle built with one port still works
// when pm2 starts it with another.
// API_BASE is the control-plane versioned root (…/api/v1). Rewrites below strip that
// suffix so both /api/foo and /api/v1/foo proxy correctly. Concatenating path onto
// API_BASE used to turn CONTROL_PLANE_HTTP=…/api/v1 into /api/v1/v1/… (401 missing session).
const apiBase = process.env.API_BASE ?? "http://localhost:8080/api/v1";
const apiOrigin = apiBase.replace(/\/api\/v1\/?$/, "");

const config: NextConfig = {
  // Standalone output for tiny production Docker images.
  output: "standalone",

  // The whole UI lives under /app, so the control plane (the single entry point)
  // serves /api itself and reverse-proxies /app to this app. next/link, next/router
  // and redirect() pick this prefix up automatically; raw fetch() does not (see the
  // rewrite below), and next/image src would need it added by hand (none used).
  basePath: "/app",

  // Browser calls to /api/* map onto the control plane's /api/v1. In production the
  // control plane fronts /api directly (it serves the bare /api prefix and the UI);
  // this rewrite is the standalone/dev path that keeps the UI self-contained when
  // Next is hit directly (e.g. `next dev`, or a Cloudflare Tunnel aimed at :3000).
  // basePath: false keeps the source at the real root (/api/*, not /app/api/*).
  //
  // The UI lives under /app, so a bare / would 404. Next only allows rewrites
  // outside basePath to an http(s) URL, so we loop back to this process for the
  // nginx-style welcome page at public/index.html (exposed as /app/index.html).
  async rewrites() {
    const self = `http://127.0.0.1:${process.env.PORT || "3000"}`;
    return {
      beforeFiles: [
        { source: "/", destination: `${self}/app/index.html`, basePath: false },
      ],
      afterFiles: [
        // More specific first: agents/docs often use …/api/v1 as CONTROL_PLANE_HTTP.
        { source: "/api/v1/:path*", destination: `${apiOrigin}/api/v1/:path*`, basePath: false },
        { source: "/api/:path*", destination: `${apiOrigin}/api/v1/:path*`, basePath: false },
      ],
    };
  },
};

export default config;

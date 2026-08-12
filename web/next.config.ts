import type { NextConfig } from "next";

// Only the /api rewrite below needs a build-time value; server code reads API_BASE
// at request time via lib/apiBase.ts so a bundle built with one port still works
// when pm2 starts it with another.
const apiBase = process.env.API_BASE ?? "http://localhost:8080/api/v1";

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
  // Next is hit directly (e.g. `next dev`). basePath: false keeps the source at the
  // real root (/api/*, not /app/api/*) so the client's fetch("/api/..") still
  // matches; allowed here because the destination is an absolute (external) URL.
  async rewrites() {
    return [
      { source: "/api/:path*", destination: `${apiBase}/:path*`, basePath: false },
    ];
  },
};

export default config;

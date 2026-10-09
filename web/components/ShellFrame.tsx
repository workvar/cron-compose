"use client";

import { usePathname } from "next/navigation";
import type { ReactNode } from "react";
import { isPublicPath, publicActiveNav } from "@/lib/public-routes";
import type { Me } from "@/lib/types";
import type { HealthStatus } from "@/lib/health";
import { AppShell } from "./AppShell";
import { PublicChrome } from "./public/PublicChrome";
import { Sidebar } from "./Sidebar";
import { Topbar } from "./Topbar";
import { SetupBanner } from "./SetupBanner";

/**
 * Picks chrome from the client pathname so soft navigations (docs → login,
 * logout → login) swap shells immediately. The root layout's server Shell is
 * often skipped on client transitions, which left PublicChrome wrapped around
 * the login page until a full refresh.
 */
export function ShellFrame({
  me,
  health,
  serverCount,
  children,
}: {
  me: Me | null;
  health: HealthStatus;
  serverCount: number;
  children: ReactNode;
}) {
  const pathname = usePathname() || "";
  const isLogin = pathname === "/login" || pathname.startsWith("/login/");
  const isLanding = pathname === "/landing" || pathname.startsWith("/landing/");
  const isTermFullscreen = /^\/servers\/[^/]+\/terminal\/full\/?$/.test(pathname);
  const publicPage = isPublicPath(pathname);

  if (isLogin) {
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

  if (publicPage && (!me || isLanding)) {
    return (
      <PublicChrome active={publicActiveNav(pathname)}>
        {children}
      </PublicChrome>
    );
  }

  if (!me) {
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
      <Sidebar me={me} serverCount={serverCount} />
      <div className="app-main">
        <Topbar me={me} />
        <main className="content">
          <SetupBanner status={health} />
          {children}
        </main>
      </div>
    </AppShell>
  );
}

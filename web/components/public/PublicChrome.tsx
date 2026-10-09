import Link from "next/link";
import type { ReactNode } from "react";
import { LogoMark } from "@/components/LogoMark";
import { ThemeToggle } from "@/components/ThemeToggle";

const NAV = [
  { href: "/", label: "Home", external: true },
  { href: "/use-cases", label: "Use cases" },
  { href: "/docs", label: "Docs" },
] as const;

export function PublicChrome({
  children,
  active,
}: {
  children: ReactNode;
  /** Highlight the matching nav item. */
  active?: "home" | "use-cases" | "docs";
}) {
  return (
    <div className="public-shell">
      <header className="public-header">
        <a className="brand public-brand" href="/" aria-label="CronCompose home">
          <span className="mark"><LogoMark /></span>
          <span>CronCompose</span>
        </a>

        <nav className="public-nav" aria-label="Marketing">
          {NAV.map((item) => {
            const isActive =
              (active === "home" && item.label === "Home") ||
              (active === "use-cases" && item.href.startsWith("/use-cases")) ||
              (active === "docs" && item.href === "/docs");
            if ("external" in item && item.external) {
              return (
                <a
                  key={item.label}
                  href={item.href}
                  className={`public-nav-link${isActive ? " active" : ""}`}
                >
                  {item.label}
                </a>
              );
            }
            return (
              <Link
                key={item.href}
                href={item.href}
                className={`public-nav-link${isActive ? " active" : ""}`}
              >
                {item.label}
              </Link>
            );
          })}
        </nav>

        <div className="public-header-actions">
          <ThemeToggle />
          <Link href="/login" className="button sm">Get started now</Link>
        </div>
      </header>

      <main className="public-main">{children}</main>

      <footer className="public-footer">
        <div className="public-footer-inner">
          <div className="public-footer-brand">
            <a className="brand" href="/" aria-label="CronCompose home">
              <span className="mark"><LogoMark /></span>
              <span>CronCompose</span>
            </a>
            <p>Deploy and schedule jobs across your Linux fleet — offline-first.</p>
          </div>
          <div className="public-footer-cols">
            <div>
              <div className="public-footer-label">Product</div>
              <a href="/">Home</a>
              <Link href="/use-cases">Use cases</Link>
              <Link href="/docs">Docs</Link>
            </div>
            <div>
              <div className="public-footer-label">Get started</div>
              <Link href="/login">Sign in</Link>
              <Link href="/login">Create account</Link>
            </div>
            <div>
              <div className="public-footer-label">Resources</div>
              <Link href="/docs#quick-start">Quick start</Link>
              <Link href="/docs#examples">Examples</Link>
              <Link href="/docs#how-it-works">How it works</Link>
            </div>
          </div>
        </div>
        <div className="public-footer-bar">
          <span>© {new Date().getFullYear()} CronCompose</span>
          <a href="/">Back to home</a>
        </div>
      </footer>
    </div>
  );
}

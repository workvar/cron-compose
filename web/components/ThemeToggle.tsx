"use client";

import { useTheme } from "./ThemeProvider";

export function ThemeToggle() {
  const { theme, setTheme, ready } = useTheme();
  const isDark = theme === "dark";

  return (
    <div className="theme-toggle" data-ready={ready ? "1" : "0"}>
      <span className="theme-toggle-label">Appearance</span>
      <div className="theme-switch" role="group" aria-label="Color theme">
        <button
          type="button"
          className={`theme-switch-btn${!isDark ? " on" : ""}`}
          onClick={() => setTheme("light")}
          aria-pressed={!isDark}
        >
          <SunIcon />
          <span>Light</span>
        </button>
        <button
          type="button"
          className={`theme-switch-btn${isDark ? " on" : ""}`}
          onClick={() => setTheme("dark")}
          aria-pressed={isDark}
        >
          <MoonIcon />
          <span>Dark</span>
        </button>
      </div>
    </div>
  );
}

function SunIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
      <circle cx="12" cy="12" r="4" />
      <path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M4.93 19.07l1.41-1.41M17.66 6.34l1.41-1.41" />
    </svg>
  );
}

function MoonIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
      <path d="M21 14.5A8.5 8.5 0 1 1 9.5 3a7 7 0 0 0 11.5 11.5Z" />
    </svg>
  );
}

"use client";

import { createContext, useContext, useEffect, useState, type ReactNode } from "react";

type SidebarState = { collapsed: boolean; toggle: () => void };

const SidebarContext = createContext<SidebarState | null>(null);

const STORAGE_KEY = "cc:sidebar-collapsed";

/** Reads persisted state read-once at construction time, before first paint. */
function readStoredCollapsed(): boolean {
  if (typeof window === "undefined") return false;
  try {
    return window.localStorage.getItem(STORAGE_KEY) === "1";
  } catch {
    return false;
  }
}

/**
 * Wraps the app-shell grid (sidebar + main content) and owns whether the
 * sidebar is collapsed. Sidebar reads/toggles this via useSidebar() so the
 * grid column width and the sidebar's own layout stay in sync.
 */
export function AppShell({ children }: { children: ReactNode }) {
  const [collapsed, setCollapsed] = useState(false);
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setCollapsed(readStoredCollapsed());
    setMounted(true);
  }, []);

  useEffect(() => {
    if (!mounted) return;
    try {
      window.localStorage.setItem(STORAGE_KEY, collapsed ? "1" : "0");
    } catch {
      // Best-effort only; a private/blocked storage just skips persistence.
    }
  }, [collapsed, mounted]);

  const toggle = () => setCollapsed((value) => !value);

  return (
    <SidebarContext.Provider value={{ collapsed, toggle }}>
      <div className={`app-shell${collapsed ? " sidebar-collapsed" : ""}`}>{children}</div>
    </SidebarContext.Provider>
  );
}

export function useSidebar(): SidebarState {
  const ctx = useContext(SidebarContext);
  if (!ctx) throw new Error("useSidebar must be used within <AppShell>");
  return ctx;
}

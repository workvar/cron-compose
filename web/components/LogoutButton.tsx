"use client";

import { IconLogout } from "./icons";

// Hard-navigate after logout so the login page mounts clean (no stale AppShell /
// PublicChrome from a soft transition, and no leftover WebAuthn ceremony).
async function logoutAndLeave() {
  try {
    await fetch("/api/auth/logout", { method: "POST", credentials: "include" });
  } catch {
    /* still leave */
  }
  window.location.assign("/app/login");
}

// `variant="nav"` renders as a sidebar nav item; default renders as a button.
export function LogoutButton({ variant = "button" }: { variant?: "button" | "nav" }) {
  if (variant === "nav") {
    return (
      <button onClick={() => void logoutAndLeave()} className="nav-item" type="button">
        <IconLogout />
        <span>Logout</span>
      </button>
    );
  }

  return (
    <button onClick={() => void logoutAndLeave()} className="button secondary sm" type="button">
      Sign out
    </button>
  );
}

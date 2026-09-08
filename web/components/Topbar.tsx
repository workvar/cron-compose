"use client";

import { useEffect, useRef, useState } from "react";
import type { Me } from "@/lib/types";
import { LogoutButton } from "./LogoutButton";
import { IconSearch, IconMail, IconBell } from "./icons";

function initials(me: Me): string {
  const src = me.name?.trim() || me.email;
  const parts = src.split(/[\s@._-]+/).filter(Boolean);
  if (parts.length >= 2) return (parts[0][0] + parts[1][0]).toUpperCase();
  return src.slice(0, 2).toUpperCase();
}

export function Topbar({ me }: { me: Me }) {
  const name = me.name?.trim() || me.email.split("@")[0];
  const [open, setOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  // Close on outside click or Escape, the two standard ways to dismiss a menu.
  useEffect(() => {
    if (!open) return;

    function onPointerDown(event: PointerEvent) {
      if (!containerRef.current?.contains(event.target as Node)) setOpen(false);
    }
    function onKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") setOpen(false);
    }

    document.addEventListener("pointerdown", onPointerDown);
    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.removeEventListener("pointerdown", onPointerDown);
      document.removeEventListener("keydown", onKeyDown);
    };
  }, [open]);

  return (
    <header className="topbar">
      <div className="search">
        <IconSearch />
        <input type="search" placeholder="Search servers, jobs, runs…" aria-label="Search" />
        <span className="kbd">⌘ F</span>
      </div>

      <div className="topbar-actions">
        <button className="icon-btn" aria-label="Messages" type="button"><IconMail /></button>
        <button className="icon-btn" aria-label="Notifications" type="button"><IconBell /></button>
        <div className="profile" ref={containerRef}>
          <button
            type="button"
            className="profile-trigger"
            onClick={() => setOpen((v) => !v)}
            aria-haspopup="true"
            aria-expanded={open}
            aria-label="Account menu"
          >
            <span className="avatar">{initials(me)}</span>
          </button>

          {open && (
            <div className="profile-dropdown" role="menu">
              <div className="profile-dropdown-who">
                <div className="name">{name}</div>
                <div className="mail">{me.email}</div>
              </div>
              <div className="profile-dropdown-sep" />
              <LogoutButton variant="nav" />
            </div>
          )}
        </div>
      </div>
    </header>
  );
}

"use client";

import Link from "next/link";
import type { Me } from "@/lib/types";
import { Brand } from "./Brand";
import { NavLink } from "./NavLink";
import { useSidebar } from "./AppShell";
import { shouldShowServerPromo } from "@/lib/ui-helpers";
import {
  IconDashboard, IconServer, IconJobs, IconKey, IconShield,
  IconSettings, IconZap, IconPlus, IconPlug, IconPorts, IconGit,
  IconChevronLeft, IconChevronRight,
} from "./icons";

export function Sidebar({ me, serverCount }: { me: Me; serverCount: number }) {
  const isAdmin = me.role === "admin" || me.role === "owner";
  const showPromo = shouldShowServerPromo(serverCount);
  const { collapsed, toggle } = useSidebar();

  return (
    <aside className={`sidebar${collapsed ? " collapsed" : ""}`}>
      <div className="sidebar-top">
        <Brand href="/" />
        <button
          type="button"
          className="sidebar-toggle"
          onClick={toggle}
          aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"}
          title={collapsed ? "Expand sidebar" : "Collapse sidebar"}
        >
          {collapsed ? <IconChevronRight /> : <IconChevronLeft />}
        </button>
      </div>

      <div className="nav-section">
        <div className="nav-label">Menu</div>
        <NavLink href="/" icon={<IconDashboard />}>Dashboard</NavLink>
        <NavLink href="/servers" icon={<IconServer />}>Servers</NavLink>
        <NavLink href="/jobs" icon={<IconJobs />}>Jobs</NavLink>
        <NavLink href="/deploys" icon={<IconGit />}>Deploy</NavLink>
        <NavLink href="/connectors" icon={<IconPlug />}>Connectors</NavLink>
        <NavLink href="/ports" icon={<IconPorts />}>Ports</NavLink>
        {isAdmin && <NavLink href="/secrets" icon={<IconKey />}>Secrets</NavLink>}
        {isAdmin && <NavLink href="/audit" icon={<IconShield />}>Audit</NavLink>}
      </div>

      <div className="nav-section">
        <div className="nav-label">General</div>
        <NavLink href="/settings" icon={<IconSettings />}>Settings</NavLink>
      </div>

      {showPromo && (
        <div className="sidebar-foot">
          <div className="promo">
            <span className="promo-icon"><IconZap /></span>
            <h4>Offline-first agents</h4>
            <p>Each server keeps firing its jobs even when the control plane is unreachable.</p>
            <Link href="/servers/new" className="button sm">
              <IconPlus /> Add server
            </Link>
          </div>
        </div>
      )}
    </aside>
  );
}

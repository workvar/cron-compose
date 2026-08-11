import Link from "next/link";
import type { Me } from "@/lib/types";
import { NavLink } from "./NavLink";
import { LogoutButton } from "./LogoutButton";
import {
  IconDashboard, IconServer, IconJobs, IconKey, IconShield,
  IconSettings, IconZap, IconPlus, IconPlug,
} from "./icons";

export function Sidebar({ me }: { me: Me }) {
  const isAdmin = me.role === "admin" || me.role === "owner";

  return (
    <aside className="sidebar">
      <Link href="/" className="brand">
        <span className="mark"><IconZap /></span>
        <span>CronCompose</span>
      </Link>

      <div className="nav-section">
        <div className="nav-label">Menu</div>
        <NavLink href="/" icon={<IconDashboard />}>Dashboard</NavLink>
        <NavLink href="/servers" icon={<IconServer />}>Servers</NavLink>
        <NavLink href="/jobs" icon={<IconJobs />}>Jobs</NavLink>
        <NavLink href="/connectors" icon={<IconPlug />}>Connectors</NavLink>
        {isAdmin && <NavLink href="/secrets" icon={<IconKey />}>Secrets</NavLink>}
        {isAdmin && <NavLink href="/audit" icon={<IconShield />}>Audit</NavLink>}
      </div>

      <div className="nav-section">
        <div className="nav-label">General</div>
        <NavLink href="/settings" icon={<IconSettings />}>Settings</NavLink>
        <LogoutButton variant="nav" />
      </div>

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
    </aside>
  );
}

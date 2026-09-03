import Link from "next/link";
import { apiGet } from "@/lib/api";
import type { ListResponse, Me, NotificationTarget, UpdateStatus } from "@/lib/types";
import { LogoutButton } from "@/components/LogoutButton";
import { IconKey, IconShield } from "@/components/icons";
import { TargetManager } from "@/components/notify/TargetManager";
import { UpdatesPanel } from "@/components/UpdatesPanel";

function initials(me: Me): string {
  const src = me.name?.trim() || me.email;
  const parts = src.split(/[\s@._-]+/).filter(Boolean);
  if (parts.length >= 2) return (parts[0][0] + parts[1][0]).toUpperCase();
  return src.slice(0, 2).toUpperCase();
}

export default async function SettingsPage() {
  let me: Me | null = null;
  let targets: NotificationTarget[] = [];
  let updates: UpdateStatus | null = null;
  try {
    me = await apiGet<Me>("/me");
  } catch { /* shown below */ }
  try {
    targets = (await apiGet<ListResponse<NotificationTarget>>("/notification-targets")).items;
  } catch { /* non-admin or unavailable */ }
  try {
    updates = await apiGet<UpdateStatus>("/updates");
  } catch { /* control plane unreachable */ }

  const isAdmin = me?.role === "admin" || me?.role === "owner";

  return (
    <>
      <div className="page-head">
        <div>
          <h1>Settings</h1>
          <p className="subtle">Your account and control-plane preferences.</p>
        </div>
      </div>

      {me && (
        <div className="panel" style={{ maxWidth: 560 }}>
          <div className="row">
            <div className="cluster" style={{ flexWrap: "nowrap" }}>
              <span className="avatar" style={{ width: 52, height: 52, fontSize: 18 }}>{initials(me)}</span>
              <div>
                <div style={{ fontWeight: 700, fontSize: 16, color: "var(--text)" }}>{me.name || me.email.split("@")[0]}</div>
                <div className="subtle" style={{ fontSize: 13 }}>{me.email}</div>
              </div>
            </div>
            <span className="status ok" style={{ textTransform: "capitalize" }}>{me.role}</span>
          </div>
          <div style={{ marginTop: 16 }}><LogoutButton /></div>
        </div>
      )}

      <h2>Notifications</h2>
      <p className="subtle" style={{ marginTop: -6, marginBottom: 12 }}>
        Where CronCompose tells you a job failed. Nothing is sent if there are no enabled targets.
      </p>
      {isAdmin
        ? <TargetManager initial={targets} />
        : <div className="panel"><div className="empty">Only admins can view notification targets.</div></div>}

      <h2>Updates</h2>
      <p className="subtle" style={{ marginTop: -6, marginBottom: 12 }}>
        Checks GitHub about once a day. Click Update to build the release from source on each host.
      </p>
      <UpdatesPanel initial={updates} canUpdate={isAdmin} />

      {isAdmin && (
        <>
          <h2>Admin</h2>
          <div className="cards">
            <Link href="/secrets" className="panel">
              <div className="cluster" style={{ flexWrap: "nowrap" }}>
                <span className="mini-icon"><IconKey /></span>
                <div>
                  <div style={{ fontWeight: 700, color: "var(--text)" }}>Secrets</div>
                  <div className="subtle" style={{ fontSize: 12 }}>Manage write-only secret values.</div>
                </div>
              </div>
            </Link>
            <Link href="/audit" className="panel">
              <div className="cluster" style={{ flexWrap: "nowrap" }}>
                <span className="mini-icon"><IconShield /></span>
                <div>
                  <div style={{ fontWeight: 700, color: "var(--text)" }}>Audit log</div>
                  <div className="subtle" style={{ fontSize: 12 }}>Review recent control-plane activity.</div>
                </div>
              </div>
            </Link>
          </div>
        </>
      )}
    </>
  );
}

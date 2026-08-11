import { apiGet } from "@/lib/api";
import { getHealth } from "@/lib/health";
import type { Me } from "@/lib/types";
import { Sidebar } from "./Sidebar";
import { Topbar } from "./Topbar";
import { SetupBanner } from "./SetupBanner";

// Wraps every route. When there's no session (e.g. /login) it renders a bare
// centered shell; otherwise it renders the full sidebar + topbar app frame. Also
// checks control-plane/db health so a setup banner can render regardless of auth
// state (a down db means /me fails too, so this must not depend on being logged in).
export async function Shell({ children }: { children: React.ReactNode }) {
  const [me, health] = await Promise.all([
    apiGet<Me>("/me").catch(() => null),
    getHealth(),
  ]);

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

  return (
    <div className="app-shell">
      <Sidebar me={me} />
      <div className="app-main">
        <Topbar me={me} />
        <main className="content">
          <SetupBanner status={health} />
          {children}
        </main>
      </div>
    </div>
  );
}

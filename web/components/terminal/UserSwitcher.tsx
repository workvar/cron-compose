"use client";

// A "run as" picker backed by the real OS accounts on the target server (GET
// .../terminal/users), instead of a free-text guess. Used both on the terminal setup
// screen and, compact, in the live terminal's top bar so a session can be switched to
// another user without leaving the terminal. Built on the app's SearchableSelect so it
// matches every other picker in the UI, with allowCustom so an account the list
// doesn't carry (or a server whose agent couldn't be reached) can still be typed.
// Unavailable users stay hidden until agent root mode is actually active.
import { useEffect, useState } from "react";
import { SearchableSelect, type SelectOption } from "@/components/SearchableSelect";
import type { Server, SystemUser } from "@/lib/types";
import { visibleTerminalUsers } from "@/lib/terminal-users";

const ROOT = "root";
const AGENT_USER = ""; // empty run_as means "the agent's own user"

type Props = {
  serverId: string;
  /** Current run_as. Empty string means the agent's own user. */
  value: string;
  onChange: (runAs: string) => void;
  /** Compact styling for the live terminal's dark top bar. */
  compact?: boolean;
  id?: string;
};

type LoadState = "loading" | "ready" | "error";

export function UserSwitcher({ serverId, value, onChange, compact, id }: Props) {
  const [users, setUsers] = useState<SystemUser[]>([]);
  const [state, setState] = useState<LoadState>("loading");

  useEffect(() => {
    let cancelled = false;
    setState("loading");
    Promise.all([
      fetch(`/api/servers/${serverId}/terminal/users`).then((res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        return res.json() as Promise<{ users: SystemUser[] }>;
      }),
      fetch(`/api/servers/${serverId}`)
        .then((res) => {
          if (!res.ok) throw new Error(`HTTP ${res.status}`);
          return res.json() as Promise<Server>;
        })
        .catch(() => null),
    ])
      .then(([data, server]) => {
        if (cancelled) return;
        const rootModeActive = !!(server?.agent_root_enabled && server?.agent_euid_root);
        setUsers(sortUsers(visibleTerminalUsers(data.users ?? [], rootModeActive)));
        setState("ready");
      })
      .catch(() => {
        if (!cancelled) setState("error");
      });
    return () => {
      cancelled = true;
    };
  }, [serverId]);

  const options: SelectOption[] = [
    { value: AGENT_USER, label: "Agent's own user" },
    ...users.map((u) => ({ value: u.username, label: userLabel(u) })),
  ];

  const placeholder =
    state === "loading" ? "Loading users…" : state === "error" ? "Agent's own user" : "Select a user…";

  return (
    <SearchableSelect
      id={id}
      className={compact ? "term-user-select" : undefined}
      aria-label="Run as"
      value={value}
      onChange={onChange}
      options={options}
      placeholder={placeholder}
      allowCustom
    />
  );
}

function userLabel(u: SystemUser): string {
  const name = u.username === ROOT ? "root" : u.username;
  return `${name} (uid ${u.uid})`;
}

// Root first (it's the one people go looking for), then alphabetical.
function sortUsers(users: SystemUser[]): SystemUser[] {
  return [...users].sort((a, b) => {
    if (a.username === ROOT) return -1;
    if (b.username === ROOT) return 1;
    return a.username.localeCompare(b.username);
  });
}

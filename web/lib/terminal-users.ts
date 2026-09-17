import type { SystemUser } from "./types";

export function visibleTerminalUsers(users: SystemUser[], rootModeActive: boolean): SystemUser[] {
  if (rootModeActive) return users;
  return users.filter((u) => u.available);
}

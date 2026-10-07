/** Rewrite /opt/… under a non-root account's home (matches control-plane ClonePathForUser). */
export function clonePathForUser(clonePath: string, runAs: string, home: string): string {
  const user = runAs.trim();
  const h = home.trim().replace(/\/+$/, "");
  if (!user || user === "root" || !h || h === "/") return clonePath;
  const path = clonePath.trim() || "/opt/apps";
  if (path === h || path.startsWith(h + "/")) return path;
  if (path.startsWith("/opt/")) return `${h}${path}`;
  const base = path.split("/").filter(Boolean).pop() || "app";
  return `${h}/opt/apps/${base}`;
}

export function userTmpDir(runAs: string, home: string): string {
  const user = runAs.trim();
  const h = home.trim().replace(/\/+$/, "");
  if (!user || user === "root" || !h) return "/tmp";
  return `${h}/tmp`;
}

/** Keep in sync with agent/control-plane QualifyProcessName. */
export function slugProcessName(s: string): string {
  return (s || "")
    .trim()
    .replace(/[^a-zA-Z0-9._-]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .replace(/--+/g, "-")
    .toLowerCase();
}

export function qualifyProcessName(project: string, app: string): string {
  const p = slugProcessName(project);
  const a = slugProcessName(app);
  if (!p) return a || "app";
  if (!a || a === p) return p;
  const prefix = `${p}-`;
  if (a.startsWith(prefix)) return a;
  return prefix + a;
}

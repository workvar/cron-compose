export type GitRepoLike = {
  id: string;
  full_name: string;
  description?: string;
};

export type GitRepoOwnerOption = {
  owner: string;
  personal: boolean;
};

/** Owner/namespace of owner/repo or group/sub/project (everything before the last slash). */
export function gitRepoOwner(fullName: string): string {
  const name = fullName.trim();
  if (!name) return "";
  const slash = name.lastIndexOf("/");
  if (slash <= 0) return name;
  return name.slice(0, slash);
}

export function listGitRepoOwners(repos: GitRepoLike[], personalLogin?: string): GitRepoOwnerOption[] {
  const seen = new Set<string>();
  const out: GitRepoOwnerOption[] = [];
  const personal = (personalLogin || "").trim().toLowerCase();
  for (const r of repos) {
    const owner = gitRepoOwner(r.full_name);
    if (!owner || seen.has(owner)) continue;
    seen.add(owner);
    out.push({ owner, personal: personal !== "" && owner.toLowerCase() === personal });
  }
  out.sort((a, b) => {
    if (a.personal !== b.personal) return a.personal ? -1 : 1;
    return a.owner.localeCompare(b.owner);
  });
  return out;
}

export function toggleOwnerFilter(selected: string[], owner: string): string[] {
  if (selected.includes(owner)) return selected.filter((o) => o !== owner);
  return [...selected, owner];
}

export function filterGitRepos<T extends GitRepoLike>(
  repos: T[],
  opts: { query: string; owners: string[] },
): T[] {
  const q = opts.query.trim().toLowerCase();
  const owners = new Set(opts.owners);
  return repos.filter((r) => {
    if (owners.size > 0 && !owners.has(gitRepoOwner(r.full_name))) return false;
    if (!q) return true;
    const hay = `${r.full_name} ${r.description || ""}`.toLowerCase();
    return hay.includes(q);
  });
}

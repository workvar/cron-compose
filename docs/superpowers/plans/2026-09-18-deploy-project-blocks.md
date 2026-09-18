# Deploy Project Blocks + Hybrid Folder Browse Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let Import git configure one or more project blocks with browseable (lazy + optional full-tree) roots, each with its own language/install/port/process manager, submitted as existing `apps[]`.

**Architecture:** New `GET /git/dirs` on the control plane lists directories via GitHub/GitLab (shallow by default, recursive capped at 2000). The Import wizard Build step becomes a list of project-block panels with a folder picker; Runtime keeps only shared server/branch/clone path + per-app env.

**Tech Stack:** Go 1.25, Fiber v3, existing `deploys.GitAPI`, Next.js React Import wizard, node assert tests under `web/lib/`.

**Spec:** `docs/superpowers/specs/2026-09-18-deploy-project-blocks-design.md`

## Global Constraints

- No DB migrations; reuse `apps[]` on `POST /deploys`.
- Repo root selectable as `"."` in the UI (not returned as a child of itself from the API).
- Non-recursive dirs = immediate children only; recursive max **2000** with `truncated: true`.
- Paths repo-relative, `/`-separated, no leading `./`.
- Ignore `.git` when listing (`.github` may remain).
- Duplicate roots: block Continue; colliding names: auto-suffix (`web-2`).
- Top-level create fields (`language`, `install_script`, `root_directory`, `port`, `process_manager`) come from **first** block.
- TDD: failing test first; frequent commits.
- Do not expand scope to editing existing project apps on the detail page.

## File map

| Path | Responsibility |
|------|----------------|
| `control-plane/internal/deploys/model.go` | `DirEntry`, `DirList` response types |
| `control-plane/internal/deploys/dirs.go` | Path normalize + list/filter helpers + `maxDirEntries` |
| `control-plane/internal/deploys/dirs_test.go` | Pure helper tests |
| `control-plane/internal/deploys/provider.go` | `ListDirs` GitHub/GitLab |
| `control-plane/internal/deploys/provider_dirs_test.go` | httptest ListDirs tests |
| `control-plane/internal/deploys/handler.go` | `listDirs` handler |
| `control-plane/internal/deploys/routes.go` | Register `GET /git/dirs` |
| `docs/api.md` | Document `/git/dirs` |
| `web/lib/project-blocks.ts` | Block helpers → `DeployApp[]` |
| `web/lib/project-blocks.test.ts` | Assert tests |
| `web/components/deploys/RepoFolderPicker.tsx` | Lazy + recursive folder UI |
| `web/components/deploys/ProjectBlockCard.tsx` | One project block panel |
| `web/app/deploys/new/page.tsx` | Wizard Build/Runtime/Review/submit |
| `web/lib/types.ts` | `GitDirEntry`, `GitDirList` if needed |
| `web/tsconfig.json` | Exclude new `*.test.ts` |

---

### Task 1: Dir list pure helpers (Go)

**Files:**
- Create: `control-plane/internal/deploys/dirs.go`
- Create: `control-plane/internal/deploys/dirs_test.go`
- Modify: `control-plane/internal/deploys/model.go` (add types)

**Interfaces:**
- Produces:
  - `const MaxDirEntries = 2000`
  - `type DirEntry struct { Name string \`json:"name"\`; Path string \`json:"path"\` }`
  - `type DirList struct { Path string \`json:"path"\`; Recursive bool \`json:"recursive"\`; Truncated bool \`json:"truncated"\`; Items []DirEntry \`json:"items"\` }`
  - `func NormalizeRepoPath(p string) string` — trim, strip leading `./`, collapse `//`, empty → `""` (API path for root)
  - `func DirBaseName(path string) string` — last segment; `""` or `"."` → `"."`
  - `func ShouldSkipDirName(name string) bool` — true for `.git`
  - `func FilterAndCapDirs(paths []string, parent string, recursive bool, cap int) (items []DirEntry, truncated bool)`

- [ ] **Step 1: Write failing tests**

```go
func TestNormalizeRepoPath(t *testing.T) {
	cases := map[string]string{
		"": ".", // wait: API uses "" for root query; Normalize for storage/UI of selected root uses "."
	}
	// Spec: NormalizeRepoPath for *query* parent: "" stays ""; "./apps/" → "apps"; "apps//web" → "apps/web"
	if got := NormalizeRepoPath(" ./apps/web/ "); got != "apps/web" {
		t.Fatalf("got %q", got)
	}
	if got := NormalizeRepoPath(""); got != "" {
		t.Fatalf("empty query path got %q", got)
	}
	if got := NormalizeRepoPath("."); got != "" {
		t.Fatalf(". as query parent should be empty, got %q", got)
	}
}

func TestFilterAndCapDirsShallow(t *testing.T) {
	paths := []string{"apps", "apps/web", "backend", ".git", "README.md"}
	// For shallow under "", only immediate dir names that appear as path with no slash OR we pass pre-filtered child dirs
	items, trunc := FilterAndCapDirs([]string{"apps", "backend", ".git"}, "", false, 2000)
	if trunc || len(items) != 2 {
		t.Fatalf("items=%v trunc=%v", items, trunc)
	}
}

func TestFilterAndCapDirsRecursivePrefixAndCap(t *testing.T) {
	var paths []string
	for i := 0; i < 5; i++ {
		paths = append(paths, fmt.Sprintf("apps/p%d", i))
	}
	items, trunc := FilterAndCapDirs(paths, "apps", true, 3)
	if !trunc || len(items) != 3 {
		t.Fatalf("len=%d trunc=%v", len(items), trunc)
	}
}
```

- [ ] **Step 2: Run tests — expect FAIL**

Run: `cd control-plane && go test ./internal/deploys -run 'TestNormalizeRepoPath|TestFilterAndCapDirs' -count=1`

- [ ] **Step 3: Implement `dirs.go` + types in `model.go`**

```go
const MaxDirEntries = 2000

func NormalizeRepoPath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimPrefix(p, "./")
	p = strings.Trim(p, "/")
	for strings.Contains(p, "//") {
		p = strings.ReplaceAll(p, "//", "/")
	}
	if p == "." {
		return ""
	}
	return p
}

func ShouldSkipDirName(name string) bool {
	return name == ".git"
}

// FilterAndCapDirs: paths are full repo-relative directory paths already known to be dirs.
// Shallow: keep only entries whose parent equals parent (NormalizeRepoPath).
// Recursive: keep entries equal to parent or under parent+"/"; sort; cap.
```

- [ ] **Step 4: Run tests — expect PASS**

Run: `cd control-plane && go test ./internal/deploys -run 'TestNormalizeRepoPath|TestFilterAndCapDirs|TestDirBaseName|TestShouldSkipDirName' -count=1`

- [ ] **Step 5: Commit**

```bash
git add control-plane/internal/deploys/dirs.go control-plane/internal/deploys/dirs_test.go control-plane/internal/deploys/model.go
git commit -m "feat(deploys): add dir list path helpers"
```

---

### Task 2: GitAPI.ListDirs (GitHub + GitLab)

**Files:**
- Modify: `control-plane/internal/deploys/provider.go`
- Create: `control-plane/internal/deploys/provider_dirs_test.go`

**Interfaces:**
- Consumes: helpers from Task 1; `g.githubBase`, `g.gitlabAPI()`, `g.getJSON`
- Produces: `func (g *GitAPI) ListDirs(ctx context.Context, provider, token, fullName, branch, path string, recursive bool) (DirList, error)`

- [ ] **Step 1: Write failing httptest tests**

```go
func TestListDirsGitHubShallow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/contents") {
			_ = json.NewEncoder(w).Encode([]map[string]string{
				{"name": "web", "path": "apps/web", "type": "dir"},
				{"name": "readme.md", "path": "apps/readme.md", "type": "file"},
				{"name": ".git", "path": "apps/.git", "type": "dir"},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	g := NewGitAPI("")
	g.githubBase = srv.URL
	g.http = srv.Client()
	out, err := g.ListDirs(context.Background(), "github", "tok", "acme/r", "main", "apps", false)
	if err != nil {
		t.Fatal(err)
	}
	if out.Recursive || out.Truncated || len(out.Items) != 1 || out.Items[0].Path != "apps/web" {
		t.Fatalf("%+v", out)
	}
}

func TestListDirsGitHubRecursiveCap(t *testing.T) {
	// Serve git/trees?recursive=1 with >cap tree entries of type tree; assert truncated
}

func TestListDirsGitLabShallow(t *testing.T) {
	// Serve /api/v4/projects/.../repository/tree without recursive; type=tree only
}
```

- [ ] **Step 2: Run — expect FAIL (ListDirs undefined)**

Run: `cd control-plane && go test ./internal/deploys -run TestListDirs -count=1`

- [ ] **Step 3: Implement `ListDirs`**

- If `branch == ""`, fetch repo meta for default branch (reuse patterns from `fetchGitHub` / `fetchGitLab` meta calls).
- GitHub shallow: `GET {githubAPI}/repos/{fullName}/contents/{path}?ref=` — if path empty, contents at repo root. Filter `type=="dir"`, skip `.git`.
- GitHub recursive: `GET .../git/trees/{branch}?recursive=1`, filter `type==tree`, apply `FilterAndCapDirs` with parent=path.
- GitLab shallow: `.../repository/tree?path=&ref=&per_page=100`, `type==tree`.
- GitLab recursive: same with `recursive=true`, paginate `Link` or page=` until cap (simple page loop `page=1..` while len < cap).
- Return `DirList{Path: NormalizeRepoPath(path), Recursive: recursive, ...}`.

- [ ] **Step 4: Run — expect PASS**

Run: `cd control-plane && go test ./internal/deploys -run TestListDirs -count=1`

- [ ] **Step 5: Commit**

```bash
git add control-plane/internal/deploys/provider.go control-plane/internal/deploys/provider_dirs_test.go
git commit -m "feat(deploys): ListDirs for GitHub and GitLab"
```

---

### Task 3: HTTP handler + route + API docs

**Files:**
- Modify: `control-plane/internal/deploys/handler.go`
- Modify: `control-plane/internal/deploys/routes.go`
- Modify: `docs/api.md`

**Interfaces:**
- Consumes: `h.git.ListDirs`, `h.conns.Token`
- Produces: `GET /git/dirs` → JSON `DirList`

- [ ] **Step 1: Add handler (mirror `listRepos` / `inspect` error codes)**

```go
func (h *handler) listDirs(c fiber.Ctx) error {
	provider := c.Query("provider", "github")
	repo := c.Query("repo")
	branch := c.Query("branch")
	path := c.Query("path")
	recursive := c.Query("recursive") == "1" || strings.EqualFold(c.Query("recursive"), "true")
	if repo == "" {
		return jsonError(c, fiber.StatusBadRequest, "missing_repo", errors.New("repo is required"))
	}
	token, err := h.conns.Token(c.Context(), auth.CurrentUserID(c), provider)
	// 409 not_connected, 500 token_failed — same as listRepos
	out, err := h.git.ListDirs(c.Context(), provider, token, repo, branch, path, recursive)
	// 502 git_api
	return c.JSON(out)
}
```

- [ ] **Step 2: Register route next to inspect**

```go
r.Get("/git/dirs", h.listDirs)
```

- [ ] **Step 3: Document in `docs/api.md`**

Add row: `GET | /git/dirs?provider=&repo=&branch=&path=&recursive= | viewer | List directories (shallow or recursive, capped).`

- [ ] **Step 4: Compile**

Run: `cd control-plane && go test ./internal/deploys -count=1 && go build ./...`

- [ ] **Step 5: Commit**

```bash
git add control-plane/internal/deploys/handler.go control-plane/internal/deploys/routes.go docs/api.md
git commit -m "feat(deploys): expose GET /git/dirs"
```

---

### Task 4: Web project-block helpers

**Files:**
- Create: `web/lib/project-blocks.ts`
- Create: `web/lib/project-blocks.test.ts`
- Modify: `web/tsconfig.json` (exclude test)
- Modify: `web/lib/types.ts` — add:

```ts
export type GitDirEntry = { name: string; path: string };
export type GitDirList = {
  path: string;
  recursive: boolean;
  truncated: boolean;
  items: GitDirEntry[];
};
```

**Interfaces:**
- Produces:
  - `export type ProjectBlock = { id: string; name: string; root: string; language: string; install: string; port: string; processManager: string }`
  - `export function newBlockId(): string`
  - `export function nameFromRoot(root: string, repoFullName: string): string`
  - `export function normalizeBlockRoot(root: string): string` — `""` → `"."`
  - `export function seedBlockFromInspect(inspect: DeployInspect, repoFullName: string): ProjectBlock`
  - `export function emptyBlock(): ProjectBlock` — `root: ""` until picked
  - `export function ensureUniqueBlockNames(blocks: ProjectBlock[]): ProjectBlock[]`
  - `export function hasDuplicateRoots(blocks: ProjectBlock[]): boolean`
  - `export function blocksReady(blocks: ProjectBlock[]): boolean` — every root non-empty after normalize
  - `export function blocksToDeployApps(blocks: ProjectBlock[], appEnv: Record<string, DeployEnvVar[]>): DeployApp[]`

- [ ] **Step 1: Write failing test file**

```ts
import assert from "node:assert/strict";
import {
  blocksReady, blocksToDeployApps, ensureUniqueBlockNames, hasDuplicateRoots,
  nameFromRoot, normalizeBlockRoot, seedBlockFromInspect,
} from "./project-blocks.ts";

assert.equal(normalizeBlockRoot(""), ".");
assert.equal(normalizeBlockRoot("apps/web"), "apps/web");
assert.equal(nameFromRoot(".", "acme/widgets"), "widgets");
assert.equal(nameFromRoot("apps/web", "acme/widgets"), "web");

const dup = ensureUniqueBlockNames([
  { id: "1", name: "web", root: "a", language: "", install: "", port: "", processManager: "none" },
  { id: "2", name: "web", root: "b", language: "", install: "", port: "", processManager: "none" },
]);
assert.equal(dup[1].name, "web-2");

assert.equal(hasDuplicateRoots([
  { id: "1", name: "a", root: "apps/web", language: "", install: "", port: "", processManager: "none" },
  { id: "2", name: "b", root: "apps/web", language: "", install: "", port: "", processManager: "none" },
]), true);

assert.equal(blocksReady([{ id: "1", name: "x", root: "", language: "", install: "", port: "", processManager: "none" }]), false);
```

- [ ] **Step 2: Run — expect FAIL**

Run: `cd web && node --experimental-strip-types lib/project-blocks.test.ts`

- [ ] **Step 3: Implement `project-blocks.ts` + types**

- [ ] **Step 4: Run — expect PASS**; exclude test in tsconfig

- [ ] **Step 5: Commit**

```bash
git add web/lib/project-blocks.ts web/lib/project-blocks.test.ts web/lib/types.ts web/tsconfig.json
git commit -m "feat(web): project block helpers for import wizard"
```

---

### Task 5: RepoFolderPicker component

**Files:**
- Create: `web/components/deploys/RepoFolderPicker.tsx`

**Interfaces:**
- Consumes: `GET /api/git/dirs?...`, workspace chips from props
- Produces: React component

```tsx
export function RepoFolderPicker({
  provider, repo, branch, value, workspaces, onChange, onClose,
}: {
  provider: string;
  repo: string;
  branch: string;
  value: string; // current root "." or path
  workspaces: string[];
  onChange: (root: string) => void;
  onClose: () => void;
})
```

Behavior:
- State: `browsePath` (API parent, `""` for root), `items`, `loading`, `error`, `mode: "lazy" | "tree"`, `treeQuery`, `manual`.
- On open / browsePath change (lazy): fetch `/api/git/dirs?provider&repo&branch&path=browsePath`.
- Breadcrumb from browsePath segments; **Up**; list dirs as buttons that set browsePath; **Use this folder** commits `browsePath === "" ? "." : browsePath`.
- Workspace chips call `onChange(w)` + `onClose`.
- **Load full tree** → fetch `recursive=1`, show searchable filtered list; selecting a path commits.
- Manual text input + Apply for escape hatch.
- Show truncated warning when `truncated`.

- [ ] **Step 1: Implement component** (no separate unit test; covered by manual + helpers)

- [ ] **Step 2: Typecheck**

Run: `cd web && npx tsc --noEmit`

- [ ] **Step 3: Commit**

```bash
git add web/components/deploys/RepoFolderPicker.tsx
git commit -m "feat(web): RepoFolderPicker with lazy and full tree"
```

---

### Task 6: ProjectBlockCard + wizard Build step

**Files:**
- Create: `web/components/deploys/ProjectBlockCard.tsx`
- Modify: `web/app/deploys/new/page.tsx`

**Interfaces:**
- Consumes: `ProjectBlock`, `RepoFolderPicker`, helpers from Task 4
- `ProjectBlockCard` props: `block`, `workspaces`, `provider`, `repo`, `branch`, `canRemove`, `onChange`, `onRemove`

Each card fields: name, root display + Change (opens picker), language, install textarea, port, process manager select (same options as current Runtime).

Wizard draft: replace `language/root/install/port/processManager/selectedApps` with `blocks: ProjectBlock[]`. On `inspectRepo` success, `blocks: [seedBlockFromInspect(...)]`.

Build step UI: map blocks → `ProjectBlockCard`; **+ Add project** → `emptyBlock()`; Continue disabled when `!blocksReady(blocks) || hasDuplicateRoots(blocks)` (show error text for duplicates).

- [ ] **Step 1: Implement card + rewire Build step**

- [ ] **Step 2: Typecheck**

Run: `cd web && npx tsc --noEmit`

- [ ] **Step 3: Commit**

```bash
git add web/components/deploys/ProjectBlockCard.tsx web/app/deploys/new/page.tsx
git commit -m "feat(web): project blocks on Import Build step"
```

---

### Task 7: Runtime, Review, submit

**Files:**
- Modify: `web/app/deploys/new/page.tsx`

**Interfaces:**
- `apps = useMemo(() => blocksToDeployApps(ensureUniqueBlockNames(draft.blocks), draft.appEnv), ...)`
- Runtime: server (prefer `SearchableSelect`), branch, clonePath, `AppEnvEditor` only — remove shared port/PM.
- Review: list each block’s name/root/language/install/port/PM.
- `submit()`: `apps` from helpers; top-level from `apps[0]` (or first block):

```ts
const apps = blocksToDeployApps(ensureUniqueBlockNames(draft.blocks), draft.appEnv);
const first = apps[0];
const body = {
  // ...repo fields...
  language: first?.language || "",
  install_script: first?.install || "",
  root_directory: first?.root || ".",
  port: first?.port || 0,
  process_manager: first?.process_manager || "none",
  apps,
};
```

- [ ] **Step 1: Update Runtime / Review / submit**

- [ ] **Step 2: Typecheck + helper tests**

Run: `cd web && node --experimental-strip-types lib/project-blocks.test.ts && npx tsc --noEmit`

- [ ] **Step 3: Commit**

```bash
git add web/app/deploys/new/page.tsx
git commit -m "feat(web): wire project blocks through Runtime Review submit"
```

---

### Task 8: Smoke verification

- [ ] **Step 1: Full Go package test**

Run: `cd control-plane && go test ./internal/deploys -count=1`

- [ ] **Step 2: Manual checklist (document results in commit message or leave for human)**
  - Import a monorepo; open folder picker; drill into a subdir; Use this folder
  - Load full tree; search; select
  - Add second block; configure different root; Deploy; project shows two apps

- [ ] **Step 3: Final commit if any doc/fixups**

```bash
git status
# commit only if needed
```

---

## Spec coverage check

| Spec item | Task |
|-----------|------|
| `GET /git/dirs` shallow + recursive + cap | 1–3 |
| Ignore `.git` | 1–2 |
| ProjectBlock model + seed from inspect | 4, 6 |
| Folder picker lazy + full tree + chips + manual | 5 |
| Multi blocks add/remove | 6 |
| Runtime shared only + env | 7 |
| Review lists blocks | 7 |
| Submit `apps[]` + first-block top-level | 7 |
| Duplicate roots / unique names | 4, 6 |
| docs/api.md | 3 |
| No migrations | — |

## Placeholder scan

None intentional; all steps include concrete signatures and commands.

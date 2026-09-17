# Agent Root Access + Passkeys Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add WebAuthn passkeys (passwordless login + step-up) and a per-server “Agent root access” switch that elevates/demotes the agent via a narrow sudo helper, while the terminal hides other OS users until root mode is active.

**Architecture:** Control plane stores WebAuthn credentials and per-server root flags; step-up assertions authorize `POST /servers/:id/agent-root`; the agent runs `sudo -n agent-privctl elevate|demote` and reports `euid_root` on Hello/Heartbeat. Terminal picker filters on `agent_root_enabled && agent_euid_root`.

**Tech Stack:** Go 1.25, Fiber v3, pgx, `github.com/go-webauthn/webauthn`, existing agent gRPC proto, Next.js React login/settings/server UI, bash `agent_sudoers.sh` + new `agent-privctl` helper.

**Spec:** `docs/superpowers/specs/2026-09-18-agent-root-passkeys-design.md`

## Global Constraints

- Passkey required before root toggle; password is never a substitute for step-up.
- Admin/owner only for root toggle; any signed-in user manages their own passkeys.
- Sudoers grants are only `agent-privctl elevate` and `agent-privctl demote` (plus existing connector/Ports paths) — never `NOPASSWD: ALL`.
- Terminal hides unavailable users when root mode inactive (do not show `needs root agent` tease).
- Disable must attempt automatic demotion; failed demotion surfaces an error.
- RP ID / origins from `PUBLIC_BASE_URL` / `PUBLIC_HTTP_URL`.
- Follow TDD: failing test first for each behavior; frequent commits.
- Regenerate proto with `make proto` (or checked-in protoc flow) after proto edits.

## File map

| Path | Responsibility |
|------|----------------|
| `migrations/0017_webauthn.sql` | Credentials + challenges tables |
| `migrations/0018_server_agent_root.sql` | Server root columns |
| `control-plane/internal/auth/webauthn_*.go` | WebAuthn config, store, handlers |
| `control-plane/internal/servers/*` | Root flag persistence + HTTP handler |
| `control-plane/internal/agentgw/*` | Send `AgentRootCommand`; update euid from Hello |
| `proto/agent.proto` | `AgentRootCommand`; Hello/Heartbeat `euid_root`, `service_user` |
| `agent/internal/runtime/root.go` | Handle elevate/demote command |
| `agent/cmd/agent-privctl/` or `install/bin/agent-privctl` | Root helper |
| `install/lib/agent_sudoers.sh` | Add privctl sudoers lines |
| `web/app/login/page.tsx` | Passkey login button |
| `web/components/Passkey*.tsx` | Enroll / list / assert helpers |
| `web/app/settings/*` | Security section |
| `web/components/AgentRootToggle.tsx` | Server-page switch + step-up |
| `web/components/terminal/UserSwitcher.tsx` | Hide unavailable users |
| `docs/terminal.md`, `docs/security.md`, `docs/operations.md` | Operator docs |

---

### Task 1: WebAuthn migration + credential store

**Files:**
- Create: `migrations/0017_webauthn.sql`
- Create: `control-plane/internal/auth/webauthn_store.go`
- Create: `control-plane/internal/auth/webauthn_store_test.go`

**Interfaces:**
- Produces: `type WebAuthnStore struct`; methods:
  - `InsertCredential(ctx, Cred) error`
  - `ListByUser(ctx, userID string) ([]Cred, error)`
  - `GetByCredentialID(ctx, id []byte) (*Cred, error)`
  - `Delete(ctx, userID, credPK string) error`
  - `UpdateSignCount(ctx, credPK string, n uint32) error`
  - `PutChallenge(ctx, Challenge) error`
  - `TakeChallenge(ctx, id string) (*Challenge, error)` — delete-on-read

- [ ] **Step 1: Write failing store test**

```go
func TestWebAuthnStoreInsertAndList(t *testing.T) {
  // skip if no INTEGRATION_DB_URL; or use pgxmock if the package already prefers unit fakes.
  // Prefer: table-driven test against real store with a test helper if one exists in auth.
  s := newTestWebAuthnStore(t)
  c := Cred{ID: "c1", UserID: "u1", CredentialID: []byte{1, 2, 3}, PublicKey: []byte{9}, Name: "Laptop"}
  if err := s.InsertCredential(context.Background(), c); err != nil {
    t.Fatal(err)
  }
  list, err := s.ListByUser(context.Background(), "u1")
  if err != nil || len(list) != 1 || list[0].Name != "Laptop" {
    t.Fatalf("got %+v err=%v", list, err)
  }
}
```

- [ ] **Step 2: Run test — expect fail (type/store missing)**

Run: `cd control-plane && go test ./internal/auth -count=1 -run TestWebAuthnStoreInsertAndList`

- [ ] **Step 3: Add migration `0017_webauthn.sql`** exactly as in the spec (tables `webauthn_credentials`, `webauthn_challenges`), then implement `WebAuthnStore` with the methods above.

- [ ] **Step 4: Re-run test — pass**

- [ ] **Step 5: Commit**

```bash
git add migrations/0017_webauthn.sql control-plane/internal/auth/webauthn_store.go control-plane/internal/auth/webauthn_store_test.go
git commit -m "feat(auth): add WebAuthn credential store"
```

---

### Task 2: WebAuthn config + begin/finish ceremonies

**Files:**
- Create: `control-plane/internal/auth/webauthn.go` (RP from PublicBaseURL)
- Create: `control-plane/internal/auth/webauthn_handler.go`
- Create: `control-plane/internal/auth/webauthn_test.go`
- Modify: `control-plane/internal/auth/handler.go` (`config` JSON adds `passkey_login`)
- Modify: `control-plane/internal/api/router.go` (register routes)
- Modify: `control-plane/go.mod` — add `github.com/go-webauthn/webauthn`

**Interfaces:**
- Consumes: `WebAuthnStore`, `auth.Store` (users), session `SignSession`
- Produces: HTTP handlers for register/login begin/finish; `HasPasskey(ctx, userID) (bool, error)`

- [ ] **Step 1: Write failing test for RP ID parsing**

```go
func TestRelyingPartyFromPublicURL(t *testing.T) {
  rp, err := relyingParty("https://cron.example.com")
  if err != nil {
    t.Fatal(err)
  }
  if rp.ID != "cron.example.com" {
    t.Fatalf("id=%q", rp.ID)
  }
}
```

- [ ] **Step 2: Run — fail**

Run: `cd control-plane && go test ./internal/auth -count=1 -run TestRelyingPartyFromPublicURL`

- [ ] **Step 3: Implement `relyingParty`, wire `go-webauthn`, handlers**

Routes (match spec):
- `POST /auth/passkey/login/begin` + `/finish` on public router
- `POST /auth/passkey/register/begin` + `/finish`, `GET /auth/passkeys`, `DELETE /auth/passkeys/:id` on authenticated group
- Extend `GET /auth/config` with `"passkey_login": true` when RP can be derived

Login finish must call `SignSession` and set `cc_session` like password login.

- [ ] **Step 4: Unit-test challenge put/take expiry (reject expired)**

- [ ] **Step 5: Commit**

```bash
git add control-plane/go.mod control-plane/go.sum control-plane/internal/auth/webauthn*.go control-plane/internal/api/router.go control-plane/internal/auth/handler.go
git commit -m "feat(auth): WebAuthn register and login ceremonies"
```

---

### Task 3: Passkey UI — Settings Security + login button

**Files:**
- Create: `web/components/PasskeyManager.tsx`
- Create: `web/lib/webauthn.ts` (base64url helpers + `navigator.credentials` wrappers)
- Modify: `web/app/settings/page.tsx` — Security section
- Modify: `web/app/login/page.tsx` — “Sign in with passkey”
- Modify: `web/lib/types.ts` if needed for auth config

**Interfaces:**
- Consumes: `/api/auth/passkey/*`, `/api/auth/passkeys`, `/api/auth/config`
- Produces: enroll/list/remove UI; passwordless login entry point

- [ ] **Step 1: Add `web/lib/webauthn.ts` with `bufferToBase64url` / `base64urlToBuffer` and a failing vitest/node assert file if the repo uses node assert tests (mirror `update-progress.test.ts` style)**

```ts
import assert from "node:assert/strict";
import { bufferToBase64url, base64urlToBuffer } from "./webauthn.ts";
const buf = new Uint8Array([1, 2, 255]);
assert.deepEqual(new Uint8Array(base64urlToBuffer(bufferToBase64url(buf))), buf);
```

- [ ] **Step 2: Run** `cd web && npx tsx lib/webauthn.test.ts` — implement until pass

- [ ] **Step 3: Build `PasskeyManager`** — list keys, “Add passkey” runs register begin→create→finish; delete with confirm

- [ ] **Step 4: Login page** — if `authCfg.passkey_login`, show button that runs login begin→get→finish then `router.push(next)`

- [ ] **Step 5: Mount Security section on Settings** under account panel

- [ ] **Step 6: Commit**

```bash
git add web/lib/webauthn.ts web/lib/webauthn.test.ts web/components/PasskeyManager.tsx web/app/settings/page.tsx web/app/login/page.tsx web/lib/types.ts web/tsconfig.json
git commit -m "feat(web): passkey login and Settings Security"
```

---

### Task 4: Server root columns + step-up API (flag only)

**Files:**
- Create: `migrations/0018_server_agent_root.sql`
- Modify: `control-plane/internal/servers/model.go`, `store.go`
- Create: `control-plane/internal/servers/root_handler.go`
- Create: `control-plane/internal/servers/root_handler_test.go`
- Modify: `control-plane/internal/servers/routes.go`
- Modify: `control-plane/internal/auth/webauthn_handler.go` or shared `VerifyStepUp(ctx, userID, assertion) error`

**Interfaces:**
- Produces: `Server.AgentRootEnabled`, `AgentEuidRoot`, `AgentRootChangedAt`, `AgentRootChangedBy`, `AgentServiceUser`
- Produces: `POST /servers/:id/agent-root` body `{enabled, credential}` → 200 updates flag; 403 no passkey / bad assertion / wrong role

- [ ] **Step 1: Failing test — reject toggle when user has no passkeys**

```go
func TestAgentRootRequiresPasskey(t *testing.T) {
  // handler with empty WebAuthnStore for user
  // POST enabled=true → 403 code passkey_required
}
```

- [ ] **Step 2: Run — fail**

- [ ] **Step 3: Migration + store columns + handler**

```sql
alter table servers
  add column if not exists agent_root_enabled boolean not null default false,
  add column if not exists agent_euid_root boolean not null default false,
  add column if not exists agent_root_changed_at timestamptz,
  add column if not exists agent_root_changed_by text references users(id),
  add column if not exists agent_service_user text;
```

Update `selectColumns` / `scan` / JSON on `Server`. Register route with `RequireRole("admin")` (owner included if existing helper treats owner as admin — match delete-server gating).

On success: audit write + `SetAgentRootEnabled` — **do not** send agent command yet (Task 6).

- [ ] **Step 4: Tests pass for require-passkey, bad assertion, happy path with mocked verifier**

- [ ] **Step 5: Commit**

```bash
git add migrations/0018_server_agent_root.sql control-plane/internal/servers/ control-plane/internal/auth/
git commit -m "feat(servers): agent-root toggle API with passkey step-up"
```

---

### Task 5: Server UI toggle + terminal picker hide

**Files:**
- Create: `web/components/AgentRootToggle.tsx`
- Modify: `web/app/servers/[id]/page.tsx`
- Modify: `web/components/terminal/UserSwitcher.tsx`
- Modify: `web/app/servers/[id]/terminal/page.tsx` (copy)
- Modify: `web/lib/types.ts` (`Server` fields)

**Interfaces:**
- Consumes: `POST /api/servers/:id/agent-root`, `GET /api/auth/passkeys`, server JSON fields
- Produces: switch UI; picker filters `users.filter(u => u.available || rootModeActive)` where `rootModeActive = server.agent_root_enabled && server.agent_euid_root`

- [ ] **Step 1: Unit-test filter helper**

```ts
// web/lib/terminal-users.ts
export function visibleTerminalUsers(users: SystemUser[], rootModeActive: boolean): SystemUser[] {
  if (rootModeActive) return users;
  return users.filter((u) => u.available);
}
```

```ts
import assert from "node:assert/strict";
import { visibleTerminalUsers } from "./terminal-users.ts";
const users = [
  { username: "a", uid: 1000, home: "", shell: "/bin/bash", available: true },
  { username: "b", uid: 1001, home: "", shell: "/bin/bash", available: false },
];
assert.equal(visibleTerminalUsers(users, false).length, 1);
assert.equal(visibleTerminalUsers(users, true).length, 2);
```

- [ ] **Step 2: Run test — implement until pass**

- [ ] **Step 3: `UserSwitcher` uses `visibleTerminalUsers`; drop `needs root agent` label path for hidden users**

- [ ] **Step 4: `AgentRootToggle`** — if no passkeys, disabled + link to Settings; else confirm → WebAuthn get (step_up challenge from begin endpoint or embedded in toggle flow) → POST agent-root → `router.refresh()`

Add `POST /auth/passkey/step-up/begin` if cleaner than stuffing challenge into agent-root (allowed by spec). Prefer: `POST /auth/passkey/step-up/begin` then finish assertion inside `POST /servers/:id/agent-root`.

- [ ] **Step 5: Mount toggle on server detail for admin/owner; update terminal page subtle copy**

- [ ] **Step 6: Commit**

```bash
git add web/
git commit -m "feat(web): agent root toggle and hide terminal users"
```

---

### Task 6: Proto AgentRootCommand + euid reporting

**Files:**
- Modify: `proto/agent.proto`
- Regenerate: `proto/agent/v1/*.pb.go`
- Modify: `agent/internal/runtime/runtime.go` (Hello fields)
- Modify: `control-plane/internal/agentgw/stream.go` (`onHello` / heartbeat)
- Modify: `control-plane/internal/agentgw/server.go` — `SendAgentRootCommand`
- Modify: Task 4 handler to call `SendAgentRootCommand` after DB update

**Interfaces:**
- Produces:
```protobuf
message AgentRootCommand { bool enabled = 1; }
// Hello additions:
bool euid_root = 6;
string service_user = 7;
```
- `Gateway.SendAgentRootCommand(serverID string, enabled bool) error`

- [ ] **Step 1: Edit proto; run `make proto` / protoc; `make proto-check`**

- [ ] **Step 2: Agent sets `EuidRoot: os.Geteuid()==0` and `ServiceUser` (from env `USER` or marker) on Hello**

- [ ] **Step 3: Control plane persists euid + service_user on Hello; handler pushes command after toggle**

- [ ] **Step 4: Test `SendAgentRootCommand` registry send (table test with fake registry if pattern exists)**

- [ ] **Step 5: Commit**

```bash
git add proto/ agent/ control-plane/
git commit -m "feat(agent): report euid_root and accept AgentRootCommand"
```

---

### Task 7: `agent-privctl` helper + sudoers + agent handler

**Files:**
- Create: `agent/cmd/agent-privctl/main.go` (or `install/agent-privctl` built into `agent/bin/`)
- Create: `agent/cmd/agent-privctl/main_test.go`
- Create: `agent/internal/runtime/root.go`
- Modify: `agent/internal/runtime/sync.go` — route `AgentRootCommand`
- Modify: `install/lib/agent_sudoers.sh`
- Modify: `install/README.md`, `docs/operations.md`, `docs/security.md`, `docs/terminal.md`

**Interfaces:**
- Helper CLI: `agent-privctl elevate|demote` exit 0 on success; stderr message on failure
- Marker files under agent data/runtime dir: `agent-supervisor`, `agent-service-user`
- Agent: `sudo -n /usr/libexec/croncompose/agent-privctl elevate|demote` then expect restart

- [ ] **Step 1: Failing test for helper argument parsing**

```go
func TestPrivctlArgs(t *testing.T) {
  if err := validateArgs([]string{"elevate"}); err != nil {
    t.Fatal(err)
  }
  if err := validateArgs([]string{"elevate", "extra"}); err == nil {
    t.Fatal("expected reject")
  }
}
```

- [ ] **Step 2: Implement helper**

Behavior (minimal viable):
- Read `CC_RUNTIME_DIR` / marker files
- **systemd path:** write/remove drop-in `User=root`, `systemctl daemon-reload && systemctl restart <unit>`
- **pm2 path:** if no systemd, return error `systemd required to run agent as root under pm2`
- Prefer invoking real binaries via absolute paths; never `sh -c`

- [ ] **Step 3: Extend `agent_sudoers_content` to append privctl lines when binary exists (or always document install path)**

- [ ] **Step 4: `runtime/root.go` — on command, run sudo helper; on failure send ephemeral error (reuse direct-send) and log

- [ ] **Step 5: Docs update** — sudoers example, WebAuthn `PUBLIC_BASE_URL`, terminal root mode

- [ ] **Step 6: Commit**

```bash
git add agent/ install/ docs/
git commit -m "feat(agent): privctl elevate/demote helper and sudoers"
```

---

### Task 8: End-to-end wiring + status UX

**Files:**
- Modify: `web/components/AgentRootToggle.tsx` — show Enabling / On (root) / On (waiting) / Error from `agent_root_enabled` × `agent_euid_root`
- Modify: control-plane server GET to return both flags
- Optional: agent reports root command result message type if not already covered

- [ ] **Step 1: Manual checklist as automated smoke where possible**
  - Passkey enroll → login with passkey
  - Toggle without passkey → blocked
  - Toggle with passkey → flag true → after mock Hello euid_root true → picker shows all users
  - Toggle off → demote command sent → euid_root false → picker hides again

- [ ] **Step 2: Fix any gaps from checklist**

- [ ] **Step 3: Run full suites**

```bash
cd control-plane && go test ./...
cd agent && go test ./...
cd web && npx tsc --noEmit && npx tsx lib/webauthn.test.ts && npx tsx lib/terminal-users.test.ts
make proto-check
```

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "feat: finish agent root mode status and e2e wiring"
```

---

## Spec coverage checklist

| Spec requirement | Task |
|------------------|------|
| WebAuthn tables + store | 1 |
| Register/login ceremonies + auth/config | 2 |
| Settings Security + login button | 3 |
| Step-up + POST agent-root + server columns | 4 |
| Per-server toggle UI + hide users | 5 |
| Proto + euid on Hello + send command | 6 |
| privctl + sudoers + docs | 7 |
| Status UX + verification | 8 |
| Password not accepted for step-up | 4 |
| Demote on disable | 6–7 |
| pm2 limitation message | 7 |

## Placeholder scan

None intentional. If `INTEGRATION_DB_URL` is unavailable for store tests, use the same pattern as other auth/DB tests in this repo (skip or shared test pool) — do not leave “add tests later”.

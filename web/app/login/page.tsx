"use client";

import { Suspense, useEffect, useRef, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Brand } from "@/components/Brand";
import { LoginShowcase } from "@/components/login/LoginShowcase";
import { ControlPlaneError } from "@/lib/agent-root";
import { loginWithPasskey, supportsConditionalMediation } from "@/lib/webauthn";

/** Auth-noise from signed-out probes — never paint under the form. */
function isSessionNoise(err: unknown): boolean {
  if (err instanceof ControlPlaneError && err.code === "unauthenticated") return true;
  const msg = err instanceof Error ? err.message : String(err);
  return /missing session|unauthenticated|invalid session/i.test(msg);
}

type AuthConfig = {
  password_login: boolean;
  oidc_enabled: boolean;
  oidc_start_url: string;
  github_enabled?: boolean;
  github_start_url?: string;
  gitlab_enabled?: boolean;
  gitlab_start_url?: string;
  passkey_login?: boolean;
};

function safeNext(raw: string | null): string {
  const next = raw || "/";
  // Only allow in-app relative paths (avoid open redirects).
  if (!next.startsWith("/") || next.startsWith("//")) return "/";
  return next;
}

function LoginForm() {
  const router = useRouter();
  const params = useSearchParams();
  const next = safeNext(params.get("next"));

  const [authCfg, setAuthCfg] = useState<AuthConfig | null>(null);
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [passkeyBusy, setPasskeyBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [condGen, setCondGen] = useState(0);
  const condAbort = useRef<AbortController | null>(null);

  useEffect(() => {
    // Quiet session probe (401 is expected when signed out; never surface it).
    fetch("/api/me", { credentials: "include" })
      .then((r) => { if (r.ok) router.replace(next); })
      .catch(() => {});
    fetch("/api/auth/config", { credentials: "include" })
      .then((r) => r.json() as Promise<AuthConfig>)
      .then(setAuthCfg)
      .catch(() => setAuthCfg({ password_login: true, oidc_enabled: false, oidc_start_url: "" }));
  }, [next, router]);

  useEffect(() => {
    if (!authCfg?.passkey_login) return;
    const ac = new AbortController();
    condAbort.current = ac;
    void (async () => {
      if (!(await supportsConditionalMediation())) return;
      try {
        await loginWithPasskey({ mediation: "conditional", signal: ac.signal });
        if (ac.signal.aborted) return;
        const dest = next.startsWith("/app") ? next : next === "/" ? "/app" : `/app${next}`;
        window.location.assign(dest);
      } catch {
        // Conditional UI is ambient autofill — never surface errors under the form
        // (including 401 "missing session" from a signed-out probe race).
      }
    })();
    return () => ac.abort();
  }, [authCfg?.passkey_login, condGen, next, router]);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const res = await fetch("/api/auth/login", {
        method: "POST",
        credentials: "include",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ email, password }),
      });
      if (!res.ok) {
        if (res.status === 401) throw new Error("Wrong email or password");
        throw new Error(`Sign-in failed (HTTP ${res.status})`);
      }
      const dest = next.startsWith("/app") ? next : next === "/" ? "/app" : `/app${next}`;
      window.location.assign(dest);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  function startOAuth(url?: string) {
    if (!url) return;
    const u = new URL(url, window.location.origin);
    const dest = next.startsWith("/app") ? next : (next === "/" ? "/app" : `/app${next}`);
    u.searchParams.set("next", dest);
    window.location.href = u.toString();
  }

  function startSSO() {
    startOAuth(authCfg?.oidc_start_url);
  }

  async function signInWithPasskey() {
    condAbort.current?.abort();
    // Let the aborted conditional ceremony settle before starting a modal one.
    await new Promise((r) => setTimeout(r, 50));
    setPasskeyBusy(true);
    setError(null);
    try {
      await loginWithPasskey();
      // Hard navigate so the app shell mounts with a fresh session (soft nav can
      // keep stale chrome after logout → login).
      const dest = next.startsWith("/app") ? next : next === "/" ? "/app" : `/app${next}`;
      window.location.assign(dest);
    } catch (e) {
      setCondGen((n) => n + 1);
      if (e instanceof DOMException && (e.name === "NotAllowedError" || e.name === "AbortError")) return;
      if (isSessionNoise(e)) {
        setError("Passkey sign-in failed. Try again, or use email and password.");
        return;
      }
      setError((e as Error).message);
    } finally {
      setPasskeyBusy(false);
    }
  }

  return (
    <div className="login-split">
      <LoginShowcase />
      <section className="login-panel">
        <div className="login-panel-inner">
          <Brand />
          <h1>Sign in</h1>
          <p className="subtle login-lede">Welcome back to your control plane.</p>

          {(authCfg?.oidc_enabled || authCfg?.github_enabled || authCfg?.gitlab_enabled || authCfg?.passkey_login) && (
            <div className="stack" style={{ marginTop: 20 }}>
              {authCfg.passkey_login && (
                <button className="button block secondary" onClick={() => void signInWithPasskey()} type="button" disabled={busy || passkeyBusy}>
                  {passkeyBusy ? "Waiting for passkey…" : "Sign in with passkey"}
                </button>
              )}
              {authCfg.oidc_enabled && (
                <button className="button block secondary" onClick={startSSO} type="button">
                  Sign in with SSO
                </button>
              )}
              {authCfg.github_enabled && (
                <button className="button block secondary" onClick={() => startOAuth(authCfg.github_start_url)} type="button">
                  Sign in with GitHub
                </button>
              )}
              {authCfg.gitlab_enabled && (
                <button className="button block secondary" onClick={() => startOAuth(authCfg.gitlab_start_url)} type="button">
                  Sign in with GitLab
                </button>
              )}
              <div className="divider">or with email</div>
            </div>
          )}

          <form onSubmit={submit} className="stack" style={{ marginTop: 16 }}>
            <div>
              <label htmlFor="email">Email</label>
              <input
                id="email"
                type="email"
                autoComplete={authCfg?.passkey_login ? "username webauthn" : "email"}
                placeholder="you@example.com"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
              />
            </div>
            <div>
              <label htmlFor="password">Password</label>
              <input
                id="password"
                type="password"
                autoComplete="current-password"
                placeholder="••••••••"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
              />
            </div>
            {error && <p className="form-error">{error}</p>}
            <button type="submit" className="button block" disabled={busy || passkeyBusy || !email || !password}>
              {busy ? "Signing in…" : "Sign in"}
            </button>
          </form>
        </div>
      </section>
    </div>
  );
}

function LoginFallback() {
  return (
    <div className="login-split">
      <aside className="login-showcase" aria-hidden>
        <div className="login-showcase-glow" />
        <div className="login-showcase-grid" />
      </aside>
      <section className="login-panel">
        <div className="login-panel-inner">
          <Brand />
          <h1>Sign in</h1>
          <p className="subtle login-lede">Welcome back to your control plane.</p>
        </div>
      </section>
    </div>
  );
}

// useSearchParams() must sit inside a Suspense boundary or Next.js 16 fails to
// statically prerender /login (CSR bailout). Matching fallback prevents the
// blank flash when arriving via ?next= redirect.
export default function LoginPage() {
  return (
    <Suspense fallback={<LoginFallback />}>
      <LoginForm />
    </Suspense>
  );
}

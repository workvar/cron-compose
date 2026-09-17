"use client";

import { Suspense, useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Brand } from "@/components/Brand";
import { loginWithPasskey } from "@/lib/webauthn";

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

function LoginForm() {
  const router = useRouter();
  const params = useSearchParams();
  const next = params.get("next") || "/";

  const [authCfg, setAuthCfg] = useState<AuthConfig | null>(null);
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [passkeyBusy, setPasskeyBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetch("/api/me", { credentials: "include" })
      .then((r) => { if (r.ok) router.replace(next); })
      .catch(() => {});
    fetch("/api/auth/config", { credentials: "include" })
      .then((r) => r.json() as Promise<AuthConfig>)
      .then(setAuthCfg)
      .catch(() => setAuthCfg({ password_login: true, oidc_enabled: false, oidc_start_url: "" }));
  }, [next, router]);

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
      router.push(next);
      router.refresh();
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
    setPasskeyBusy(true);
    setError(null);
    try {
      await loginWithPasskey();
      router.push(next);
      router.refresh();
    } catch (e) {
      if (e instanceof DOMException && (e.name === "NotAllowedError" || e.name === "AbortError")) return;
      setError((e as Error).message);
    } finally {
      setPasskeyBusy(false);
    }
  }

  return (
    <div className="auth-card">
        <Brand />
        <h1 style={{ marginTop: 18 }}>Sign in</h1>
        <p className="subtle" style={{ margin: "0 0 4px" }}>
          Welcome back to your control plane.
        </p>

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
              autoComplete="email"
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
  );
}

// useSearchParams() must sit inside a Suspense boundary or Next.js 16 fails to
// statically prerender /login (CSR bailout). The boundary keeps the build happy.
export default function LoginPage() {
  return (
    <Suspense fallback={null}>
      <LoginForm />
    </Suspense>
  );
}

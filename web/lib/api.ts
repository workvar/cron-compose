// Thin fetch wrapper around the control-plane REST API. Server components forward
// the user's session cookie so authenticated reads work in SSR.
import { cookies } from "next/headers";
import { apiBase } from "./apiBase";

// Thrown by every failed call so callers can tell "not signed in" (401/403) apart
// from "the control plane is down", which need different UI.
export class ApiError extends Error {
  constructor(
    readonly status: number,   // 0 when the request never got a response
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
  get unauthorized() { return this.status === 401 || this.status === 403; }
  get unreachable()  { return this.status === 0; }
}

async function call<T>(method: string, path: string, body?: unknown): Promise<T> {
  const base = apiBase();
  const cookieJar = await cookies();
  const cookieHeader = cookieJar
    .getAll()
    .map((c) => `${c.name}=${c.value}`)
    .join("; ");

  const headers: Record<string, string> = {};
  if (body !== undefined) headers["content-type"] = "application/json";
  if (cookieHeader) headers["cookie"] = cookieHeader;

  let res: Response;
  try {
    res = await fetch(`${base}${path}`, {
      method,
      headers,
      body: body !== undefined ? JSON.stringify(body) : undefined,
      cache: "no-store",
    });
  } catch (err) {
    throw new ApiError(0, `${method} ${path}: ${(err as Error).message}`);
  }
  if (!res.ok) {
    const txt = await res.text().catch(() => "");
    let detail = txt;
    try {
      const parsed = JSON.parse(txt) as { error?: { message?: string } };
      if (parsed?.error?.message) detail = parsed.error.message;
    } catch { /* keep raw body */ }
    throw new ApiError(res.status, `${method} ${path}: ${res.status} ${detail}`.trim());
  }
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

export const apiGet    = <T>(p: string)               => call<T>("GET", p);
export const apiPost   = <T>(p: string, body: unknown) => call<T>("POST", p, body);
export const apiPatch  = <T>(p: string, body: unknown) => call<T>("PATCH", p, body);
export const apiDelete = <T>(p: string)               => call<T>("DELETE", p);

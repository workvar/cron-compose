/** Browser Web Push helpers for CronCompose notifications. */

export const PUSH_EVENT_LABELS: Record<string, string> = {
  deploy_started: "Deploy started",
  deploy_succeeded: "Deploy succeeded",
  deploy_failed: "Deploy failed",
  job_failed: "Job failed",
  update_available: "Update available",
};

export type PushSubscriptionRow = {
  id: string;
  endpoint: string;
  user_agent?: string;
  events: string[];
  created_at: string;
  last_seen_at: string;
};

function urlBase64ToUint8Array(base64String: string): Uint8Array {
  const padding = "=".repeat((4 - (base64String.length % 4)) % 4);
  const base64 = (base64String + padding).replace(/-/g, "+").replace(/_/g, "/");
  const raw = atob(base64);
  const out = new Uint8Array(raw.length);
  for (let i = 0; i < raw.length; i++) out[i] = raw.charCodeAt(i);
  return out;
}

function arrayBufferToBase64Url(buf: ArrayBuffer): string {
  const bytes = new Uint8Array(buf);
  let s = "";
  for (let i = 0; i < bytes.length; i++) s += String.fromCharCode(bytes[i]);
  return btoa(s).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

export function pushSupported(): boolean {
  return (
    typeof window !== "undefined" &&
    "serviceWorker" in navigator &&
    "PushManager" in window &&
    "Notification" in window
  );
}

export async function ensurePushServiceWorker(): Promise<ServiceWorkerRegistration> {
  // basePath is /app — register the worker under that scope.
  return navigator.serviceWorker.register("/app/sw.js", { scope: "/app/" });
}

export async function fetchVapidPublicKey(): Promise<{ public_key: string; events: string[] }> {
  const res = await fetch("/api/push/vapid-public-key", { credentials: "include" });
  if (!res.ok) throw new Error(`VAPID key HTTP ${res.status}`);
  return res.json();
}

export async function subscribeBrowserPush(events?: string[]): Promise<PushSubscriptionRow> {
  const reg = await ensurePushServiceWorker();
  await navigator.serviceWorker.ready;
  const perm = await Notification.requestPermission();
  if (perm !== "granted") {
    throw new Error("Notification permission was not granted");
  }
  const { public_key, events: defaults } = await fetchVapidPublicKey();
  let sub = await reg.pushManager.getSubscription();
  if (!sub) {
    sub = await reg.pushManager.subscribe({
      userVisibleOnly: true,
      applicationServerKey: urlBase64ToUint8Array(public_key),
    });
  }
  const json = sub.toJSON();
  const res = await fetch("/api/push/subscriptions", {
    method: "POST",
    credentials: "include",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({
      endpoint: json.endpoint,
      p256dh: json.keys?.p256dh || arrayBufferToBase64Url(sub.getKey("p256dh")!),
      auth: json.keys?.auth || arrayBufferToBase64Url(sub.getKey("auth")!),
      user_agent: navigator.userAgent,
      events: events?.length ? events : defaults,
    }),
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body?.error?.message || `Subscribe HTTP ${res.status}`);
  }
  return res.json();
}

export async function listPushSubscriptions(): Promise<PushSubscriptionRow[]> {
  const res = await fetch("/api/push/subscriptions", { credentials: "include" });
  if (!res.ok) return [];
  const body = await res.json();
  return body.items || [];
}

export async function updatePushEvents(id: string, events: string[]): Promise<PushSubscriptionRow> {
  const res = await fetch(`/api/push/subscriptions/${id}`, {
    method: "PATCH",
    credentials: "include",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ events }),
  });
  if (!res.ok) throw new Error(`Update HTTP ${res.status}`);
  return res.json();
}

export async function deletePushSubscription(id: string): Promise<void> {
  const reg = await navigator.serviceWorker.getRegistration("/app/");
  const sub = await reg?.pushManager.getSubscription();
  await sub?.unsubscribe();
  await fetch(`/api/push/subscriptions/${id}`, { method: "DELETE", credentials: "include" });
}

export async function testPush(): Promise<void> {
  const res = await fetch("/api/push/test", { method: "POST", credentials: "include" });
  const body = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(body?.error?.message || `Test HTTP ${res.status}`);
  }
}

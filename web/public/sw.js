/* CronCompose push service worker — shows notifications when the tab is closed. */
/* eslint-disable no-restricted-globals */

self.addEventListener("install", (event) => {
  event.waitUntil(self.skipWaiting());
});

self.addEventListener("activate", (event) => {
  event.waitUntil(self.clients.claim());
});

self.addEventListener("push", (event) => {
  let data = {
    title: "CronCompose",
    body: "Something happened.",
    url: "/app",
    tag: "croncompose",
  };
  try {
    if (event.data) {
      data = { ...data, ...event.data.json() };
    }
  } catch {
    try {
      data.body = event.data ? event.data.text() : data.body;
    } catch {
      /* keep defaults */
    }
  }
  const opts = {
    body: data.body || "",
    tag: data.tag || data.event || "croncompose",
    renotify: true,
    data: { url: data.url || "/app", ...(data.data || {}) },
    icon: data.icon || "/app/logo.png",
    badge: data.icon || "/app/logo.png",
  };
  event.waitUntil(self.registration.showNotification(data.title || "CronCompose", opts));
});

self.addEventListener("notificationclick", (event) => {
  event.notification.close();
  const raw = (event.notification.data && event.notification.data.url) || "/app";
  const target = new URL(raw, self.location.origin).href;
  event.waitUntil(
    self.clients.matchAll({ type: "window", includeUncontrolled: true }).then((clients) => {
      for (const client of clients) {
        if ("focus" in client && client.url.startsWith(self.location.origin)) {
          client.navigate(target);
          return client.focus();
        }
      }
      if (self.clients.openWindow) {
        return self.clients.openWindow(target);
      }
      return undefined;
    }),
  );
});

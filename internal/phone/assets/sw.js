"use strict";
// broker's service worker: shows request notifications, and keeps a copy
// of the page so opening it while broker is not listening still says so
// instead of showing a browser error.
const CACHE = "broker-1";
const SHELL = ["/", "/app.js", "/icon.svg"];

self.addEventListener("install", (e) => {
  e.waitUntil(caches.open(CACHE).then((c) => c.addAll(SHELL)).then(() => self.skipWaiting()));
});

self.addEventListener("activate", (e) => {
  e.waitUntil(caches.keys()
    .then((keys) => Promise.all(keys.filter((k) => k !== CACHE).map((k) => caches.delete(k))))
    .then(() => self.clients.claim()));
});

self.addEventListener("fetch", (e) => {
  const url = new URL(e.request.url);
  if (e.request.method !== "GET" || url.origin !== location.origin || url.pathname.startsWith("/api/")) return;
  const shell = url.pathname === "/enroll" ? "/" : url.pathname;
  e.respondWith(fetch(e.request)
    .then((res) => {
      if (res.ok && SHELL.includes(url.pathname)) {
        const copy = res.clone();
        caches.open(CACHE).then((c) => c.put(url.pathname, copy));
      }
      return res;
    })
    .catch(() => caches.match(shell).then((res) => res || Response.error())));
});

// The payload is broker's, but it only ever says how many secrets; the
// details are on the page.
self.addEventListener("push", (e) => {
  let d = {};
  try { d = e.data ? e.data.json() : {}; } catch (_) { /* show the default */ }
  e.waitUntil(self.registration.showNotification(String(d.title || "broker"), {
    body: String(d.body || "An agent is asking for secrets."),
    tag: "broker-request",
    renotify: true,
    requireInteraction: true,
  }));
});

// A tap always opens this origin's page, never a URL from the payload.
self.addEventListener("notificationclick", (e) => {
  e.notification.close();
  e.waitUntil((async () => {
    const wins = await self.clients.matchAll({ type: "window", includeUncontrolled: true });
    for (const w of wins) {
      if (new URL(w.url).pathname === "/") {
        w.postMessage("refresh");
        return w.focus();
      }
    }
    return self.clients.openWindow("/");
  })());
});

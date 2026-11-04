"use strict";
// broker's phone page. At /enroll it makes a WebAuthn credential on the
// security key plugged into this phone and hands broker the key's PRF
// output; at / it shows the request broker is asking about and approves
// it with the same key. Everything broker sends is shown with
// textContent, never parsed as HTML.

const TOKEN = "broker.token";
const app = document.getElementById("app");

const b64 = {
  enc(buf) {
    let s = "";
    for (const b of new Uint8Array(buf)) s += String.fromCharCode(b);
    return btoa(s).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
  },
  dec(str) {
    const s = atob(str.replace(/-/g, "+").replace(/_/g, "/") + "===".slice((str.length + 3) % 4));
    return Uint8Array.from(s, (c) => c.charCodeAt(0));
  },
};

function el(tag, attrs, ...kids) {
  const e = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs || {})) {
    if (k === "class") e.className = v;
    else if (k.startsWith("on")) e.addEventListener(k.slice(2), v);
    else if (v === true) e.setAttribute(k, "");
    else if (v !== false && v != null) e.setAttribute(k, v);
  }
  for (const k of kids) if (k != null) e.append(k.nodeType ? k : document.createTextNode(String(k)));
  return e;
}

function header(title, sub) {
  return el("header", {}, el("div", { class: "icon" }, el("img", { src: "/icon.svg", alt: "" })),
    el("div", {}, el("h1", {}, title), sub ? el("div", { class: "sub", id: "sub" }, sub) : null));
}

function result(text, bad) {
  return el("div", { class: "action" }, el("div", { class: "result" + (bad ? " bad" : "") },
    el("span", { class: "mark" }, bad ? "✕" : "✓"), text));
}

function errorBox(text) { return el("div", { class: "action error" }, text); }

function waiting(text) {
  return el("div", { class: "action touch" }, el("span", { class: "spin" }), el("div", { class: "hint" }, text));
}

function random(n) { return crypto.getRandomValues(new Uint8Array(n)); }

// prfOf pulls the PRF output out of a WebAuthn result, or explains why
// there is none.
function prfOf(cred) {
  const ext = cred.getClientExtensionResults();
  if (!ext.prf) throw new Error("This browser does not support the WebAuthn PRF extension.");
  if (ext.prf.enabled === false) throw new Error("This security key does not support PRF (hmac-secret).");
  return ext.prf.results && ext.prf.results.first;
}

function why(e) {
  if (e && e.name === "NotAllowedError") return "The security key prompt was cancelled or timed out.";
  if (e && e.name === "InvalidStateError") return "This key is already registered for this page. Remove it with broker phone rm, or use another key.";
  return (e && e.message) || String(e);
}

if ("serviceWorker" in navigator) {
  navigator.serviceWorker.register("/sw.js").catch(() => {});
  navigator.serviceWorker.addEventListener("message", (e) => { if (e.data === "refresh") watch(); });
}

// ---------------------------------------------------------------- enroll

async function enroll() {
  const token = location.hash.slice(1);
  // Keep the one-time token out of history and screenshots.
  history.replaceState(null, "", "/enroll");
  const st = { step: "start", error: null, opts: null, cred: null, prf: null, push: null, pushNote: null };

  const render = () => {
    const kids = [header("Enroll this phone", "broker")];
    if (!token) {
      kids.push(errorBox("This page needs the link broker phone enroll printed, with its #token."));
      return app.replaceChildren(...kids);
    }
    kids.push(el("ol", {},
      el("li", {}, "Plug your YubiKey into this phone."),
      el("li", {}, "Tap Use security key and touch the key each time it blinks (once or twice)."),
      el("li", {}, "Allow notifications, so requests reach you."),
      el("li", {}, "Finish on the PC: follow the terminal there to unlock the vault with its own key.")));
    if (st.error) kids.push(errorBox(st.error));
    if (st.step === "start" || st.step === "key") {
      kids.push(el("div", { class: "buttons" },
        el("button", { class: "primary", disabled: st.step === "key", onclick: useKey }, st.step === "key" ? "Waiting for the key…" : "Use security key")));
    } else if (st.step === "push") {
      kids.push(el("div", { class: "buttons" },
        el("button", { class: "primary", onclick: allowPush }, "Allow notifications"),
        el("button", { onclick: () => { st.pushNote = "Notifications are off: open this page yourself when an agent asks."; send(); } }, "Skip")));
    } else if (st.step === "sending") {
      kids.push(waiting("Saving…"));
    } else if (st.step === "pc") {
      kids.push(el("div", { class: "action touch" }, el("span", { class: "dot" }),
        el("div", {}, el("div", { class: "title" }, "Now finish on the PC"),
          el("div", { class: "hint" }, "The terminal there says which YubiKey unlocks the vault. If it is the one in this phone, move it to the PC; otherwise leave this one here. Then press Enter there and touch the PC's key."))));
    } else if (st.step === "done") {
      kids.push(result("This phone is enrolled", false));
      kids.push(el("div", { class: "note" }, st.pushNote || "A test notification is on its way."));
    } else if (st.step === "failed") {
      kids.push(errorBox("Enrolling failed on the PC: " + st.failure));
      kids.push(el("div", { class: "note" }, "Nothing was added. Run broker phone enroll again."));
    }
    app.replaceChildren(...kids);
  };

  const call = async (method, body) => {
    const res = await fetch("/api/enroll", {
      method, headers: { Authorization: "Bearer " + token, "Content-Type": "application/json" },
      body: body ? JSON.stringify(body) : undefined,
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(data.error || res.statusText);
    return data;
  };

  async function useKey() {
    st.step = "key"; st.error = null; render();
    try {
      const o = st.opts || (st.opts = await call("GET"));
      const salt = b64.dec(o.salt);
      const cred = await navigator.credentials.create({ publicKey: {
        rp: { id: o.rpId, name: "broker" },
        user: { id: b64.dec(o.userId), name: "broker", displayName: "broker" },
        challenge: b64.dec(o.challenge),
        pubKeyCredParams: [{ type: "public-key", alg: -7 }, { type: "public-key", alg: -8 }, { type: "public-key", alg: -257 }],
        authenticatorSelection: { authenticatorAttachment: "cross-platform", residentKey: "discouraged", requireResidentKey: false, userVerification: "discouraged" },
        attestation: "none",
        timeout: 120000,
        extensions: { prf: { eval: { first: salt } } },
      } });
      let prf = prfOf(cred);
      if (!prf) {
        // Most keys only evaluate PRF on an assertion, not at creation.
        const got = await navigator.credentials.get({ publicKey: {
          rpId: o.rpId, challenge: random(32), timeout: 120000, userVerification: "discouraged",
          allowCredentials: [{ type: "public-key", id: cred.rawId }],
          extensions: { prf: { eval: { first: salt } } },
        } });
        prf = prfOf(got);
      }
      if (!prf) throw new Error("The key gave no PRF output.");
      st.cred = b64.enc(cred.rawId); st.prf = b64.enc(prf);
      st.step = ("PushManager" in window && "Notification" in window) ? "push" : "sending";
      if (st.step === "sending") { st.pushNote = "This browser cannot get notifications."; send(); }
    } catch (e) {
      st.step = "start"; st.error = why(e);
    }
    render();
  }

  async function allowPush() {
    try {
      if (await Notification.requestPermission() !== "granted") {
        st.pushNote = "Notifications were not allowed: open this page yourself when an agent asks.";
      } else {
        const reg = await navigator.serviceWorker.ready;
        // A subscription made for an older key would refuse the new one.
        const old = await reg.pushManager.getSubscription();
        if (old) await old.unsubscribe();
        const sub = (await reg.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: b64.dec(st.opts.vapid) })).toJSON();
        st.push = { endpoint: sub.endpoint, p256dh: sub.keys.p256dh, auth: sub.keys.auth };
      }
    } catch (e) {
      st.pushNote = "Notifications could not be set up (" + why(e) + "): open this page yourself when an agent asks.";
    }
    send();
  }

  async function send() {
    st.step = "sending"; render();
    try {
      const r = await call("POST", { credential: st.cred, prf: st.prf, push: st.push });
      st.prf = null;
      st.step = "pc"; render();
      await waitForPC(r.token);
    } catch (e) {
      st.step = "start"; st.error = why(e);
    }
    render();
  }

  // The phone's part is half of enrolling: the PC still has to unlock the
  // vault and wrap it to this key. Only then is the token worth keeping.
  async function waitForPC(phoneToken) {
    for (;;) {
      let data;
      try {
        const res = await fetch("/api/enroll/result", { headers: { Authorization: "Bearer " + phoneToken }, cache: "no-store" });
        data = await res.json();
        if (!res.ok) throw new Error(data.error || res.statusText);
      } catch (e) {
        st.step = "failed"; st.failure = "lost touch with the PC (" + why(e) + "). Check the terminal there.";
        return;
      }
      if (!data.done) continue;
      if (data.error) { st.step = "failed"; st.failure = data.error; return; }
      localStorage.setItem(TOKEN, phoneToken);
      st.step = "done";
      return;
    }
  }

  render();
}

// ---------------------------------------------------------------- watch

const sessionRoots = new Set([
  "systemd", "init", "login", "agetty", "launchd", "loginwindow",
  "Hyprland", "start-hyprland", "sway", "niri", "river", "labwc", "wayfire", "hikari", "dwl", "Xorg", "Xwayland",
  "gnome-shell", "gnome-session-b", "gdm-wayland-ses", "gdm-x-session", "gdm-session-wor",
  "kwin_wayland", "kwin_x11", "plasmashell", "startplasma-way", "startplasma-x11", "ksmserver", "sddm-helper",
  "lightdm", "xfce4-session", "cinnamon-sessio", "mate-session", "i3", "bspwm", "awesome", "openbox",
]);

// changedNote explains a request for secrets that are still granted but
// whose values changed in the vault since: the grants hold the old values.
function changedNote(secrets) {
  const names = secrets.filter((s) => s.changed).map((s) => s.name);
  if (!names.length) return null;
  const one = names.length === 1;
  const list = names.length < 3 ? names.join(" and ") : names.slice(0, -1).join(", ") + ", and " + names[names.length - 1];
  return el("div", { class: "changed" }, "Asking again: " + list + (one ? " is" : " are") +
    " still granted, but the value changed in the vault since, so the grant holds the old one. Approving grants the new " +
    (one ? "value." : "values."));
}

// extendNote explains a request for secrets that are still granted but
// end sooner than asked: approving replaces the grants.
function extendNote(secrets) {
  const names = secrets.filter((s) => s.expires).map((s) => s.name);
  if (!names.length) return null;
  const one = names.length === 1;
  const list = names.length < 3 ? names.join(" and ") : names.slice(0, -1).join(", ") + ", and " + names[names.length - 1];
  return el("div", { class: "changed" }, "Extending: " + list + (one ? " is" : " are") +
    " still granted, but " + (one ? "ends" : "end") + " sooner than asked. Approving replaces the " +
    (one ? "grant with a new one" : "grants with new ones") + " for the time below.");
}

// grantLeft is how long a secret's live grant has left, to the minute.
function grantLeft(s) {
  return human(Math.max(60, Math.round((Date.parse(s.expires) - Date.now()) / 60000) * 60)) + " left";
}

function human(secs) {
  if (secs % 3600 === 0) { const h = secs / 3600; return h + (h === 1 ? " hour" : " hours"); }
  if (secs < 3600 && secs % 60 === 0) return (secs / 60) + " min";
  const h = Math.floor(secs / 3600), m = Math.round((secs % 3600) / 60);
  return (h ? h + "h " : "") + m + "m";
}

function quote(argv) {
  return argv.map((a) => /^[\w@%+=:,./-]+$/.test(a) ? a : "'" + a.split("'").join("'\\''") + "'").join(" ");
}

let watching = null;

function watch() {
  if (watching) watching.stop();
  watching = new Watch();
}

class Watch {
  constructor() {
    this.token = localStorage.getItem(TOKEN);
    this.req = null; this.key = null; this.next = 0; this.picked = 0;
    this.phase = "loading"; // loading, idle, request, key, unlocking, denying, done, unenrolled
    this.status = null; this.error = null; this.done = null; this.stopped = false;
    this.timer = setInterval(() => this.tick(), 1000);
    this.render();
    if (!this.token) { this.phase = "unenrolled"; this.render(); return; }
    this.loop();
  }

  stop() { this.stopped = true; clearInterval(this.timer); }

  async loop() {
    while (!this.stopped) {
      let data;
      try {
        const res = await fetch("/api/poll?after=" + this.next, { headers: { Authorization: "Bearer " + this.token }, cache: "no-store" });
        if (res.status === 401) { this.phase = "unenrolled"; return this.render(); }
        if (!res.ok) throw new Error(res.statusText);
        data = await res.json();
      } catch (_) {
        // broker only listens while it is asking.
        if (this.phase !== "done") this.phase = this.req ? "done" : "idle";
        if (this.req && !this.done) this.done = { text: "This request is over", bad: true };
        return this.render();
      }
      if (this.stopped) return;
      this.key = { rpId: data.rpId, credential: data.credential, salt: data.salt };
      for (const m of data.messages) this.receive(m);
      this.next = data.next;
      if (this.phase === "done") return this.render();
      if (data.closed) {
        if (!this.done) this.done = { text: "This request is over", bad: true };
        this.phase = "done";
        return this.render();
      }
      this.render();
    }
  }

  receive(m) {
    switch (m.type) {
      case "request":
        this.req = m.request;
        if (this.phase === "loading" || this.phase === "idle") this.phase = "request";
        break;
      case "status":
        // The window's own steps (touch the key on the PC) mean nothing
        // here; only what follows this phone's answer is shown.
        if (this.phase === "unlocking") this.status = m;
        break;
      case "done":
        this.done = { text: m.text, bad: !!m.error };
        this.phase = "done";
        break;
    }
  }

  async reply(body) {
    try {
      await fetch("/api/reply", { method: "POST", headers: { Authorization: "Bearer " + this.token, "Content-Type": "application/json" }, body: JSON.stringify(body) });
    } catch (_) { /* the poll notices broker going away */ }
  }

  async approve() {
    this.phase = "key"; this.error = null; this.render();
    try {
      const left = new Date(this.req.deadline) - Date.now();
      const got = await navigator.credentials.get({ publicKey: {
        rpId: this.key.rpId, challenge: random(32), userVerification: "discouraged",
        timeout: Math.max(10000, left),
        allowCredentials: [{ type: "public-key", id: b64.dec(this.key.credential) }],
        extensions: { prf: { eval: { first: b64.dec(this.key.salt) } } },
      } });
      const prf = prfOf(got);
      if (!prf) throw new Error("The key gave no PRF output.");
      this.phase = "unlocking"; this.status = null; this.render();
      const res = await fetch("/api/unlock", {
        method: "POST", headers: { Authorization: "Bearer " + this.token, "Content-Type": "application/json" },
        body: JSON.stringify({ prf: b64.enc(prf), for: this.picked || this.req.for }),
      });
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        throw new Error(data.error || res.statusText);
      }
    } catch (e) {
      if (this.phase !== "done") { this.phase = "request"; this.error = why(e); }
    }
    this.render();
  }

  tick() {
    const s = document.getElementById("sub");
    if (s && this.req && (this.phase === "request" || this.phase === "key")) s.textContent = "broker request · expires in " + this.left();
  }

  left() {
    const ms = new Date(this.req.deadline) - Date.now();
    if (ms <= 0) return "0:00";
    const s = Math.ceil(ms / 1000);
    return Math.floor(s / 60) + ":" + String(s % 60).padStart(2, "0");
  }

  tilde(s) {
    const home = this.req && this.req.home;
    if (!home || !s) return s;
    if (s === home) return "~";
    return s.split(home + "/").join("~/");
  }

  render() {
    if (this.stopped) return;
    if (this.phase === "loading") return app.replaceChildren(header("broker", "Checking…"));
    if (this.phase === "unenrolled") {
      return app.replaceChildren(header("broker"), errorBox("This phone is not enrolled. On the PC, run broker phone enroll and open the link it prints."));
    }
    if (this.phase === "idle") {
      return app.replaceChildren(header("broker"),
        el("div", { class: "action" }, el("div", { class: "title" }, "Nothing to approve"),
          el("div", { class: "hint" }, "broker is not asking for anything right now. A notification brings you back here when an agent asks.")),
        el("div", { class: "buttons" }, el("button", { onclick: () => watch() }, "Check again")));
    }
    const r = this.req;
    if (!r) return app.replaceChildren(header("broker"), this.done ? result(this.done.text, this.done.bad) : waiting("Loading…"));
    const n = r.secrets.length;
    const full = (r.ancestry || []).slice(r.ancestry && r.ancestry[0] === "broker" ? 1 : 0)
      .filter((c, i, a) => i === 0 || c !== a[i - 1]);
    const root = full.findIndex((c) => sessionRoots.has(c));
    let chain = root > 0 ? full.slice(0, root) : full;
    if (chain.length > 6) chain = [...chain.slice(0, 6), "…"];
    const picked = this.picked || r.for;
    const open = this.phase === "request";

    let forView = human(picked);
    if (open && (r.choices || []).length > 1) {
      forView = el("select", { onchange: (e) => { this.picked = Number(e.target.value); this.reply({ type: "for", for: this.picked }); } });
      for (const c of [...r.choices].sort((a, b) => b - a)) {
        forView.append(el("option", { value: String(c), selected: c === picked }, human(c) + (c === r.for ? " (asked)" : "")));
      }
    }

    let action;
    if (this.phase === "done") action = result(this.done.text, this.done.bad);
    else if (this.phase === "key") {
      action = el("div", { class: "action touch" }, el("span", { class: "dot" }),
        el("div", {}, el("div", { class: "title" }, "Touch your YubiKey"), el("div", { class: "hint" }, "It is plugged into this phone.")));
    } else if (this.phase === "denying") {
      action = waiting("Denying\u2026");
    } else if (this.phase === "unlocking") {
      action = this.status && this.status.error ? errorBox(this.status.text) : waiting("Unlocking on the PC…");
    } else {
      action = el("div", {}, this.error ? errorBox(this.error) : null,
        el("div", { class: "buttons" },
          el("button", { class: "primary", onclick: () => this.approve() }, "Approve with security key"),
          el("button", { onclick: () => { this.reply({ type: "deny" }); this.phase = "denying"; this.render(); } }, "Deny")));
    }

    app.replaceChildren(
      header(n === 1 ? "An agent wants to use a secret" : "An agent wants to use " + n + " secrets",
        this.phase === "done" ? "broker request" : "broker request · expires in " + this.left()),
      el("div", { class: "label" }, n === 1 ? "Secret" : "Secrets"),
      ...r.secrets.map((s) => el("div", { class: "secret" },
        el("div", {}, el("span", { class: "mono" }, s.name), s.changed ? el("span", { class: "tag" }, "changed") : null,
          s.expires ? el("span", { class: "tag" }, grantLeft(s)) : null),
        s.description ? el("div", { class: "desc" }, s.description) : null)),
      changedNote(r.secrets),
      extendNote(r.secrets),
      el("div", { class: "label" }, "Request"),
      el("div", { class: "grid" },
        el("div", { class: "k" }, "For"), el("div", { class: "v" }, forView),
        el("div", { class: "k" }, "Asked by"), el("div", { class: "v mono" }, chain.join(" < ") || "unknown"),
        r.cwd ? el("div", { class: "k" }, "Directory") : null, r.cwd ? el("div", { class: "v mono" }, this.tilde(r.cwd)) : null,
        r.command && r.command.length ? el("div", { class: "k" }, "Will run") : null,
        r.command && r.command.length ? el("div", { class: "v mono" }, quote(r.command)) : null),
      r.reason ? el("div", { class: "claim" }, el("div", { class: "who" }, "The agent says (unverified)"), r.reason) : null,
      el("div", { class: "note" }, "While granted, any agent on the PC can use " + (n === 1 ? "it" : "them") + ", not just this one."),
      action);
  }
}

if (location.pathname === "/enroll") enroll();
else {
  watch();
  document.addEventListener("visibilitychange", () => {
    if (document.visibilityState === "visible" && watching && (watching.phase === "idle" || watching.phase === "done")) watch();
  });
}

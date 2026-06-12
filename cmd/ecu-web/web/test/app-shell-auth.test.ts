import { test, expect, describe, afterEach } from "bun:test";
import "../src/app-shell.ts";
import type { EcuApp } from "../src/app-shell.ts";
import { api } from "../src/api.ts";

// Mount <ecu-app> but neutralise init()'s authStatus fetch so the shell lands
// in a deterministic unauthed-but-ready state without network. We then drive
// auth state directly, the way onAuthed/logout do.
async function mount(): Promise<EcuApp> {
  location.hash = "#/dashboard";
  const origFetch = globalThis.fetch;
  // authStatus() returns {configured, authenticated:false} → ready, unauthed.
  globalThis.fetch = (async () =>
    new Response(JSON.stringify({ configured: true, authenticated: false }), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    })) as typeof fetch;
  const el = document.createElement("ecu-app") as EcuApp;
  document.body.appendChild(el);
  await el.updateComplete;
  // let init()'s async authStatus resolve
  await new Promise((r) => setTimeout(r, 0));
  await el.updateComplete;
  globalThis.fetch = origFetch;
  return el;
}

afterEach(() => {
  document.body.querySelectorAll("ecu-app").forEach((n) => n.remove());
});

describe("<ecu-app> 401 handling", () => {
  test("a 401 from /api/system while authed drops to the login view", async () => {
    const el = await mount();

    // Force the shell into an authed state, as onAuthed would. Avoid
    // startStreams() (SSE/timers) — we only exercise the 401 path.
    el.authed = true;
    el.fleet = { ts_ms: 1, inverters: [] } as never;
    el.system = { invdriver_connected: true, peers: [] } as never;
    await el.updateComplete;
    expect(el.shadowRoot!.querySelector("login-view")).toBeNull();

    // Drive a real 401 through the api layer so the registered handler fires.
    const origFetch = globalThis.fetch;
    globalThis.fetch = (async () =>
      new Response("", { status: 401 })) as typeof fetch;
    try {
      await expect(api.system()).rejects.toThrow();
    } finally {
      globalThis.fetch = origFetch;
    }

    await el.updateComplete;
    expect(el.authed).toBe(false);
    expect(el.fleet).toBeNull();
    expect(el.system).toBeNull();
    expect(el.shadowRoot!.querySelector("login-view")).not.toBeNull();
  });

  test("a 401 while NOT authed (login attempt) is a no-op", async () => {
    const el = await mount();
    expect(el.authed).toBe(false);

    // A wrong-password login returns 401; the handler must not throw or change
    // state — the login view surfaces its own error.
    const origFetch = globalThis.fetch;
    globalThis.fetch = (async () =>
      new Response(JSON.stringify({ error: "bad password" }), {
        status: 401,
        headers: { "Content-Type": "application/json" },
      })) as typeof fetch;
    try {
      await expect(api.login("nope")).rejects.toThrow();
    } finally {
      globalThis.fetch = origFetch;
    }

    await el.updateComplete;
    expect(el.authed).toBe(false);
    // still on the login view, no crash
    expect(el.shadowRoot!.querySelector("login-view")).not.toBeNull();
  });
});

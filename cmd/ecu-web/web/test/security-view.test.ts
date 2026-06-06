import { test, expect, describe, beforeEach, afterEach } from "bun:test";
import "../src/views/security-view.ts";
import type { SecurityView } from "../src/views/security-view.ts";
import type { AccessState } from "../src/api.ts";

// fakeAccess builds the minimal /api/access/ssh-keys response the view needs.
function fakeAccess(comment: string): AccessState {
  return {
    provider: "openaps",
    keys: [
      { fingerprint: "SHA256:abc", comment, added_ms: 0 },
    ],
  } as AccessState;
}

let originalFetch: typeof globalThis.fetch;
let fetchCount = 0;
let currentComment = "first";

function installCountingFetch() {
  fetchCount = 0;
  globalThis.fetch = (async (input: RequestInfo | URL) => {
    const url = typeof input === "string" ? input : input.toString();
    if (url.includes("/api/access/ssh-keys")) {
      fetchCount++;
      return new Response(JSON.stringify(fakeAccess(currentComment)), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    }
    return new Response("{}", { status: 200, headers: { "Content-Type": "application/json" } });
  }) as typeof globalThis.fetch;
}

async function flush() {
  await new Promise((r) => setTimeout(r, 0));
}

beforeEach(() => {
  originalFetch = globalThis.fetch;
  currentComment = "first";
  installCountingFetch();
});

afterEach(() => {
  globalThis.fetch = originalFetch;
});

describe("security-view", () => {
  test("fetches the key list on connect", async () => {
    const el = document.createElement("security-view") as SecurityView;
    document.body.appendChild(el);
    await flush();
    await el.updateComplete;
    expect(fetchCount).toBe(1);
    expect(el.state?.keys?.[0]?.comment).toBe("first");
    el.remove();
  });

  test("re-fetches every time the view reconnects (no stale cache)", async () => {
    const el = document.createElement("security-view") as SecurityView;
    document.body.appendChild(el);
    await flush();
    await el.updateComplete;
    expect(fetchCount).toBe(1);

    // Simulate navigating away (disconnect) then back (reconnect): the shell
    // swaps the active element, so a return must re-fetch the current file.
    el.remove();
    currentComment = "second";
    document.body.appendChild(el);
    await flush();
    await el.updateComplete;

    expect(fetchCount).toBe(2);
    expect(el.state?.keys?.[0]?.comment).toBe("second");
    el.remove();
  });

  test("re-fetches when the tab becomes visible again", async () => {
    const el = document.createElement("security-view") as SecurityView;
    document.body.appendChild(el);
    await flush();
    await el.updateComplete;
    expect(fetchCount).toBe(1);

    currentComment = "refreshed";
    document.dispatchEvent(new Event("visibilitychange"));
    await flush();
    await el.updateComplete;

    // happy-dom reports document.visibilityState "visible" by default, so the
    // handler reloads on the event.
    expect(fetchCount).toBe(2);
    expect(el.state?.keys?.[0]?.comment).toBe("refreshed");
    el.remove();
  });

  test("removes the visibility listener on disconnect", async () => {
    const el = document.createElement("security-view") as SecurityView;
    document.body.appendChild(el);
    await flush();
    await el.updateComplete;
    expect(fetchCount).toBe(1);

    el.remove();
    // After disconnect a visibility change must not trigger another fetch.
    document.dispatchEvent(new Event("visibilitychange"));
    await flush();
    expect(fetchCount).toBe(1);
  });
});

import { test, expect, afterEach } from "bun:test";
import { api, setUnauthorizedHandler } from "../src/api.ts";

const realFetch = globalThis.fetch;

// stubFetch replaces global fetch with one that returns a single canned
// Response (status + body). Restored in afterEach.
function stubFetch(status: number, body: string): void {
  globalThis.fetch = (async () => new Response(body, { status })) as typeof fetch;
}

afterEach(() => {
  globalThis.fetch = realFetch;
  setUnauthorizedHandler(null);
});

test("401 invokes the registered handler AND still throws", async () => {
  let fired = 0;
  setUnauthorizedHandler(() => {
    fired++;
  });
  stubFetch(401, "");
  await expect(api.system()).rejects.toThrow();
  expect(fired).toBe(1);
});

test("200 does not invoke the handler", async () => {
  let fired = 0;
  setUnauthorizedHandler(() => {
    fired++;
  });
  stubFetch(200, JSON.stringify({ ok: true }));
  await api.system();
  expect(fired).toBe(0);
});

test("a non-401 error still throws and does not invoke the handler", async () => {
  let fired = 0;
  setUnauthorizedHandler(() => {
    fired++;
  });
  stubFetch(400, JSON.stringify({ error: "bad request" }));
  await expect(api.system()).rejects.toThrow("bad request");
  expect(fired).toBe(0);
});

test("setUnauthorizedHandler(null) unregisters", async () => {
  let fired = 0;
  setUnauthorizedHandler(() => {
    fired++;
  });
  setUnauthorizedHandler(null);
  stubFetch(401, "");
  await expect(api.system()).rejects.toThrow();
  expect(fired).toBe(0);
});

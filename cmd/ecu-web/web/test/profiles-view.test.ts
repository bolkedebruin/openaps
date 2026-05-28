import { test, expect, describe, beforeEach, afterEach } from "bun:test";
import "../src/views/profiles-view.ts";
import type { ProfilesView } from "../src/views/profiles-view.ts";
import type { OverlayDraft } from "../src/components/local-site-profile-form.ts";
import type { ProfilesState, SettingsResult } from "../src/api.ts";

// fakeProfilesState is the minimal shape <profiles-view> needs from /api/profiles.
const fakeProfilesState: ProfilesState = {
  base: { active_base: "", reconciler_ready: true, profiles: [] },
  base_defaults: {},
  overlays: [],
  inverters: [
    { uid: "704000006835", model: "DS3", model_code: 0x20, writable_codes: ["CB"], current: {} },
  ],
  params: [
    {
      aps_code: "CB",
      long_name: "Over_frequency_Watt_Low_set",
      unit: "Hz",
      group: "CrvSet",
      model: 134,
    },
  ],
  conflict_rules: [],
};

const fakeSettings: SettingsResult = { settings: { inverter_names: {} } as SettingsResult["settings"] };

type FetchInit = RequestInit | undefined;
type FetchHandler = (url: string, init: FetchInit) => Promise<Response>;

let originalFetch: typeof globalThis.fetch;
let originalConfirm: typeof globalThis.confirm;
let lastPutBody: unknown = null;

function installFetch(handler: FetchHandler) {
  globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === "string" ? input : input.toString();
    return handler(url, init);
  }) as typeof globalThis.fetch;
}

function jsonResp(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

beforeEach(() => {
  originalFetch = globalThis.fetch;
  originalConfirm = globalThis.confirm;
  globalThis.confirm = (() => true) as typeof globalThis.confirm;
  lastPutBody = null;
});

afterEach(() => {
  globalThis.fetch = originalFetch;
  globalThis.confirm = originalConfirm;
});

async function mountAndLoad(): Promise<ProfilesView> {
  const el = document.createElement("profiles-view") as ProfilesView;
  document.body.appendChild(el);
  // connectedCallback fired; wait for async load() + render.
  await new Promise((r) => setTimeout(r, 0));
  await el.updateComplete;
  await el.updateComplete;
  return el;
}

describe("<profiles-view>", () => {
  test("save overlay shows queued banner referencing the events log", async () => {
    installFetch(async (url, init) => {
      if (url === "/api/profiles") return jsonResp(200, fakeProfilesState);
      if (url === "/api/settings") {
        const r = {
          ecu_id: "",
          mac: "",
          pan_override: "",
          zigbee_type: "",
          inverter_names: {},
        };
        return jsonResp(200, r);
      }
      if (url === "/api/profiles/overlay" && init?.method === "PUT") {
        lastPutBody = JSON.parse(String(init.body));
        return jsonResp(202, {
          id: "victron-shift",
          status: "queued",
          uids: ["704000006835", "806000042582"],
        });
      }
      throw new Error("unexpected fetch: " + url);
    });

    const el = await mountAndLoad();

    // Dispatch the save event directly (bypassing the modal form interaction).
    const draft: OverlayDraft = {
      id: "victron-shift",
      uids: ["704000006835", "806000042582"],
      points: [{ aps_code: "CB", value: 50.3 }],
    };
    el.dispatchEvent(new CustomEvent<OverlayDraft>("save", { detail: draft })); // unused
    // The save handler lives on profiles-view via @save listener wired to the
    // child form; invoke the underlying handler by dispatching the event from
    // inside the shadow tree the way local-site-profile-form would.
    // Simpler: call the bound handler property directly via a public hook —
    // construct + dispatch as if the form emitted it.
    const innerEvent = new CustomEvent<OverlayDraft>("save", {
      detail: draft,
      bubbles: true,
      composed: true,
    });
    // The profiles-view template renders <local-site-profile-form> only after
    // newProfile() is called; trigger the path the form would.
    el["editing"] = { id: "victron-shift", uids: [], points: [] };
    await el.updateComplete;
    const form = el.shadowRoot!.querySelector("local-site-profile-form");
    expect(form).toBeTruthy();
    form!.dispatchEvent(innerEvent);

    // Wait for the async handler to flush.
    await new Promise((r) => setTimeout(r, 10));
    await el.updateComplete;
    await el.updateComplete;

    expect(lastPutBody).toMatchObject({ id: "victron-shift" });
    // Notice banner is rendered with the queued copy referencing Events.
    const banner = el.shadowRoot!.querySelector(".banner.ok");
    expect(banner).toBeTruthy();
    const text = banner!.textContent ?? "";
    expect(text).toContain("victron-shift");
    expect(text).toContain("queued");
    expect(text).toContain("2 inverters");
    expect(text).toContain("Events");
    // No red error banner on a successful queue.
    expect(el.shadowRoot!.querySelector(".banner.err")).toBeNull();
  });

  test("save overlay shows error banner on transport failure", async () => {
    installFetch(async (url, init) => {
      if (url === "/api/profiles") return jsonResp(200, fakeProfilesState);
      if (url === "/api/settings") {
        return jsonResp(200, {
          ecu_id: "",
          mac: "",
          pan_override: "",
          zigbee_type: "",
          inverter_names: {},
        });
      }
      if (url === "/api/profiles/overlay" && init?.method === "PUT") {
        return new Response("inv-driver down", { status: 502 });
      }
      throw new Error("unexpected fetch: " + url);
    });

    const el = await mountAndLoad();
    el["editing"] = { id: "x", uids: [], points: [] };
    await el.updateComplete;
    const form = el.shadowRoot!.querySelector("local-site-profile-form");
    form!.dispatchEvent(
      new CustomEvent<OverlayDraft>("save", {
        detail: { id: "x", uids: ["704000006835"], points: [{ aps_code: "CB", value: 50 }] },
        bubbles: true,
        composed: true,
      }),
    );

    await new Promise((r) => setTimeout(r, 10));
    await el.updateComplete;

    const err = el.shadowRoot!.querySelector(".banner.err");
    expect(err).toBeTruthy();
    expect(err!.textContent ?? "").toContain("inv-driver down");
  });
});

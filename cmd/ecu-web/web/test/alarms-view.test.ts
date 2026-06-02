import { test, expect, describe, beforeEach, afterEach } from "bun:test";
import "../src/views/alarms-view.ts";
import type { AlarmsView } from "../src/views/alarms-view.ts";
import type { Event, EventsResult, Fleet, Inverter } from "../src/api.ts";

function inv(over: Partial<Inverter> = {}): Inverter {
  return {
    uid: "999900000001", short_addr: 1, model: "DS3", model_code: 0x20, phase: 1,
    sw_version: 3067, online: true, last_seen_ms: Date.now(), age_s: 2,
    active_power_w: 0, nameplate_w: 750, load_pct: 0, grid_v: 0, bus_v: 0,
    freq_hz: 0, reactive_var: 0, rssi: 0, lqi: 0, panels: [],
    ...over,
  };
}

function fleet(inverters: Inverter[]): Fleet {
  return {
    ts_ms: Date.now(), nameplate_total_w: 0, inverter_count: inverters.length,
    online_count: 0, active_power_w: 0, lifetime_wh: 0, today_wh: 0, month_wh: 0, year_wh: 0,
    inverters,
  };
}

async function mount(f: Fleet | null): Promise<AlarmsView> {
  const el = document.createElement("alarms-view") as AlarmsView;
  el.fleet = f;
  document.body.appendChild(el);
  // wait the initial async loadRecent() + render
  await el.updateComplete;
  await new Promise((r) => setTimeout(r, 0));
  await el.updateComplete;
  return el;
}

// Patch api.events on the shared api object so the view's load() goes
// through the stub. Restored after each test (settings-form.test pattern).
let restoreEvents: (() => void) | null = null;
let lastQuery: { kind?: string; since_ms?: number; limit?: number } | null = null;

async function stubEvents(impl: () => Promise<EventsResult>) {
  const mod = await import("../src/api.ts");
  const orig = mod.api.events;
  mod.api.events = (q = {}) => {
    lastQuery = q;
    return impl();
  };
  restoreEvents = () => {
    mod.api.events = orig;
  };
}

beforeEach(() => {
  lastQuery = null;
});

afterEach(() => {
  if (restoreEvents) restoreEvents();
  restoreEvents = null;
  document.body.querySelectorAll("alarms-view").forEach((n) => n.remove());
});

describe("<alarms-view> live faults section", () => {
  test("renders an active fault chip exactly as today", async () => {
    await stubEvents(async () => ({ events: [] }));
    const el = await mount(fleet([inv({ faults: { over_freq_stage1: true } })]));
    const text = el.shadowRoot!.textContent ?? "";
    expect(text).toContain("Over Freq Stage1");
    // severity badge
    const sev = el.shadowRoot!.querySelector(".row.fault .sev");
    expect(sev?.textContent?.trim()).toBe("fault");
    // model + uid on the row
    expect(text).toContain("DS3");
    expect(text).toContain("999900000001");
  });

  test("shows the ok state when no faults and inverter online", async () => {
    await stubEvents(async () => ({ events: [] }));
    const el = await mount(fleet([inv()]));
    expect(el.shadowRoot!.textContent ?? "").toContain("No active alarms");
  });
});

describe("<alarms-view> Recent (24h) section", () => {
  function ev(over: Partial<Event> = {}): Event {
    return {
      id: 1,
      ts_ms: Date.now() - 60_000,
      kind: "fault_raised",
      severity: "warn",
      inverter_uid: "999900000001",
      detail: "over_freq_stage1 freq=50.42 v=248.1 w=0",
      by: "inv-driver",
      ...over,
    };
  }

  test("renders a fault_raised row via <events-table> with the By cell", async () => {
    await stubEvents(async () => ({ events: [ev()] }));
    const el = await mount(fleet([inv()]));

    // header is present
    expect(el.shadowRoot!.textContent ?? "").toContain("Recent (24h)");

    const table = el.shadowRoot!.querySelector("events-table");
    expect(table).not.toBeNull();
    const inner = (table as HTMLElement).shadowRoot!;
    const text = inner.textContent ?? "";
    expect(text).toContain("inv-driver");
    expect(text).toContain("999900000001");
    expect(text).toContain("over_freq_stage1");
    // By column header exists
    const headers = Array.from(inner.querySelectorAll("th")).map((h) => h.textContent?.trim());
    expect(headers).toContain("By");
  });

  test("queries /api/events with kind=fault_raised, since_ms ≈ 24h ago, limit=100", async () => {
    await stubEvents(async () => ({ events: [] }));
    await mount(fleet([inv()]));
    expect(lastQuery).not.toBeNull();
    expect(lastQuery!.kind).toBe("fault_raised");
    expect(lastQuery!.limit).toBe(100);
    const ago = Date.now() - (lastQuery!.since_ms ?? 0);
    // window is ~24h, allow generous slack
    expect(ago).toBeGreaterThan(23 * 3600 * 1000);
    expect(ago).toBeLessThan(25 * 3600 * 1000);
  });

  test("empty events render the 'No fault events in the last 24h.' message", async () => {
    await stubEvents(async () => ({ events: [] }));
    const el = await mount(fleet([inv()]));
    expect(el.shadowRoot!.textContent ?? "").toContain("No fault events in the last 24h.");
    // no events-table when empty
    expect(el.shadowRoot!.querySelector("events-table")).toBeNull();
  });

  test("fetch failure shows a muted error and leaves the live section intact", async () => {
    await stubEvents(async () => {
      throw new Error("boom");
    });
    const el = await mount(fleet([inv({ faults: { over_freq_stage1: true } })]));
    const text = el.shadowRoot!.textContent ?? "";
    expect(text).toContain("boom");
    // live chip still rendered
    expect(el.shadowRoot!.querySelector(".row.fault")).not.toBeNull();
    expect(text).toContain("Over Freq Stage1");
  });

  test("refetches when the fleet prop updates (SSE-driven)", async () => {
    let calls = 0;
    await stubEvents(async () => {
      calls++;
      return { events: [] };
    });
    const el = await mount(fleet([inv()]));
    const before = calls;
    el.fleet = fleet([inv({ active_power_w: 42 })]);
    await el.updateComplete;
    await new Promise((r) => setTimeout(r, 0));
    expect(calls).toBeGreaterThan(before);
  });
});

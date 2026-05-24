import { test, expect, describe } from "bun:test";
import "../src/views/inverters-view.ts";
import type { InvertersView } from "../src/views/inverters-view.ts";
import type { Fleet, Inverter } from "../src/api.ts";

function inv(over: Partial<Inverter> = {}): Inverter {
  return {
    uid: "704000006835", short_addr: 1, model: "DS3", model_code: 0x20, phase: 1,
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

async function mount(f: Fleet): Promise<InvertersView> {
  const el = document.createElement("inverters-view") as InvertersView;
  el.fleet = f;
  document.body.appendChild(el);
  await el.updateComplete;
  return el;
}

describe("<inverters-view>", () => {
  test("has a Firmware column between Model and Status", async () => {
    const el = await mount(fleet([inv()]));
    const headers = Array.from(el.shadowRoot!.querySelectorAll("th")).map((h) => h.textContent?.trim());
    const mi = headers.indexOf("Model");
    const fi = headers.indexOf("Firmware");
    const si = headers.indexOf("Status");
    expect(fi).toBeGreaterThan(-1);
    expect(fi).toBe(mi + 1);
    expect(si).toBe(fi + 1);
  });

  test("renders the firmware version value", async () => {
    const el = await mount(fleet([inv({ sw_version: 5203 })]));
    expect(el.shadowRoot?.querySelector(".fw")?.textContent?.trim()).toBe("5203");
  });

  test("shows a dash when firmware is unknown (0)", async () => {
    const el = await mount(fleet([inv({ sw_version: 0 })]));
    expect(el.shadowRoot?.querySelector(".fw")?.textContent?.trim()).toBe("—");
  });
});

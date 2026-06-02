import { test, expect, describe } from "bun:test";
import "../src/components/cap-bar.ts";
import type { CapBar } from "../src/components/cap-bar.ts";
import type { Inverter } from "../src/api.ts";

function sample(over: Partial<Inverter> = {}): Inverter {
  return {
    uid: "999900000003", short_addr: 1, model: "QS1A", model_code: 0x18, phase: 1, sw_version: 1,
    online: true, last_seen_ms: Date.now(), age_s: 1, active_power_w: 800, nameplate_w: 1600,
    load_pct: 50, grid_v: 230, bus_v: 360, freq_hz: 50, reactive_var: 0, rssi: 70, lqi: 200,
    panels: [{ index: 0, dc_v: 35, dc_a: 5, w: 175 }], ...over,
  };
}

async function mount(inv: Inverter): Promise<CapBar> {
  const el = document.createElement("cap-bar") as CapBar;
  el.inverter = inv;
  document.body.appendChild(el);
  await el.updateComplete;
  return el;
}

describe("<cap-bar>", () => {
  test("renders the red caret + line and live fill", async () => {
    const el = await mount(sample({ load_pct: 95 }));
    expect(el.shadowRoot?.querySelector(".caret")).not.toBeNull();
    expect(el.shadowRoot?.querySelector(".capline")).not.toBeNull();
    expect(el.shadowRoot?.querySelector(".fill.high")).not.toBeNull();
  });

  test("cap value comes from the read-back (DA): 375/500 × 1600 = 1.20 kW", async () => {
    const el = await mount(sample({ protection: { DA: 375 } }));
    expect(el.shadowRoot?.querySelector(".capval")?.textContent).toContain("1.20 kW");
  });

  test("no read-back → shows nameplate (uncapped)", async () => {
    const el = await mount(sample({ nameplate_w: 1600 }));
    expect(el.shadowRoot?.querySelector(".capval")?.textContent).toContain("1.60 kW");
  });

  test("caret position reflects the cap fraction of nameplate", async () => {
    const el = await mount(sample({ protection: { DA: 250 } })); // 250/500 = 50%
    const caret = el.shadowRoot?.querySelector(".caret") as HTMLElement;
    expect(caret.style.left).toBe("50%");
  });
});

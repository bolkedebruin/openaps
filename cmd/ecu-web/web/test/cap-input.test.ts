import { test, expect, describe } from "bun:test";
import "../src/components/cap-input.ts";
import type { CapInput } from "../src/components/cap-input.ts";
import type { Inverter } from "../src/api.ts";

function sample(over: Partial<Inverter> = {}): Inverter {
  return {
    uid: "806000042582", short_addr: 1, model: "QS1A", model_code: 0x18, phase: 1, sw_version: 1,
    online: true, last_seen_ms: Date.now(), age_s: 1, active_power_w: 800, nameplate_w: 1600,
    load_pct: 50, grid_v: 230, bus_v: 360, freq_hz: 50, reactive_var: 0, rssi: 70, lqi: 200,
    panels: [{ index: 0, dc_v: 35, dc_a: 5, w: 175 }], ...over,
  };
}

async function mount(inv: Inverter): Promise<CapInput> {
  const el = document.createElement("cap-input") as CapInput;
  el.inverter = inv;
  document.body.appendChild(el);
  await el.updateComplete;
  return el;
}

describe("<cap-input>", () => {
  test("input shows the read-back cap (DA 375/500 × 1600 = 1200) and nameplate suffix", async () => {
    const el = await mount(sample({ protection: { DA: 375 } }));
    const input = el.shadowRoot?.querySelector("input") as HTMLInputElement;
    expect(input.value).toBe("1200");
    expect(el.shadowRoot?.querySelector(".max")?.textContent).toContain("/ 1600 W");
  });

  test("no read-back → input shows nameplate (uncapped)", async () => {
    const el = await mount(sample({ nameplate_w: 1600 }));
    const input = el.shadowRoot?.querySelector("input") as HTMLInputElement;
    expect(input.value).toBe("1600");
  });

  test("offline inverter disables the input", async () => {
    const el = await mount(sample({ online: false }));
    const input = el.shadowRoot?.querySelector("input") as HTMLInputElement;
    expect(input.disabled).toBe(true);
  });
});

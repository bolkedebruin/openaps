import { test, expect, describe } from "bun:test";
import "../src/components/inverter-card.ts";
import type { InverterCard } from "../src/components/inverter-card.ts";
import type { Inverter } from "../src/api.ts";

function sample(over: Partial<Inverter> = {}): Inverter {
  return {
    uid: "aabbccddeeff",
    short_addr: 1,
    model: "QS1A",
    model_code: 0x18,
    phase: 1,
    sw_version: 1,
    online: true,
    last_seen_ms: Date.now(),
    age_s: 3,
    active_power_w: 800,
    nameplate_w: 1600,
    load_pct: 50,
    grid_v: 230.5,
    bus_v: 360,
    freq_hz: 50.01,
    reactive_var: 0,
    rssi: 70,
    lqi: 200,
    panels: [
      { index: 0, dc_v: 35.2, dc_a: 5.1, w: 179.5 },
      { index: 1, dc_v: 34.9, dc_a: 5.0, w: 174.5 },
    ],
    ...over,
  };
}

async function mount(inv: Inverter, profile = ""): Promise<InverterCard> {
  const el = document.createElement("inverter-card") as InverterCard;
  el.inverter = inv;
  el.profile = profile;
  document.body.appendChild(el);
  await el.updateComplete;
  return el;
}

function text(el: HTMLElement): string {
  return el.shadowRoot?.textContent?.replace(/\s+/g, " ").trim() ?? "";
}

describe("<inverter-card>", () => {
  test("renders model, uid, power vs cap", async () => {
    const el = await mount(sample());
    const t = text(el);
    expect(t).toContain("QS1A");
    expect(t).toContain("aabbccddeeff");
    expect(t).toContain("800 W");
    expect(t).toContain("1.60 kW"); // nameplate, formatted
    expect(t).toContain("50%");
  });

  test("renders the cap-bar control", async () => {
    const el = await mount(sample());
    const cb = el.shadowRoot?.querySelector("cap-bar") as HTMLElement & { inverter?: Inverter };
    expect(cb).not.toBeNull();
    expect(cb?.inverter?.uid).toBe("aabbccddeeff");
  });

  test("shows online state and panels", async () => {
    const el = await mount(sample());
    expect(el.shadowRoot?.querySelector(".dot.on")).not.toBeNull();
    expect(el.shadowRoot?.querySelectorAll(".panel").length).toBe(2);
    expect(text(el)).toContain("230.5 V");
    expect(text(el)).toContain("50.01 Hz");
  });

  test("offline inverter has off dot", async () => {
    const el = await mount(sample({ online: false }));
    expect(el.shadowRoot?.querySelector(".dot.off")).not.toBeNull();
    expect(text(el)).toContain("offline");
  });

  test("active faults render as chips", async () => {
    const el = await mount(sample({ faults: { ac_over_volt_stage1: true, dc_bus_fault: false } }));
    const chips = el.shadowRoot?.querySelectorAll(".chip") ?? [];
    expect(chips.length).toBe(1);
    expect(chips[0]?.textContent).toContain("Ac Over Volt Stage1");
  });

  test("no faults -> no chips", async () => {
    const el = await mount(sample());
    expect((el.shadowRoot?.querySelectorAll(".chip") ?? []).length).toBe(0);
  });

  test("active custom profile shows a badge with its name", async () => {
    const el = await mount(sample(), "victron-shift");
    const badge = el.shadowRoot?.querySelector(".profile");
    expect(badge).not.toBeNull();
    expect(badge?.textContent).toContain("victron-shift");
  });

  test("no custom profile -> no badge", async () => {
    const el = await mount(sample());
    expect(el.shadowRoot?.querySelector(".profile")).toBeNull();
  });

  test("single-phase shows three leg buttons with the assigned leg selected", async () => {
    const el = await mount(sample({ model_code: 0x18, phase: 2 }));
    expect(el.shadowRoot?.querySelectorAll(".legbtn").length).toBe(3);
    const selected = el.shadowRoot?.querySelectorAll(".legbtn.sel");
    expect(selected?.length).toBe(1);
    expect(selected?.[0]?.textContent?.trim()).toBe("L2");
    expect(el.shadowRoot?.querySelector(".three")).toBeNull();
  });

  test("three-phase (QT2) shows a read-only label and no leg buttons", async () => {
    const el = await mount(sample({ model: "QT2", model_code: 0x32, three_phase: true }));
    expect(el.shadowRoot?.querySelector(".three")).not.toBeNull();
    expect(el.shadowRoot?.querySelectorAll(".legbtn").length).toBe(0);
  });
});

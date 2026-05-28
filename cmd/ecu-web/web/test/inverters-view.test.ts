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

  test("has an Encryption column for the link-encryption badge", async () => {
    const el = await mount(fleet([inv()]));
    const headers = Array.from(el.shadowRoot!.querySelectorAll("th")).map((h) => h.textContent?.trim());
    expect(headers).toContain("Encryption");
    expect(headers).not.toContain("Link");
  });

  test("badge shows plaintext for encrypted=false and AES for true in the normal list", async () => {
    const elPlain = await mount(fleet([inv({ encrypted: false })]));
    const plainBadge = elPlain.shadowRoot?.querySelector("tbody .enc");
    expect(plainBadge?.classList.contains("enc-warn")).toBe(true);
    expect(plainBadge?.textContent?.toLowerCase()).toContain("plaintext");

    const elAes = await mount(fleet([inv({ encrypted: true })]));
    const aesBadge = elAes.shadowRoot?.querySelector("tbody .enc");
    expect(aesBadge?.classList.contains("enc-ok")).toBe(true);
    expect(aesBadge?.textContent).toContain("AES");
  });

  test("encrypted=true renders an AES lock badge", async () => {
    const el = await mount(fleet([inv({ encrypted: true })]));
    const badge = el.shadowRoot?.querySelector(".enc");
    expect(badge?.classList.contains("enc-ok")).toBe(true);
    expect(badge?.textContent).toContain("AES");
  });

  test("encrypted=false renders a plaintext warning badge", async () => {
    const el = await mount(fleet([inv({ encrypted: false })]));
    const badge = el.shadowRoot?.querySelector(".enc");
    expect(badge?.classList.contains("enc-warn")).toBe(true);
    expect(badge?.textContent?.toLowerCase()).toContain("plaintext");
  });

  test("missing encrypted renders the neutral unknown badge", async () => {
    const el = await mount(fleet([inv()])); // encrypted undefined
    const badge = el.shadowRoot?.querySelector(".enc");
    expect(badge?.classList.contains("enc-unknown")).toBe(true);
  });

  test("each row has a Replace button", async () => {
    const el = await mount(fleet([inv()]));
    const btn = el.shadowRoot?.querySelector("button.replace") as HTMLButtonElement;
    expect(btn).not.toBeNull();
    expect(btn.textContent?.trim()).toBe("Replace");
  });

  test("renders the scan panel and fleet re-key control", async () => {
    const el = await mount(fleet([inv()]));
    expect(el.shadowRoot?.querySelector("pairing-scan-panel")).not.toBeNull();
    expect(el.shadowRoot?.querySelector("button.rekey-btn")).not.toBeNull();
    expect(el.shadowRoot?.querySelector("pairing-progress-drawer")).not.toBeNull();
  });
});

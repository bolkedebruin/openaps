import { test, expect } from "bun:test";
import { capFloorW, capCeilW, readbackCapW } from "../src/power.ts";
import type { Inverter } from "../src/api.ts";

function inv(p: Partial<Inverter>): Inverter {
  return {
    uid: "x", short_addr: 0, model: "QS1A", model_code: 0x18, phase: 1, sw_version: 0,
    online: true, last_seen_ms: 0, age_s: 0, active_power_w: 0, nameplate_w: 1600,
    load_pct: 0, grid_v: 0, bus_v: 0, freq_hz: 0, reactive_var: 0, rssi: 0, lqi: 0,
    panels: [], ...p,
  } as Inverter;
}

test("capCeilW is the nameplate", () => {
  expect(capCeilW(inv({ nameplate_w: 1600 }))).toBe(1600);
  expect(capCeilW(inv({ nameplate_w: 750 }))).toBe(750);
});

test("capFloorW = 20/500 of nameplate (per-panel floor as a fraction)", () => {
  expect(capFloorW(inv({ nameplate_w: 1600 }))).toBe(64); // 1600 * 20/500
  expect(capFloorW(inv({ nameplate_w: 750 }))).toBe(30); // 750 * 20/500
});

test("readbackCapW = (DA/500) × nameplate, else undefined", () => {
  // uncapped: DA 500 → full nameplate
  expect(readbackCapW(inv({ nameplate_w: 1600, protection: { DA: 500 } }))).toBe(1600);
  // curtailed: DA 375 of 500 = 75% → 1200
  expect(readbackCapW(inv({ nameplate_w: 1600, protection: { DA: 375 } }))).toBe(1200);
  // DS3 250/500 = 50% → 375
  expect(readbackCapW(inv({ nameplate_w: 750, protection: { DA: 250 } }))).toBe(375);
  expect(readbackCapW(inv({ nameplate_w: 1600 }))).toBeUndefined();
});

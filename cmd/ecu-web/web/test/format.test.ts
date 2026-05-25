import { test, expect, describe } from "bun:test";
import {
  fmtW,
  fmtWh,
  fmtPct,
  fmtV,
  fmtHz,
  fmtA,
  loadClass,
  ageLabel,
  humanizeFault,
  faultLabels,
  fmtNum,
} from "../src/format.ts";

describe("fmtNum", () => {
  test("rounds device floats and drops trailing zeros", () => {
    expect(fmtNum(16.569348154600167)).toBe("16.569");
    expect(fmtNum(52.00002496001198)).toBe("52");
    expect(fmtNum(50.2)).toBe("50.2");
    expect(fmtNum(49.75000621875078)).toBe("49.75");
    expect(fmtNum(0)).toBe("0");
  });
  test("non-finite -> empty", () => {
    expect(fmtNum(NaN)).toBe("");
  });
});

describe("fmtW", () => {
  test("watts below 1k", () => expect(fmtW(750)).toBe("750 W"));
  test("rounds", () => expect(fmtW(179.6)).toBe("180 W"));
  test("kW above 1k", () => expect(fmtW(1500)).toBe("1.50 kW"));
  test("non-finite", () => expect(fmtW(NaN)).toBe("—"));
});

describe("fmtWh", () => {
  test("Wh", () => expect(fmtWh(500)).toBe("500 Wh"));
  test("kWh", () => expect(fmtWh(4200)).toBe("4.20 kWh"));
  test("MWh", () => expect(fmtWh(2_500_000)).toBe("2.50 MWh"));
});

describe("scalar formatters", () => {
  test("pct", () => expect(fmtPct(49.6)).toBe("50%"));
  test("pct nan", () => expect(fmtPct(NaN)).toBe("—"));
  test("volts", () => expect(fmtV(230.5)).toBe("230.5 V"));
  test("volts zero -> dash", () => expect(fmtV(0)).toBe("—"));
  test("hz", () => expect(fmtHz(50.012)).toBe("50.01 Hz"));
  test("amps", () => expect(fmtA(5.1)).toBe("5.10 A"));
});

describe("loadClass", () => {
  test("idle at zero/negative", () => {
    expect(loadClass(0)).toBe("idle");
    expect(loadClass(-5)).toBe("idle");
  });
  test("low", () => expect(loadClass(20)).toBe("low"));
  test("mid", () => expect(loadClass(60)).toBe("mid"));
  test("high", () => expect(loadClass(90)).toBe("high"));
  test("boundaries", () => {
    expect(loadClass(40)).toBe("mid");
    expect(loadClass(85)).toBe("high");
  });
});

describe("ageLabel", () => {
  test("seconds", () => expect(ageLabel(12)).toBe("12s ago"));
  test("minutes", () => expect(ageLabel(150)).toBe("3m ago"));
  test("hours", () => expect(ageLabel(7200)).toBe("2h ago"));
  test("invalid", () => expect(ageLabel(-1)).toBe("—"));
});

describe("faults", () => {
  test("humanize", () => expect(humanizeFault("ac_over_volt_stage1")).toBe("Ac Over Volt Stage1"));
  test("labels only active", () => {
    expect(faultLabels({ grid_relay_fault: true, dc_bus_fault: false })).toEqual(["Grid Relay Fault"]);
  });
  test("undefined -> empty", () => expect(faultLabels(undefined)).toEqual([]));
});

import { test, expect, describe } from "bun:test";
import { paramLabel, paramDesc, conflicts, prettifyName } from "../src/param-docs.ts";
import type { ConflictRule } from "../src/api.ts";

const RULES: ConflictRule[] = [
  { left: "CB", right: "CC", message: "Over-frequency Watt: the start point (CB) must be below the end point (CC)." },
  { left: "CA", right: "AF", message: "Over-frequency curtailment start (CA) must be below the over-frequency trip (AF)." },
];

describe("param-docs labels", () => {
  test("known code gets a friendly label + description", () => {
    expect(paramLabel("AB", "min10_Over_average_voltage")).toBe("10-minute mean overvoltage");
    expect(paramDesc("AB")).toContain("10-minute");
  });
  test("unknown code falls back to a prettified long_name", () => {
    expect(paramLabel("ZZ", "some_raw_name")).toBe("Some Raw Name");
    expect(prettifyName("over_voltage_slow", "AD")).toBe("Over Voltage Slow");
    expect(paramDesc("ZZ")).toBe("");
  });
});

describe("conflicts", () => {
  const eff = (m: Record<string, number>) => (c: string) => m[c];

  test("flags slope start past end (CB >= CC)", () => {
    expect(conflicts(RULES, eff({ CB: 51, CC: 50 }))).toContain(
      "Over-frequency Watt: the start point (CB) must be below the end point (CC).",
    );
  });
  test("no conflict when ordered", () => {
    expect(conflicts(RULES, eff({ CB: 50.2, CC: 51.5 }))).toEqual([]);
  });
  test("ignores rules where a value is unknown", () => {
    expect(conflicts(RULES, eff({ CB: 51 }))).toEqual([]); // CC missing -> rule skipped
  });
  test("flags curtailment start above over-frequency trip (CA >= AF)", () => {
    expect(conflicts(RULES, eff({ CA: 52.5, AF: 52.0 })).some((m) => m.includes("curtailment start"))).toBe(true);
  });
});

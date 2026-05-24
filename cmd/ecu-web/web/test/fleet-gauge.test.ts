import { test, expect, describe } from "bun:test";
import "../src/components/fleet-gauge.ts";
import "../src/components/stat-card.ts";
import type { FleetGauge } from "../src/components/fleet-gauge.ts";

async function mountGauge(power: number, cap: number): Promise<FleetGauge> {
  const el = document.createElement("fleet-gauge") as FleetGauge;
  el.power = power;
  el.cap = cap;
  document.body.appendChild(el);
  await el.updateComplete;
  return el;
}

describe("<fleet-gauge>", () => {
  test("shows power and percent of cap", async () => {
    const el = await mountGauge(800, 1600);
    const t = el.shadowRoot?.textContent?.replace(/\s+/g, " ") ?? "";
    expect(t).toContain("800 W");
    expect(t).toContain("50%");
  });

  test("idle when no output", async () => {
    const el = await mountGauge(0, 1600);
    expect(el.shadowRoot?.querySelector(".arc.idle")).not.toBeNull();
  });

  test("high when near cap", async () => {
    const el = await mountGauge(1550, 1600);
    expect(el.shadowRoot?.querySelector(".arc.high")).not.toBeNull();
  });

  test("zero cap does not divide by zero", async () => {
    const el = await mountGauge(500, 0);
    const t = el.shadowRoot?.textContent ?? "";
    expect(t).toContain("0%");
  });
});

describe("<stat-card>", () => {
  test("renders label and value", async () => {
    const el = document.createElement("stat-card") as HTMLElement & {
      label: string;
      value: string;
      updateComplete: Promise<boolean>;
    };
    el.label = "Today";
    el.value = "4.20 kWh";
    document.body.appendChild(el);
    await el.updateComplete;
    const t = el.shadowRoot?.textContent ?? "";
    expect(t).toContain("Today");
    expect(t).toContain("4.20 kWh");
  });
});

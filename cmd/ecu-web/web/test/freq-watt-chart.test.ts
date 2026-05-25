import { test, expect, describe } from "bun:test";
import "../src/components/freq-watt-chart.ts";
import type { FreqWattChart } from "../src/components/freq-watt-chart.ts";

async function mount(p: Partial<FreqWattChart> = {}): Promise<FreqWattChart> {
  const el = document.createElement("freq-watt-chart") as FreqWattChart;
  el.deadband = p.deadband;
  el.slope = p.slope;
  el.trip = p.trip;
  el.nominal = p.nominal ?? 50;
  document.body.appendChild(el);
  await el.updateComplete;
  return el;
}

describe("<freq-watt-chart>", () => {
  test("prompts when start/slope are unset", async () => {
    const el = await mount({});
    expect(el.shadowRoot?.querySelector("svg")).toBeNull();
    expect(el.shadowRoot?.textContent).toContain("start frequency and slope");
  });

  test("draws the droop curve and start marker when configured", async () => {
    const el = await mount({ deadband: 50.2, slope: 40 });
    expect(el.shadowRoot?.querySelector("svg")).not.toBeNull();
    expect(el.shadowRoot?.querySelector("polyline.curve")).not.toBeNull();
    expect(el.shadowRoot?.textContent).toContain("start 50.2");
    expect(el.shadowRoot?.textContent).toContain("40 %Pref/Hz");
  });

  test("marks the over-frequency trip", async () => {
    const el = await mount({ deadband: 50.2, slope: 40, trip: 51.5 });
    expect(el.shadowRoot?.querySelector("line.trip")).not.toBeNull();
    expect(el.shadowRoot?.textContent).toContain("trip 51.5");
  });
});

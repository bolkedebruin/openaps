import { test, expect, describe } from "bun:test";
import "../src/components/trip-line.ts";
import type { TripLine, TripMarker } from "../src/components/trip-line.ts";

async function mount(markers: TripMarker[], nominal?: number): Promise<TripLine> {
  const el = document.createElement("trip-line") as TripLine;
  el.unit = "V";
  el.nominal = nominal;
  el.markers = markers;
  document.body.appendChild(el);
  await el.updateComplete;
  return el;
}

describe("<trip-line>", () => {
  test("empty when no markers", async () => {
    const el = await mount([]);
    expect(el.shadowRoot?.querySelector("svg")).toBeNull();
    expect(el.shadowRoot?.textContent).toContain("No thresholds");
  });

  test("renders under/over markers and an operating band", async () => {
    const el = await mount(
      [
        { value: 196, label: "AC", kind: "under" },
        { value: 253, label: "AD", kind: "over" },
      ],
      230,
    );
    expect(el.shadowRoot?.querySelector("svg")).not.toBeNull();
    expect(el.shadowRoot?.querySelector("line.under")).not.toBeNull();
    expect(el.shadowRoot?.querySelector("line.over")).not.toBeNull();
    expect(el.shadowRoot?.querySelector("rect.band")).not.toBeNull();
    const t = el.shadowRoot?.textContent ?? "";
    expect(t).toContain("AC 196");
    expect(t).toContain("AD 253");
    expect(t).toContain("230 V");
  });
});

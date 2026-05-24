import { test, expect, describe } from "bun:test";
import "../src/components/power-chart.ts";
import { chartPaths, type PowerPoint, type PowerChart } from "../src/components/power-chart.ts";

describe("chartPaths", () => {
  test("fewer than 2 points -> empty", () => {
    expect(chartPaths([], 600, 160)).toEqual({ line: "", area: "", max: 0 });
    expect(chartPaths([{ t: 1, w: 5 }], 600, 160).line).toBe("");
  });

  test("scales x across span and y to peak", () => {
    const pts: PowerPoint[] = [
      { t: 0, w: 0 },
      { t: 100, w: 50 },
      { t: 200, w: 100 },
    ];
    const { line, area, max } = chartPaths(pts, 600, 160);
    expect(max).toBe(100);
    // first point: x=0, y=height (w=0 -> bottom)
    expect(line.startsWith("M0.0 160.0")).toBe(true);
    // last point: x=width(600), y=0 (peak -> top)
    expect(line).toContain("L600.0 0.0");
    // area closes back to the baseline and Z
    expect(area.endsWith("Z")).toBe(true);
    expect(area).toContain("L600.0 160 L0.0 160 Z");
  });

  test("flat nonzero series stays on a line (max=value)", () => {
    const { max } = chartPaths([{ t: 0, w: 42 }, { t: 10, w: 42 }], 600, 160);
    expect(max).toBe(42);
  });
});

describe("<power-chart>", () => {
  test("placeholder until 2 points", async () => {
    const el = document.createElement("power-chart") as PowerChart;
    el.points = [{ t: 1, w: 1 }];
    document.body.appendChild(el);
    await el.updateComplete;
    expect(el.shadowRoot?.textContent).toContain("Collecting");
  });

  test("renders svg paths with data", async () => {
    const el = document.createElement("power-chart") as PowerChart;
    el.points = [{ t: 0, w: 10 }, { t: 100, w: 800 }];
    document.body.appendChild(el);
    await el.updateComplete;
    expect(el.shadowRoot?.querySelector("path.line")).not.toBeNull();
    expect(el.shadowRoot?.querySelector("path.area")).not.toBeNull();
    expect(el.shadowRoot?.textContent).toContain("800 W");
  });

  test("hover shows a tooltip with the point's watts", async () => {
    const el = document.createElement("power-chart") as PowerChart;
    el.points = [{ t: 0, w: 10 }, { t: 100, w: 800 }];
    document.body.appendChild(el);
    await el.updateComplete;
    expect(el.shadowRoot?.querySelector(".tip")).toBeNull(); // no hover yet
    el.hoverIdx = 1;
    await el.updateComplete;
    const tip = el.shadowRoot?.querySelector(".tip");
    expect(tip).not.toBeNull();
    expect(tip?.textContent).toContain("800 W");
    expect(el.shadowRoot?.querySelector("circle.cursor")).not.toBeNull();
  });
});

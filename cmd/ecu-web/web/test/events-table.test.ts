import { test, expect, describe } from "bun:test";
import "../src/components/events-table.ts";
import type { EventsTable } from "../src/components/events-table.ts";
import type { Event } from "../src/api.ts";
import { severityClass } from "../src/format.ts";

async function mount(events: Event[]): Promise<EventsTable> {
  const el = document.createElement("events-table") as EventsTable;
  el.events = events;
  document.body.appendChild(el);
  await el.updateComplete;
  return el;
}

describe("severityClass", () => {
  test("buckets", () => {
    expect(severityClass("error")).toBe("err");
    expect(severityClass("critical")).toBe("err");
    expect(severityClass("warn")).toBe("warn");
    expect(severityClass("warning")).toBe("warn");
    expect(severityClass("info")).toBe("info");
    expect(severityClass("")).toBe("info");
  });
});

describe("<events-table>", () => {
  test("empty shows placeholder", async () => {
    const el = await mount([]);
    expect(el.shadowRoot?.textContent).toContain("No events recorded");
  });

  test("renders one row per event with humanised kind", async () => {
    const el = await mount([
      { id: 2, ts_ms: 2, kind: "decode_failed", severity: "warn", short_addr: 5, detail: "bad crc" },
      { id: 1, ts_ms: 1, kind: "paired", severity: "info", inverter_uid: "aabbccddeeff" },
    ]);
    const rows = el.shadowRoot?.querySelectorAll("tbody tr") ?? [];
    expect(rows.length).toBe(2);
    const t = el.shadowRoot?.textContent ?? "";
    expect(t).toContain("Decode Failed");
    expect(t).toContain("bad crc");
    expect(t).toContain("Paired");
    expect(t).toContain("aabbccddeeff");
  });

  test("severity badge gets the right class", async () => {
    const el = await mount([{ id: 1, ts_ms: 1, kind: "x", severity: "error", detail: "boom" }]);
    expect(el.shadowRoot?.querySelector(".sev.err")).not.toBeNull();
  });

  test("By column shows the originating backend (and a dash when absent)", async () => {
    const el = await mount([
      { id: 3, ts_ms: 3, kind: "power_cap_set", severity: "info", inverter_uid: "abc", by: "ecu-web" },
      { id: 2, ts_ms: 2, kind: "power_cap_set", severity: "info", inverter_uid: "def", by: "ecu-sunspec" },
      { id: 1, ts_ms: 1, kind: "decode_failed", severity: "warn" }, // legacy / no by
    ]);
    const headers = [...(el.shadowRoot?.querySelectorAll("th") ?? [])].map((h) => h.textContent?.trim());
    expect(headers).toContain("By");
    const byCells = [...(el.shadowRoot?.querySelectorAll("td.by") ?? [])].map((c) => c.textContent?.trim());
    expect(byCells).toEqual(["ecu-web", "ecu-sunspec", "—"]);
  });
});

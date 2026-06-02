import { test, expect, describe } from "bun:test";
import "../src/components/pairing-progress-drawer.ts";
import type { PairingProgressDrawer } from "../src/components/pairing-progress-drawer.ts";
import type { PairingStatus } from "../src/api.ts";

function status(over: Partial<PairingStatus> = {}): PairingStatus {
  return {
    op: "scan",
    stage: "bind",
    total: 3,
    done: 1,
    current_serial: "999900000001",
    substep: "get-short-addr",
    per_inverter: [
      { serial: "999900000001", short_addr: 0x1a, state: "binding", encrypted: true },
      { serial: "999900000003", state: "found", encrypted: false },
    ],
    ...over,
  };
}

async function mount(props: Partial<PairingProgressDrawer> = {}): Promise<PairingProgressDrawer> {
  const el = document.createElement("pairing-progress-drawer") as PairingProgressDrawer;
  el.open = props.open ?? true;
  el.status = props.status ?? null;
  el.aborting = props.aborting ?? false;
  document.body.appendChild(el);
  await el.updateComplete;
  return el;
}

describe("<pairing-progress-drawer>", () => {
  test("idle status shows the empty message", async () => {
    const el = await mount({ status: { op: "", stage: "", total: 0, done: 0 } });
    expect(el.shadowRoot?.querySelector(".empty")).not.toBeNull();
  });

  test("renders stage chips with the current stage active", async () => {
    const el = await mount({ status: status({ stage: "bind" }) });
    const active = el.shadowRoot?.querySelector(".stage.active");
    expect(active?.textContent?.trim()).toBe("Bind");
  });

  test("progress bar reflects done/total", async () => {
    const el = await mount({ status: status({ total: 4, done: 2 }) });
    const fill = el.shadowRoot?.querySelector(".bar > i") as HTMLElement;
    expect(fill.style.width).toBe("50%");
  });

  test("shows current serial and per-inverter sub-status rows", async () => {
    const el = await mount({ status: status() });
    const t = el.shadowRoot?.textContent ?? "";
    expect(t).toContain("999900000001");
    expect(t).toContain("binding");
    const rows = el.shadowRoot?.querySelectorAll("tbody tr") ?? [];
    expect(rows.length).toBe(2);
  });

  test("sweep info renders a telemetry-paused note", async () => {
    const el = await mount({ status: status({ sweep: { chan: 15, chan_lo: 11, chan_hi: 26 } }) });
    expect(el.shadowRoot?.querySelector(".sweep")?.textContent?.toLowerCase()).toContain("telemetry paused");
  });

  test("Safe-abort is enabled while active and emits abort", async () => {
    const el = await mount({ status: status({ stage: "migrate" }) });
    let fired = false;
    el.addEventListener("abort", () => (fired = true));
    const btn = el.shadowRoot?.querySelector("button.abort") as HTMLButtonElement;
    expect(btn.disabled).toBe(false);
    btn.click();
    expect(fired).toBe(true);
  });

  test("Safe-abort disabled when the op is terminal (done)", async () => {
    const el = await mount({ status: status({ stage: "done", done: 3 }) });
    const btn = el.shadowRoot?.querySelector("button.abort") as HTMLButtonElement;
    expect(btn.disabled).toBe(true);
  });

  test("close button disabled while active, enabled when terminal", async () => {
    const active = await mount({ status: status({ stage: "bind" }) });
    expect((active.shadowRoot?.querySelector("button.x") as HTMLButtonElement).disabled).toBe(true);

    const done = await mount({ status: status({ stage: "done" }) });
    const x = done.shadowRoot?.querySelector("button.x") as HTMLButtonElement;
    expect(x.disabled).toBe(false);
    let closed = false;
    done.addEventListener("close", () => (closed = true));
    x.click();
    expect(closed).toBe(true);
  });

  test("error stage surfaces the error text", async () => {
    const el = await mount({ status: status({ stage: "error", error: "commit failed" }) });
    expect(el.shadowRoot?.querySelector(".err")?.textContent).toContain("commit failed");
  });
});

import { test, expect, describe } from "bun:test";
import "../src/components/pairing-scan-panel.ts";
import type { PairingScanPanel } from "../src/components/pairing-scan-panel.ts";

async function mount(props: Partial<PairingScanPanel> = {}): Promise<PairingScanPanel> {
  const el = document.createElement("pairing-scan-panel") as PairingScanPanel;
  el.busy = props.busy ?? false;
  document.body.appendChild(el);
  await el.updateComplete;
  return el;
}

function buttons(el: PairingScanPanel): HTMLButtonElement[] {
  return Array.from(el.shadowRoot!.querySelectorAll(".toggle button")) as HTMLButtonElement[];
}

describe("<pairing-scan-panel>", () => {
  test("defaults to Fast and shows no telemetry-pause warning", async () => {
    const el = await mount();
    const [fast, slow] = buttons(el);
    expect(fast.classList.contains("sel")).toBe(true);
    expect(slow.classList.contains("sel")).toBe(false);
    expect(el.shadowRoot?.querySelector(".warn")).toBeNull();
  });

  test("selecting Slow shows the telemetry-pause warning", async () => {
    const el = await mount();
    buttons(el)[1].click();
    await el.updateComplete;
    const warn = el.shadowRoot?.querySelector(".warn");
    expect(warn).not.toBeNull();
    expect(warn?.textContent?.toLowerCase()).toContain("pauses telemetry");
  });

  test("Scan emits {slow} reflecting the toggle", async () => {
    const el = await mount();
    let got: { slow: boolean } | null = null;
    el.addEventListener("scan", (e) => (got = (e as CustomEvent).detail));
    buttons(el)[1].click(); // Slow
    await el.updateComplete;
    (el.shadowRoot?.querySelector("button.go") as HTMLButtonElement).click();
    expect(got).toEqual({ slow: true });
  });

  test("Add is disabled until exactly 12 digits", async () => {
    const el = await mount();
    const input = el.shadowRoot?.querySelector("input.serial") as HTMLInputElement;
    const addBtn = () =>
      Array.from(el.shadowRoot!.querySelectorAll("button.go")).find(
        (b) => b.textContent?.trim() === "Add",
      ) as HTMLButtonElement;
    expect(addBtn().disabled).toBe(true);
    input.value = "70400000683"; // 11 digits
    input.dispatchEvent(new Event("input"));
    await el.updateComplete;
    expect(addBtn().disabled).toBe(true);
    input.value = "999900000001"; // 12 digits
    input.dispatchEvent(new Event("input"));
    await el.updateComplete;
    expect(addBtn().disabled).toBe(false);
  });

  test("non-digits are stripped from the serial input", async () => {
    const el = await mount();
    const input = el.shadowRoot?.querySelector("input.serial") as HTMLInputElement;
    input.value = "99-99 00ab000001x"; // mixed → 999900000001
    input.dispatchEvent(new Event("input"));
    await el.updateComplete;
    expect(el.serial).toBe("999900000001");
  });

  test("Add emits {serial} and clears the field", async () => {
    const el = await mount();
    let got: { serial: string } | null = null;
    el.addEventListener("add", (e) => (got = (e as CustomEvent).detail));
    const input = el.shadowRoot?.querySelector("input.serial") as HTMLInputElement;
    input.value = "999900000001";
    input.dispatchEvent(new Event("input"));
    await el.updateComplete;
    const addBtn = Array.from(el.shadowRoot!.querySelectorAll("button.go")).find(
      (b) => b.textContent?.trim() === "Add",
    ) as HTMLButtonElement;
    addBtn.click();
    expect(got).toEqual({ serial: "999900000001" });
    expect(el.serial).toBe("");
  });

  test("busy disables all controls and shows Scanning…", async () => {
    const el = await mount({ busy: true });
    const scanBtn = el.shadowRoot?.querySelector("button.go") as HTMLButtonElement;
    expect(scanBtn.disabled).toBe(true);
    expect(scanBtn.textContent?.trim()).toContain("Scanning");
    for (const b of buttons(el)) expect(b.disabled).toBe(true);
  });
});

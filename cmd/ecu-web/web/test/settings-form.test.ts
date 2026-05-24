import { test, expect, describe } from "bun:test";
import "../src/components/settings-form.ts";
import type { SettingsForm } from "../src/components/settings-form.ts";
import type { Settings } from "../src/api.ts";

async function mount(settings: Settings): Promise<SettingsForm> {
  const el = document.createElement("settings-form") as SettingsForm;
  el.settings = settings;
  document.body.appendChild(el);
  await el.updateComplete;
  return el;
}

const base: Settings = {
  ecu_id: "216200001234",
  mac: "80971b000000",
  pan_override: "0DCE",
  zigbee_type: "apsystems",
};

describe("<settings-form>", () => {
  test("renders the given settings into the inputs", async () => {
    const el = await mount(base);
    const root = el.shadowRoot!;
    expect(root.querySelector<HTMLInputElement>("#ecu_id")!.value).toBe("216200001234");
    expect(root.querySelector<HTMLInputElement>("#mac")!.value).toBe("80971b000000");
    expect(root.querySelector<HTMLInputElement>("#pan_override")!.value).toBe("0DCE");
  });

  test("zigbee-type select reflects the value", async () => {
    const el = await mount({ ...base, zigbee_type: "general" });
    const sel = el.shadowRoot!.querySelector<HTMLSelectElement>("#zigbee_type")!;
    expect(sel.value).toBe("general");
  });

  test("Save dispatches a save event carrying the current values", async () => {
    const el = await mount(base);
    const root = el.shadowRoot!;

    // edit a couple of fields
    const ecu = root.querySelector<HTMLInputElement>("#ecu_id")!;
    ecu.value = "999900001111";
    const sel = root.querySelector<HTMLSelectElement>("#zigbee_type")!;
    sel.value = "general";

    let detail: Settings | null = null;
    el.addEventListener("save", (e) => {
      detail = (e as CustomEvent<Settings>).detail;
    });

    root.querySelector<HTMLButtonElement>("button.save")!.click();

    expect(detail).not.toBeNull();
    expect(detail!.ecu_id).toBe("999900001111");
    expect(detail!.mac).toBe("80971b000000");
    expect(detail!.pan_override).toBe("0DCE");
    expect(detail!.zigbee_type).toBe("general");
  });

  test("save event bubbles and is composed", async () => {
    const el = await mount(base);
    let caught = false;
    document.body.addEventListener("save", () => {
      caught = true;
    });
    el.shadowRoot!.querySelector<HTMLButtonElement>("button.save")!.click();
    expect(caught).toBe(true);
  });
});

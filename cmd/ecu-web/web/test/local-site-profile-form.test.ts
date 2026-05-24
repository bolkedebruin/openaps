import { test, expect, describe } from "bun:test";
import "../src/components/local-site-profile-form.ts";
import type { LocalSiteProfileForm, OverlayDraft } from "../src/components/local-site-profile-form.ts";
import type { ParamInfo, ProfileInverter, LocalSiteProfile } from "../src/api.ts";

const PARAMS: ParamInfo[] = [
  { aps_code: "CB", long_name: "Over_frequency_Watt_Low_set", unit: "Hz", group: "CrvSet", model: 134 },
  { aps_code: "DD", long_name: "Over_Frequency_Watt_Slope_set", unit: "%Pref/Hz", group: "DERFreqDroop", model: 711 },
];
const INVERTERS: ProfileInverter[] = [
  { uid: "ds3aaaaaaaaa", model: "DS3", model_code: 32, writable_codes: ["CB", "DD"] },
  { uid: "qs1aaaaaaaaa", model: "QS1A", model_code: 24, writable_codes: ["CB"] },
];
const DEFAULTS = { CB: { value: 50.2, unit: "Hz" }, DD: { value: 16.7, unit: "%Pref/Hz" } };

async function mount(props: Partial<LocalSiteProfileForm> = {}): Promise<LocalSiteProfileForm> {
  const el = document.createElement("local-site-profile-form") as LocalSiteProfileForm;
  el.params = props.params ?? PARAMS;
  el.inverters = props.inverters ?? INVERTERS;
  el.defaults = props.defaults ?? DEFAULTS;
  el.names = props.names ?? {};
  el.profile = props.profile ?? { id: "", uids: [], points: [] };
  el.editing = props.editing ?? false;
  el.busy = props.busy ?? false;
  document.body.appendChild(el);
  await el.updateComplete;
  return el;
}

function checkboxes(el: LocalSiteProfileForm): HTMLInputElement[] {
  return Array.from(el.shadowRoot!.querySelectorAll('.target input[type="checkbox"]'));
}
function valueInputs(el: LocalSiteProfileForm): HTMLInputElement[] {
  return Array.from(el.shadowRoot!.querySelectorAll("td.val input"));
}
async function selectTarget(el: LocalSiteProfileForm, idx: number) {
  const cb = checkboxes(el)[idx];
  cb.checked = true;
  cb.dispatchEvent(new Event("change"));
  await el.updateComplete;
}

describe("<local-site-profile-form>", () => {
  test("no params shown until a target is selected", async () => {
    const el = await mount();
    expect(valueInputs(el).length).toBe(0);
    expect(el.shadowRoot?.textContent).toContain("Select a target");
  });

  test("selecting DS3 enables both CB and DD", async () => {
    const el = await mount();
    await selectTarget(el, 0); // DS3
    const [cb, dd] = valueInputs(el);
    expect(cb.disabled).toBe(false);
    expect(dd.disabled).toBe(false);
  });

  test("DD disabled when a QS1A is also selected (capability intersection)", async () => {
    const el = await mount();
    await selectTarget(el, 0); // DS3
    await selectTarget(el, 1); // QS1A — drops DD from the writable intersection
    const [cb, dd] = valueInputs(el);
    expect(cb.disabled).toBe(false);
    expect(dd.disabled).toBe(true);
  });

  test("Save emits a draft with name, targets and entered values", async () => {
    const el = await mount();
    let got: OverlayDraft | null = null;
    el.addEventListener("save", (e) => (got = (e as CustomEvent<OverlayDraft>).detail));

    const nameInput = el.shadowRoot!.querySelector('input[type="text"]') as HTMLInputElement;
    nameInput.value = "victron-shift";
    nameInput.dispatchEvent(new Event("input"));
    await selectTarget(el, 0); // DS3

    const cb = valueInputs(el)[0];
    cb.value = "50.3";
    cb.dispatchEvent(new Event("input"));
    await el.updateComplete;

    (el.shadowRoot!.querySelector("button.save") as HTMLButtonElement).click();
    expect(got).not.toBeNull();
    expect(got!.id).toBe("victron-shift");
    expect(got!.uids).toEqual(["ds3aaaaaaaaa"]);
    expect(got!.points).toEqual([{ aps_code: "CB", value: 50.3 }]);
  });

  test("Save with no name shows an error and does not emit", async () => {
    const el = await mount();
    let emitted = false;
    el.addEventListener("save", () => (emitted = true));
    await selectTarget(el, 0);
    const cb = valueInputs(el)[0];
    cb.value = "50.3";
    cb.dispatchEvent(new Event("input"));
    await el.updateComplete;
    (el.shadowRoot!.querySelector("button.save") as HTMLButtonElement).click();
    await el.updateComplete;
    expect(emitted).toBe(false);
    expect(el.shadowRoot?.textContent).toContain("name is required");
  });

  test("editing an existing profile prefills and locks the name", async () => {
    const profile: LocalSiteProfile = {
      id: "victron-shift",
      uids: ["ds3aaaaaaaaa"],
      points: [{ aps_code: "CB", value: 50.3, unit: "Hz" }],
    };
    const el = await mount({ profile, editing: true });
    const nameInput = el.shadowRoot!.querySelector('input[type="text"]') as HTMLInputElement;
    expect(nameInput.value).toBe("victron-shift");
    expect(nameInput.disabled).toBe(true);
    // target prefilled => params visible, CB prefilled with 50.3
    expect(valueInputs(el)[0].value).toBe("50.3");
  });

  test("shows the base default and uses it as placeholder", async () => {
    const el = await mount();
    await selectTarget(el, 0); // DS3 -> CB and DD editable
    const defCells = Array.from(el.shadowRoot!.querySelectorAll("td.def")).map((c) => c.textContent?.trim());
    expect(defCells).toContain("50.2 Hz");
    expect(defCells).toContain("16.7 %Pref/Hz");
    const cb = valueInputs(el)[0];
    expect(cb.placeholder).toBe("50.2");
  });

  test("a value different from the default marks the row overridden", async () => {
    const el = await mount();
    await selectTarget(el, 0);
    const cb = valueInputs(el)[0];
    cb.value = "50.3";
    cb.dispatchEvent(new Event("input"));
    await el.updateComplete;
    expect(el.shadowRoot?.querySelector("tr.over")).not.toBeNull();
    expect(el.shadowRoot?.textContent).toContain("overridden");
  });

  test("a value equal to the default is NOT overridden", async () => {
    const el = await mount();
    await selectTarget(el, 0);
    const cb = valueInputs(el)[0];
    cb.value = "50.2"; // equals base default
    cb.dispatchEvent(new Event("input"));
    await el.updateComplete;
    expect(el.shadowRoot?.querySelector("tr.over")).toBeNull();
  });

  test("Cancel emits cancel", async () => {
    const el = await mount();
    let cancelled = false;
    el.addEventListener("cancel", () => (cancelled = true));
    (el.shadowRoot!.querySelector("button.cancel") as HTMLButtonElement).click();
    expect(cancelled).toBe(true);
  });
});

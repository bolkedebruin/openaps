import { test, expect, describe } from "bun:test";
import "../src/components/local-site-profile-form.ts";
import type { LocalSiteProfileForm, OverlayDraft } from "../src/components/local-site-profile-form.ts";
import type { ParamInfo, ProfileInverter } from "../src/api.ts";

const PARAMS: ParamInfo[] = [
  { aps_code: "CA", long_name: "Over_frequency_Watt_Start_set", unit: "Hz", group: "DERFreqDroop", model: 711 },
  { aps_code: "DD", long_name: "Over_Frequency_Watt_Slope_set", unit: "%Pref/Hz", group: "DERFreqDroop", model: 711 },
  { aps_code: "CB", long_name: "Over_frequency_Watt_Low_set", unit: "Hz", group: "CrvSet", model: 134 },
  { aps_code: "CC", long_name: "Over_frequency_Watt_High_set", unit: "Hz", group: "CrvSet", model: 134 },
  { aps_code: "AF", long_name: "over_frequency_slow", unit: "Hz", group: "MustTrip", model: 710 },
];
const INVERTERS: ProfileInverter[] = [
  // DS3 cannot write CA but reports a current value for it (read-only default).
  { uid: "ds3aaaaaaaaa", model: "DS3", model_code: 32, writable_codes: ["CB", "CC", "DD", "AF"], current: { CA: 50.2, DD: 16.6 } },
  { uid: "qs1aaaaaaaaa", model: "QS1A", model_code: 24, writable_codes: ["CB", "CC", "AF"], current: {} },
];
const DEFAULTS = {
  CB: { value: 50.2, unit: "Hz", min: 50.1, max: 52.0 },
  CC: { value: 51.5, unit: "Hz", min: 50.2, max: 53.0 },
  DD: { value: 16.7, unit: "%Pref/Hz" },
  AF: { value: 52.0, unit: "Hz" },
};

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

function rowInput(el: LocalSiteProfileForm, code: string): HTMLInputElement {
  const rows = Array.from(el.shadowRoot!.querySelectorAll("tbody tr"));
  const tr = rows.find((r) => r.querySelector(".pcode")?.textContent?.trim() === code)!;
  return tr.querySelector("td.val input") as HTMLInputElement;
}
async function setVal(el: LocalSiteProfileForm, code: string, v: string) {
  const inp = rowInput(el, code);
  inp.value = v;
  inp.dispatchEvent(new Event("input"));
  await el.updateComplete;
}
async function selectTarget(el: LocalSiteProfileForm, idx: number) {
  const cb = el.shadowRoot!.querySelectorAll('.target input[type="checkbox"]')[idx] as HTMLInputElement;
  cb.checked = true;
  cb.dispatchEvent(new Event("change"));
  await el.updateComplete;
}
function saveBtn(el: LocalSiteProfileForm) {
  return el.shadowRoot!.querySelector("button.save") as HTMLButtonElement;
}

describe("<local-site-profile-form>", () => {
  test("no params until a target is selected", async () => {
    const el = await mount();
    expect(el.shadowRoot!.querySelectorAll("td.val input").length).toBe(0);
    expect(el.shadowRoot?.textContent).toContain("Select a target");
  });

  test("groups params into labelled sections", async () => {
    const el = await mount();
    await selectTarget(el, 0);
    const names = Array.from(el.shadowRoot!.querySelectorAll("summary .gname")).map((s) => s.textContent?.trim());
    expect(names).toContain("Frequency-Watt droop");
    expect(names).toContain("Frequency-Watt curve");
    expect(names).toContain("Trip thresholds");
  });

  test("spells out cryptic names with a description", async () => {
    const el = await mount();
    await selectTarget(el, 0);
    const t = el.shadowRoot?.textContent ?? "";
    expect(t).toContain("Over-freq curve — start"); // CB label
    expect(t).not.toContain("Over_frequency_Watt_Low_set"); // raw long_name not shown as the label
  });

  test("group legend badge carries an explanatory tooltip", async () => {
    const el = await mount();
    await selectTarget(el, 0);
    const badge = el.shadowRoot!.querySelector(".legend .badge") as HTMLElement;
    expect(badge).not.toBeNull();
    expect(badge.getAttribute("title")?.length).toBeGreaterThan(10);
  });

  test("capability intersection disables DD when a QS1A is also selected", async () => {
    const el = await mount();
    await selectTarget(el, 0); // DS3
    expect(rowInput(el, "DD").disabled).toBe(false);
    await selectTarget(el, 1); // + QS1A (no DD)
    expect(rowInput(el, "DD").disabled).toBe(true);
    expect(rowInput(el, "CB").disabled).toBe(false);
  });

  test("shows base default and warns when a value is outside the base range", async () => {
    const el = await mount();
    await selectTarget(el, 0);
    expect(rowInput(el, "CB").placeholder).toBe("50.2");
    await setVal(el, "CB", "60"); // > max 52.0
    expect(el.shadowRoot?.textContent).toContain("outside base range");
  });

  test("blocks Save on a conflict and unblocks when resolved", async () => {
    const el = await mount();
    let emitted = false;
    el.addEventListener("save", () => (emitted = true));
    await selectTarget(el, 0);
    const nameInput = el.shadowRoot!.querySelector('input[type="text"]') as HTMLInputElement;
    nameInput.value = "x";
    nameInput.dispatchEvent(new Event("input"));
    await setVal(el, "CB", "51");
    await setVal(el, "CC", "50"); // start (51) past end (50) -> conflict
    expect(el.shadowRoot?.textContent).toContain("Conflicting settings");
    expect(saveBtn(el).disabled).toBe(true);
    saveBtn(el).click();
    expect(emitted).toBe(false);

    await setVal(el, "CC", "52"); // resolve
    expect(saveBtn(el).disabled).toBe(false);
    saveBtn(el).click();
    expect(emitted).toBe(true);
  });

  test("Save emits a draft with entered values", async () => {
    const el = await mount();
    let got: OverlayDraft | null = null;
    el.addEventListener("save", (e) => (got = (e as CustomEvent<OverlayDraft>).detail));
    const nameInput = el.shadowRoot!.querySelector('input[type="text"]') as HTMLInputElement;
    nameInput.value = "victron-shift";
    nameInput.dispatchEvent(new Event("input"));
    await selectTarget(el, 0);
    await setVal(el, "CB", "50.3");
    saveBtn(el).click();
    expect(got).not.toBeNull();
    expect(got!.id).toBe("victron-shift");
    expect(got!.uids).toEqual(["ds3aaaaaaaaa"]);
    expect(got!.points).toEqual([{ aps_code: "CB", value: 50.3 }]);
  });

  test("editing locks the name and prefills overrides", async () => {
    const el = await mount({
      editing: true,
      profile: { id: "victron-shift", uids: ["ds3aaaaaaaaa"], points: [{ aps_code: "CB", value: 50.4, unit: "Hz" }] },
    });
    const nameInput = el.shadowRoot!.querySelector('input[type="text"]') as HTMLInputElement;
    expect(nameInput.value).toBe("victron-shift");
    expect(nameInput.disabled).toBe(true);
    expect(rowInput(el, "CB").value).toBe("50.4");
  });

  test("shows a Code column with the aps_code", async () => {
    const el = await mount();
    await selectTarget(el, 0);
    const headers = Array.from(el.shadowRoot!.querySelectorAll("th")).map((h) => h.textContent?.trim());
    expect(headers).toContain("Code");
    const codes = Array.from(el.shadowRoot!.querySelectorAll("tbody td.pcode")).map((c) => c.textContent?.trim());
    expect(codes).toContain("CB");
  });

  test("uses the inverter's current value as default and shows it read-only when not writable", async () => {
    const el = await mount();
    await selectTarget(el, 0); // DS3: CA not writable, current 50.2
    const ca = rowInput(el, "CA");
    expect(ca.disabled).toBe(true); // not writable -> read-only
    expect(ca.value).toBe("50.2"); // shows the inverter's current value
    const t = el.shadowRoot?.textContent ?? "";
    expect(t).toContain("read-only");
    expect(t).toContain("inv"); // default sourced from the inverter
  });

  test("focusing an empty editable field prefills the default", async () => {
    const el = await mount({ defaults: { CB: { value: 50.2, unit: "Hz" } } });
    await selectTarget(el, 0);
    const cb = rowInput(el, "CB");
    expect(cb.value).toBe(""); // starts empty
    cb.dispatchEvent(new Event("focus"));
    await el.updateComplete;
    expect(rowInput(el, "CB").value).toBe("50.2"); // prefilled from base default
  });

  test("clearing an override reverts to the default", async () => {
    const el = await mount({ defaults: { CB: { value: 50.2, unit: "Hz" } } });
    await selectTarget(el, 0);
    await setVal(el, "CB", "50.6");
    expect(el.shadowRoot?.querySelector("tr.over")).not.toBeNull();
    const clear = el.shadowRoot!.querySelector("button.clear") as HTMLButtonElement;
    expect(clear).not.toBeNull();
    clear.click();
    await el.updateComplete;
    expect(rowInput(el, "CB").value).toBe("");
    expect(el.shadowRoot?.querySelector("tr.over")).toBeNull();
  });

  test("a value equal to the default is not saved", async () => {
    const el = await mount({ defaults: { CB: { value: 50.2, unit: "Hz" } } });
    let emitted = false;
    el.addEventListener("save", () => (emitted = true));
    const nameInput = el.shadowRoot!.querySelector('input[type="text"]') as HTMLInputElement;
    nameInput.value = "x";
    nameInput.dispatchEvent(new Event("input"));
    await selectTarget(el, 0);
    await setVal(el, "CB", "50.2"); // equals default -> not an override
    saveBtn(el).click();
    await el.updateComplete;
    expect(emitted).toBe(false);
    expect(el.shadowRoot?.textContent).toContain("Change at least one");
  });

  test("Cancel emits cancel", async () => {
    const el = await mount();
    let cancelled = false;
    el.addEventListener("cancel", () => (cancelled = true));
    (el.shadowRoot!.querySelector("button.cancel") as HTMLButtonElement).click();
    expect(cancelled).toBe(true);
  });
});

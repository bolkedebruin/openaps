import { test, expect, describe } from "bun:test";
import "../src/components/grid-profile-form.ts";
import type { GridProfileForm } from "../src/components/grid-profile-form.ts";
import type { GridProfileSummary } from "../src/api.ts";

const PROFILES: GridProfileSummary[] = [
  { id: "EN50549-1", vnom_v: 230, source_ref: "TY=36", point_count: 13 },
  { id: "CEI-0-21", vnom_v: 230, source_ref: "TY=10", point_count: 12 },
];

async function mount(props: Partial<GridProfileForm>): Promise<GridProfileForm> {
  const el = document.createElement("grid-profile-form") as GridProfileForm;
  el.profiles = props.profiles ?? PROFILES;
  el.activeBase = props.activeBase ?? "";
  el.reconcilerReady = props.reconcilerReady ?? true;
  el.busy = props.busy ?? false;
  document.body.appendChild(el);
  await el.updateComplete;
  return el;
}

describe("<grid-profile-form>", () => {
  test("shows the active profile with details", async () => {
    const el = await mount({ activeBase: "EN50549-1" });
    const t = el.shadowRoot?.textContent ?? "";
    expect(t).toContain("EN50549-1");
    expect(t).toContain("230 V");
    expect(t).toContain("13 pts");
  });

  test("shows 'none selected' when no active base", async () => {
    const el = await mount({ activeBase: "" });
    expect(el.shadowRoot?.textContent).toContain("none selected");
  });

  test("renders one option per profile", async () => {
    const el = await mount({ activeBase: "EN50549-1" });
    const opts = el.shadowRoot?.querySelectorAll("select option") ?? [];
    // 2 profiles, no placeholder option because activeBase is set
    expect(opts.length).toBe(2);
  });

  test("Apply is disabled until a different profile is picked", async () => {
    const el = await mount({ activeBase: "EN50549-1" });
    const btn = el.shadowRoot?.querySelector("button.apply") as HTMLButtonElement;
    expect(btn.disabled).toBe(true);
  });

  test("changing the dropdown then Apply emits the selected id", async () => {
    const el = await mount({ activeBase: "EN50549-1" });
    let got = "";
    el.addEventListener("apply", (e) => {
      got = (e as CustomEvent<string>).detail;
    });
    const sel = el.shadowRoot?.querySelector("select") as HTMLSelectElement;
    sel.value = "CEI-0-21";
    sel.dispatchEvent(new Event("change"));
    await el.updateComplete;

    const btn = el.shadowRoot?.querySelector("button.apply") as HTMLButtonElement;
    expect(btn.disabled).toBe(false);
    btn.click();
    expect(got).toBe("CEI-0-21");
  });

  test("Apply disabled when reconciler not ready", async () => {
    const el = await mount({ activeBase: "", reconcilerReady: false });
    const sel = el.shadowRoot?.querySelector("select") as HTMLSelectElement;
    sel.value = "CEI-0-21";
    sel.dispatchEvent(new Event("change"));
    await el.updateComplete;
    const btn = el.shadowRoot?.querySelector("button.apply") as HTMLButtonElement;
    expect(btn.disabled).toBe(true);
    expect(el.shadowRoot?.textContent).toContain("reconciler not ready");
  });

  test("busy shows Applying and disables Apply", async () => {
    const el = await mount({ activeBase: "EN50549-1", busy: true });
    const btn = el.shadowRoot?.querySelector("button.apply") as HTMLButtonElement;
    expect(btn.textContent?.trim()).toContain("Applying");
    expect(btn.disabled).toBe(true);
  });
});

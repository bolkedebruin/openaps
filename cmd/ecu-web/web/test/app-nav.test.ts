import { test, expect, describe } from "bun:test";
import "../src/components/app-nav.ts";
import type { AppNav, NavItem } from "../src/components/app-nav.ts";

const ITEMS: NavItem[] = [
  { id: "dashboard", label: "Dashboard", icon: "▮▮" },
  { id: "inverters", label: "Inverters", icon: "⌁" },
  { id: "profiles", label: "Profiles", icon: "⛭" },
  { id: "settings", label: "Settings", icon: "⚙" },
];

async function mount(props: Partial<AppNav> = {}): Promise<AppNav> {
  const el = document.createElement("app-nav") as AppNav;
  el.items = props.items ?? ITEMS;
  el.route = props.route ?? "dashboard";
  el.open = props.open ?? false;
  document.body.appendChild(el);
  await el.updateComplete;
  return el;
}

describe("<app-nav>", () => {
  test("renders one link per item, pointing at its hash route", async () => {
    const el = await mount();
    const links = Array.from(el.shadowRoot!.querySelectorAll("a.item")) as HTMLAnchorElement[];
    expect(links.length).toBe(ITEMS.length);
    expect(links.map((a) => a.getAttribute("href"))).toEqual(ITEMS.map((i) => `#/${i.id}`));
    expect(el.shadowRoot?.textContent).toContain("Profiles");
  });

  test("marks the active route", async () => {
    const el = await mount({ route: "profiles" });
    const active = el.shadowRoot!.querySelector("a.item.active") as HTMLAnchorElement;
    expect(active.getAttribute("href")).toBe("#/profiles");
  });

  test("clicking an item emits close", async () => {
    const el = await mount({ open: true });
    let closed = false;
    el.addEventListener("close", () => (closed = true));
    (el.shadowRoot!.querySelector('a[href="#/inverters"]') as HTMLAnchorElement).click();
    expect(closed).toBe(true);
  });

  test("scrim shows only when open and emits close on click", async () => {
    const closed = await mount({ open: false });
    expect(closed.shadowRoot?.querySelector(".scrim")).toBeNull();

    const el = await mount({ open: true });
    const scrim = el.shadowRoot!.querySelector(".scrim") as HTMLElement;
    expect(scrim).not.toBeNull();
    let got = false;
    el.addEventListener("close", () => (got = true));
    scrim.click();
    expect(got).toBe(true);
  });

  test("open toggles the drawer class on nav", async () => {
    const el = await mount({ open: false });
    expect(el.shadowRoot!.querySelector("nav")!.classList.contains("open")).toBe(false);
    el.open = true;
    await el.updateComplete;
    expect(el.shadowRoot!.querySelector("nav")!.classList.contains("open")).toBe(true);
  });
});

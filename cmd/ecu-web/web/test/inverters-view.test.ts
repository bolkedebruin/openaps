import { test, expect, describe, beforeEach, afterEach } from "bun:test";
import "../src/views/inverters-view.ts";
import { api, type PairingStatus } from "../src/api.ts";
import type { InvertersView } from "../src/views/inverters-view.ts";
import type { Fleet, Inverter } from "../src/api.ts";

function inv(over: Partial<Inverter> = {}): Inverter {
  return {
    uid: "704000006835", short_addr: 1, model: "DS3", model_code: 0x20, phase: 1,
    sw_version: 3067, online: true, last_seen_ms: Date.now(), age_s: 2,
    active_power_w: 0, nameplate_w: 750, load_pct: 0, grid_v: 0, bus_v: 0,
    freq_hz: 0, reactive_var: 0, rssi: 0, lqi: 0, panels: [],
    ...over,
  };
}

function fleet(inverters: Inverter[]): Fleet {
  return {
    ts_ms: Date.now(), nameplate_total_w: 0, inverter_count: inverters.length,
    online_count: 0, active_power_w: 0, lifetime_wh: 0, today_wh: 0, month_wh: 0, year_wh: 0,
    inverters,
  };
}

async function mount(f: Fleet): Promise<InvertersView> {
  const el = document.createElement("inverters-view") as InvertersView;
  el.fleet = f;
  document.body.appendChild(el);
  await el.updateComplete;
  return el;
}

describe("<inverters-view>", () => {
  test("has a Firmware column between Model and Status", async () => {
    const el = await mount(fleet([inv()]));
    const headers = Array.from(el.shadowRoot!.querySelectorAll("th")).map((h) => h.textContent?.trim());
    const mi = headers.indexOf("Model");
    const fi = headers.indexOf("Firmware");
    const si = headers.indexOf("Status");
    expect(fi).toBeGreaterThan(-1);
    expect(fi).toBe(mi + 1);
    expect(si).toBe(fi + 1);
  });

  test("renders the firmware version value", async () => {
    const el = await mount(fleet([inv({ sw_version: 5203 })]));
    expect(el.shadowRoot?.querySelector(".fw")?.textContent?.trim()).toBe("5203");
  });

  test("shows a dash when firmware is unknown (0)", async () => {
    const el = await mount(fleet([inv({ sw_version: 0 })]));
    expect(el.shadowRoot?.querySelector(".fw")?.textContent?.trim()).toBe("—");
  });

  test("has an Encryption column for the link-encryption badge", async () => {
    const el = await mount(fleet([inv()]));
    const headers = Array.from(el.shadowRoot!.querySelectorAll("th")).map((h) => h.textContent?.trim());
    expect(headers).toContain("Encryption");
    expect(headers).not.toContain("Link");
  });

  test("badge shows plaintext for encrypted=false and AES for true in the normal list", async () => {
    const elPlain = await mount(fleet([inv({ encrypted: false })]));
    const plainBadge = elPlain.shadowRoot?.querySelector("tbody .enc");
    expect(plainBadge?.classList.contains("enc-warn")).toBe(true);
    expect(plainBadge?.textContent?.toLowerCase()).toContain("plaintext");

    const elAes = await mount(fleet([inv({ encrypted: true })]));
    const aesBadge = elAes.shadowRoot?.querySelector("tbody .enc");
    expect(aesBadge?.classList.contains("enc-ok")).toBe(true);
    expect(aesBadge?.textContent).toContain("AES");
  });

  test("encrypted=true renders an AES lock badge", async () => {
    const el = await mount(fleet([inv({ encrypted: true })]));
    const badge = el.shadowRoot?.querySelector(".enc");
    expect(badge?.classList.contains("enc-ok")).toBe(true);
    expect(badge?.textContent).toContain("AES");
  });

  test("encrypted=false renders a plaintext warning badge", async () => {
    const el = await mount(fleet([inv({ encrypted: false })]));
    const badge = el.shadowRoot?.querySelector(".enc");
    expect(badge?.classList.contains("enc-warn")).toBe(true);
    expect(badge?.textContent?.toLowerCase()).toContain("plaintext");
  });

  test("missing encrypted renders the neutral unknown badge", async () => {
    const el = await mount(fleet([inv()])); // encrypted undefined
    const badge = el.shadowRoot?.querySelector(".enc");
    expect(badge?.classList.contains("enc-unknown")).toBe(true);
  });

  test("each row has a Replace button", async () => {
    const el = await mount(fleet([inv()]));
    const btn = el.shadowRoot?.querySelector("button.replace") as HTMLButtonElement;
    expect(btn).not.toBeNull();
    expect(btn.textContent?.trim()).toBe("Replace");
  });

  test("renders the scan panel and fleet re-key control", async () => {
    const el = await mount(fleet([inv()]));
    expect(el.shadowRoot?.querySelector("pairing-scan-panel")).not.toBeNull();
    expect(el.shadowRoot?.querySelector("button.rekey-btn")).not.toBeNull();
    expect(el.shadowRoot?.querySelector("pairing-progress-drawer")).not.toBeNull();
  });

  test("renders a Change ZigBee channel control next to fleet re-key", async () => {
    const el = await mount(fleet([inv()]));
    const btns = Array.from(
      el.shadowRoot!.querySelectorAll<HTMLButtonElement>("button.rekey-btn"),
    );
    const labels = btns.map((b) => b.textContent?.trim());
    expect(labels).toContain("Fleet re-key…");
    expect(labels).toContain("Change ZigBee channel…");
  });
});

// Helpers shared by the privileged-action dialog tests below. The dialog is
// rendered into the inverters-view's shadow root (as <password-confirm-dialog>)
// and exposes its own internal value/password inputs in its own shadow root.

function rekeyBtn(el: InvertersView): HTMLButtonElement {
  const btns = Array.from(el.shadowRoot!.querySelectorAll<HTMLButtonElement>("button.rekey-btn"));
  return btns.find((b) => b.textContent?.trim() === "Fleet re-key…")!;
}

function changeChannelBtn(el: InvertersView): HTMLButtonElement {
  const btns = Array.from(el.shadowRoot!.querySelectorAll<HTMLButtonElement>("button.rekey-btn"));
  return btns.find((b) => b.textContent?.trim() === "Change ZigBee channel…")!;
}

function dialog(el: InvertersView): HTMLElement | null {
  return el.shadowRoot!.querySelector("password-confirm-dialog") as HTMLElement | null;
}

function valueInput(el: InvertersView): HTMLInputElement {
  return dialog(el)!.shadowRoot!.querySelector<HTMLInputElement>("#pcd_value")!;
}

function pwdInput(el: InvertersView): HTMLInputElement {
  return dialog(el)!.shadowRoot!.querySelector<HTMLInputElement>("#pcd_pwd")!;
}

function dialogConfirmBtn(el: InvertersView): HTMLButtonElement {
  return dialog(el)!.shadowRoot!.querySelector<HTMLButtonElement>("button.primary")!;
}

function dialogCancelBtn(el: InvertersView): HTMLButtonElement {
  return dialog(el)!.shadowRoot!.querySelector<HTMLButtonElement>("button.secondary")!;
}

async function type(input: HTMLInputElement, value: string) {
  input.value = value;
  input.dispatchEvent(new Event("input", { bubbles: true, composed: true }));
}

async function flush(el: InvertersView) {
  await new Promise((r) => setTimeout(r, 0));
  await el.updateComplete;
  const d = dialog(el);
  if (d) await (d as unknown as { updateComplete: Promise<void> }).updateComplete;
}

describe("<inverters-view> change-channel action", () => {
  let restoreCC: (() => void) | null = null;
  let restoreStatus: (() => void) | null = null;
  let restoreVerify: (() => void) | null = null;
  let lastChannel: number | null = null;
  let ccResp: { ok: boolean; error?: string; status?: PairingStatus } = { ok: true };
  let verifyOk: boolean = true;

  beforeEach(() => {
    lastChannel = null;
    ccResp = { ok: true, status: { op: "change_channel", stage: "change_channel", total: 2, done: 0 } };
    verifyOk = true;

    const origCC = api.pairingChangeChannel;
    api.pairingChangeChannel = async (channel: number) => {
      lastChannel = channel;
      return ccResp;
    };
    restoreCC = () => { api.pairingChangeChannel = origCC; };

    const origStatus = api.pairingStatus;
    api.pairingStatus = async () => ({ ok: true, status: { op: "", stage: "" } });
    restoreStatus = () => { api.pairingStatus = origStatus; };

    const origVerify = api.verifyPassword;
    api.verifyPassword = async () => verifyOk;
    restoreVerify = () => { api.verifyPassword = origVerify; };
  });

  afterEach(() => {
    restoreCC?.(); restoreStatus?.(); restoreVerify?.();
    restoreCC = restoreStatus = restoreVerify = null;
    document.body.querySelectorAll("inverters-view").forEach((n) => n.remove());
  });

  test("clicking change-channel opens the inline dialog (not browser prompt)", async () => {
    const el = await mount(fleet([inv()]));
    expect(dialog(el)).toBeNull();
    changeChannelBtn(el).click();
    await flush(el);
    expect(dialog(el)).not.toBeNull();
    expect((dialog(el) as unknown as { kind: string }).kind).toBe("channel");
  });

  test("valid channel + correct password: posts the channel and opens the drawer", async () => {
    const el = await mount(fleet([inv()]));
    changeChannelBtn(el).click();
    await flush(el);

    await type(valueInput(el), "20");
    await type(pwdInput(el), "hunter2");
    dialogConfirmBtn(el).click();
    await flush(el);

    expect(lastChannel).toBe(20);
    expect(dialog(el)).toBeNull(); // dialog closed on success
    const drawer = el.shadowRoot!.querySelector("pairing-progress-drawer")!;
    expect((drawer as unknown as { open: boolean }).open).toBe(true);
    expect((drawer as unknown as { status: PairingStatus }).status.op).toBe("change_channel");
  });

  test("out-of-range channel: inline value error, no API call, dialog stays open", async () => {
    const el = await mount(fleet([inv()]));
    changeChannelBtn(el).click();
    await flush(el);

    await type(valueInput(el), "30");
    await type(pwdInput(el), "hunter2");
    dialogConfirmBtn(el).click();
    await flush(el);

    expect(lastChannel).toBeNull();
    expect(dialog(el)).not.toBeNull();
    expect(dialog(el)!.shadowRoot!.textContent ?? "").toContain(
      "Channel must be an integer 11–26",
    );
  });

  test("wrong password: surfaces 'password is wrong', user can retry", async () => {
    const el = await mount(fleet([inv()]));
    changeChannelBtn(el).click();
    await flush(el);

    verifyOk = false;
    await type(valueInput(el), "18");
    await type(pwdInput(el), "nope");
    dialogConfirmBtn(el).click();
    await flush(el);

    expect(lastChannel).toBeNull();
    expect(dialog(el)).not.toBeNull();
    expect(dialog(el)!.shadowRoot!.textContent?.toLowerCase() ?? "").toContain(
      "password is wrong",
    );

    // retry with the correct password
    verifyOk = true;
    await type(pwdInput(el), "hunter2");
    dialogConfirmBtn(el).click();
    await flush(el);

    expect(lastChannel).toBe(18);
    expect(dialog(el)).toBeNull();
  });

  test("cancel closes the dialog cleanly", async () => {
    const el = await mount(fleet([inv()]));
    changeChannelBtn(el).click();
    await flush(el);

    dialogCancelBtn(el).click();
    await flush(el);

    expect(dialog(el)).toBeNull();
    expect(lastChannel).toBeNull();
  });

  test("action error from server is shown inside the still-open dialog", async () => {
    const el = await mount(fleet([inv()]));
    changeChannelBtn(el).click();
    await flush(el);

    ccResp = { ok: false, error: "fleet busy" };
    await type(valueInput(el), "18");
    await type(pwdInput(el), "hunter2");
    dialogConfirmBtn(el).click();
    await flush(el);

    expect(lastChannel).toBe(18);
    expect(dialog(el)).not.toBeNull();
    expect(dialog(el)!.shadowRoot!.textContent ?? "").toContain("fleet busy");
  });
});

describe("<inverters-view> rekey action", () => {
  let restoreRekey: (() => void) | null = null;
  let restoreStatus: (() => void) | null = null;
  let restoreVerify: (() => void) | null = null;
  let lastPan: string | null = null;
  let lastChannel: number | null = null;
  let rkResp: { ok: boolean; error?: string; status?: PairingStatus } = { ok: true };
  let verifyOk: boolean = true;

  beforeEach(() => {
    lastPan = null;
    lastChannel = null;
    rkResp = { ok: true, status: { op: "rekey", stage: "rekey", total: 1, done: 0 } };
    verifyOk = true;

    const origRekey = api.pairingRekey;
    api.pairingRekey = async (pan: string, channel?: number) => {
      lastPan = pan;
      lastChannel = channel ?? 0;
      return rkResp;
    };
    restoreRekey = () => { api.pairingRekey = origRekey; };

    const origStatus = api.pairingStatus;
    api.pairingStatus = async () => ({ ok: true, status: { op: "", stage: "" } });
    restoreStatus = () => { api.pairingStatus = origStatus; };

    const origVerify = api.verifyPassword;
    api.verifyPassword = async () => verifyOk;
    restoreVerify = () => { api.verifyPassword = origVerify; };
  });

  afterEach(() => {
    restoreRekey?.(); restoreStatus?.(); restoreVerify?.();
    restoreRekey = restoreStatus = restoreVerify = null;
    document.body.querySelectorAll("inverters-view").forEach((n) => n.remove());
  });

  test("clicking re-key opens the inline dialog with kind=rekey", async () => {
    const el = await mount(fleet([inv()]));
    rekeyBtn(el).click();
    await flush(el);
    expect(dialog(el)).not.toBeNull();
    expect((dialog(el) as unknown as { kind: string }).kind).toBe("rekey");
  });

  test("valid PAN + correct password: posts the PAN and opens the drawer", async () => {
    const el = await mount(fleet([inv()]));
    rekeyBtn(el).click();
    await flush(el);

    await type(valueInput(el), "0DCE");
    await type(pwdInput(el), "hunter2");
    dialogConfirmBtn(el).click();
    await flush(el);

    expect(lastPan).toBe("0DCE");
    expect(lastChannel).toBe(0);
    expect(dialog(el)).toBeNull();
    const drawer = el.shadowRoot!.querySelector("pairing-progress-drawer")!;
    expect((drawer as unknown as { open: boolean }).open).toBe(true);
  });

  test("invalid PAN: inline value error, no API call, dialog stays open", async () => {
    const el = await mount(fleet([inv()]));
    rekeyBtn(el).click();
    await flush(el);

    await type(valueInput(el), "ZZZZ");
    await type(pwdInput(el), "hunter2");
    dialogConfirmBtn(el).click();
    await flush(el);

    expect(lastPan).toBeNull();
    expect(dialog(el)).not.toBeNull();
    expect(dialog(el)!.shadowRoot!.textContent ?? "").toContain(
      "PAN must be 1–4 hexadecimal digits",
    );
  });

  test("wrong password: shows error, retry succeeds", async () => {
    const el = await mount(fleet([inv()]));
    rekeyBtn(el).click();
    await flush(el);

    verifyOk = false;
    await type(valueInput(el), "1234");
    await type(pwdInput(el), "nope");
    dialogConfirmBtn(el).click();
    await flush(el);

    expect(lastPan).toBeNull();
    expect(dialog(el)).not.toBeNull();

    verifyOk = true;
    await type(pwdInput(el), "hunter2");
    dialogConfirmBtn(el).click();
    await flush(el);

    expect(lastPan).toBe("1234");
    expect(dialog(el)).toBeNull();
  });
});

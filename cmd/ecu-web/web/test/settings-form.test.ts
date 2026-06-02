import { test, expect, describe, beforeEach, afterEach } from "bun:test";
import "../src/components/settings-form.ts";
import { channelInputInvalid } from "../src/components/settings-form.ts";
import type { SettingsForm } from "../src/components/settings-form.ts";
import type { Settings } from "../src/api.ts";

interface MountOpts {
  hostname?: string;
}

async function mount(settings: Settings, opts: MountOpts = {}): Promise<SettingsForm> {
  const el = document.createElement("settings-form") as SettingsForm;
  el.settings = settings;
  if (opts.hostname !== undefined) el.hostname = opts.hostname;
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

// Track all settings-form elements we mount so we can clean them up after each
// test — happy-dom doesn't auto-teardown between tests.
afterEach(() => {
  document.body.querySelectorAll("settings-form").forEach((n) => n.remove());
});

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

describe("<settings-form> effective hints (computed client-side)", () => {
  test("renders effective PAN (from configured MAC) / zigbee-type hints", async () => {
    // Effective PAN is now computed CLIENT-SIDE from the configured MAC; the
    // server no longer sends SettingsResponse.effective for radio fields.
    const el = await mount({
      ecu_id: "",
      mac: "aa:bb:cc:dd:ee:ff",
      pan_override: "",
      zigbee_type: "",
    });
    const text = el.shadowRoot!.textContent ?? "";
    expect(text).toContain("effective PAN source: aa:bb:cc:dd:ee:ff");
    expect(text).toContain("effective: EEFF (from MAC)");
    expect(text).toContain("effective: apsystems (default)");
  });

  test("PAN-override set: hint reads 'effective: <pan>' without (from MAC)", async () => {
    const el = await mount({
      ecu_id: "",
      mac: "",
      pan_override: "1234",
      zigbee_type: "apsystems",
    });
    const text = el.shadowRoot!.textContent ?? "";
    expect(text).toContain("effective: 1234");
    expect(text).not.toContain("(from MAC)");
  });
});

describe("<settings-form> ECU ID suggestion", () => {
  test("when ECU ID is empty, input is seeded with hostname and hint is shown", async () => {
    const el = await mount(
      { ecu_id: "", mac: "", pan_override: "", zigbee_type: "apsystems" },
      { hostname: "APS-ECU" },
    );
    const root = el.shadowRoot!;
    const input = root.querySelector<HTMLInputElement>("#ecu_id")!;
    expect(input.value).toBe("APS-ECU");
    expect(root.textContent ?? "").toContain("Recommended: use the serial on the device label.");
  });

  test("when ECU ID is set, hostname is NOT used and no hint is shown", async () => {
    const el = await mount(
      { ecu_id: "MY-ECU-01", mac: "", pan_override: "", zigbee_type: "apsystems" },
      { hostname: "APS-ECU" },
    );
    const root = el.shadowRoot!;
    const input = root.querySelector<HTMLInputElement>("#ecu_id")!;
    expect(input.value).toBe("MY-ECU-01");
    expect(root.textContent ?? "").not.toContain("Recommended: use the serial");
  });
});

describe("<settings-form> PAN-change confirm dialog", () => {
  // Stub verifyPassword on the api module so the dialog flow can be driven
  // without an HTTP server. We import the module dynamically so we can patch
  // it in-place; the form already imported it but holds a reference to the
  // exported `api` object, so patching the object's property works.
  let restorePwd: (() => void) | null = null;
  let lastPwd: string | null = null;
  let answer: boolean = true;

  beforeEach(async () => {
    const mod = await import("../src/api.ts");
    const orig = mod.api.verifyPassword;
    mod.api.verifyPassword = async (p: string) => {
      lastPwd = p;
      return answer;
    };
    restorePwd = () => {
      mod.api.verifyPassword = orig;
    };
    lastPwd = null;
    answer = true;
  });

  afterEach(() => {
    if (restorePwd) restorePwd();
    restorePwd = null;
    document.body.querySelectorAll(".backdrop").forEach((n) => n.remove());
  });

  // makeWithEffectivePAN persists a colon MAC whose lower-16 IS the wanted
  // effective PAN, so the client-side computation reproduces it without any
  // server-sent effective value.
  function makeWithEffectivePAN(pan: string): Settings {
    // pan "0DCE" -> MAC ...0d:ce. Build a MAC ending in the pan's two octets.
    const lo = pan.toUpperCase().padStart(4, "0");
    const mac = `80:97:1b:03:${lo.slice(0, 2)}:${lo.slice(2, 4)}`.toLowerCase();
    return {
      ecu_id: "",
      mac,
      pan_override: "",
      zigbee_type: "apsystems",
    };
  }

  test("no-divergence Save dispatches immediately without dialog", async () => {
    const el = await mount(makeWithEffectivePAN("0DCE"));
    let dispatched = false;
    el.addEventListener("save", () => (dispatched = true));
    el.shadowRoot!.querySelector<HTMLButtonElement>("button.save")!.click();
    await el.updateComplete;
    expect(dispatched).toBe(true);
    expect(el.shadowRoot!.querySelector(".backdrop")).toBeNull();
  });

  test("typed pan_override that diverges opens the confirm dialog and gates Save", async () => {
    const el = await mount(makeWithEffectivePAN("0DCE"));
    const root = el.shadowRoot!;

    // operator types a new PAN override
    const pan = root.querySelector<HTMLInputElement>("#pan_override")!;
    pan.value = "1234";

    let dispatched = false;
    el.addEventListener("save", () => (dispatched = true));

    root.querySelector<HTMLButtonElement>("button.save")!.click();
    await el.updateComplete;

    // dialog open, no save yet
    expect(root.querySelector(".backdrop")).not.toBeNull();
    expect(dispatched).toBe(false);
    expect(root.textContent ?? "").toContain("0DCE");
    expect(root.textContent ?? "").toContain("1234");
  });

  test("wrong password keeps the dialog open and shows an error", async () => {
    answer = false;
    const el = await mount(makeWithEffectivePAN("0DCE"));
    const root = el.shadowRoot!;
    root.querySelector<HTMLInputElement>("#pan_override")!.value = "1234";

    let dispatched = false;
    el.addEventListener("save", () => (dispatched = true));

    root.querySelector<HTMLButtonElement>("button.save")!.click();
    await el.updateComplete;

    root.querySelector<HTMLInputElement>("#confirm_pwd")!.value = "nope";
    // Confirm button is the .primary one in the dialog row
    const buttons = Array.from(root.querySelectorAll<HTMLButtonElement>(".dialog button"));
    const confirm = buttons.find((b) => b.classList.contains("primary"))!;
    confirm.click();
    // wait for the async verify to resolve and the element to re-render
    await new Promise((r) => setTimeout(r, 0));
    await el.updateComplete;

    expect(lastPwd).toBe("nope");
    expect(dispatched).toBe(false);
    expect(root.querySelector(".backdrop")).not.toBeNull();
    expect(root.textContent ?? "").toContain("Wrong password.");
  });

  test("correct password closes the dialog and dispatches save", async () => {
    answer = true;
    const el = await mount(makeWithEffectivePAN("0DCE"));
    const root = el.shadowRoot!;
    root.querySelector<HTMLInputElement>("#pan_override")!.value = "1234";

    let detail: Settings | null = null;
    el.addEventListener("save", (e) => {
      detail = (e as CustomEvent<Settings>).detail;
    });

    root.querySelector<HTMLButtonElement>("button.save")!.click();
    await el.updateComplete;

    root.querySelector<HTMLInputElement>("#confirm_pwd")!.value = "test1234";
    const buttons = Array.from(root.querySelectorAll<HTMLButtonElement>(".dialog button"));
    const confirm = buttons.find((b) => b.classList.contains("primary"))!;
    confirm.click();
    await new Promise((r) => setTimeout(r, 0));
    await el.updateComplete;

    expect(lastPwd).toBe("test1234");
    expect(detail).not.toBeNull();
    expect(detail!.pan_override).toBe("1234");
    expect(root.querySelector(".backdrop")).toBeNull();
  });

  test("Cancel closes the dialog without dispatching save", async () => {
    const el = await mount(makeWithEffectivePAN("0DCE"));
    const root = el.shadowRoot!;
    root.querySelector<HTMLInputElement>("#pan_override")!.value = "1234";

    let dispatched = false;
    el.addEventListener("save", () => (dispatched = true));

    root.querySelector<HTMLButtonElement>("button.save")!.click();
    await el.updateComplete;

    const buttons = Array.from(root.querySelectorAll<HTMLButtonElement>(".dialog button"));
    const cancel = buttons.find((b) => b.classList.contains("secondary"))!;
    cancel.click();
    await el.updateComplete;

    expect(dispatched).toBe(false);
    expect(root.querySelector(".backdrop")).toBeNull();
  });
});

// Helper: fire an "input" event on an input so the form's @input handler
// updates the typed mirrors (the form derives Save-disabled from those).
function setInput(root: ShadowRoot, id: string, value: string) {
  const inp = root.querySelector<HTMLInputElement>(`#${id}`)!;
  inp.value = value;
  inp.dispatchEvent(new Event("input", { bubbles: true }));
}

describe("<settings-form> unresolvable-PAN fail-closed (client-side)", () => {
  test("clearing both MAC and PAN-override: Save disabled, banner shown", async () => {
    // Both MAC and override empty resolve to NO client-side PAN (the radio
    // would fall back to the live eth0 MAC, which the browser can't preview),
    // so a sensitive change to that state is fail-closed.
    const el = await mount({
      ecu_id: "",
      mac: "aa:bb:cc:dd:ee:ff",
      pan_override: "",
      zigbee_type: "apsystems",
    });
    const root = el.shadowRoot!;
    setInput(root, "mac", ""); // clear the MAC -> no resolvable PAN
    await el.updateComplete;

    const save = root.querySelector<HTMLButtonElement>("button.save")!;
    expect(save.disabled).toBe(true);
    expect(root.textContent ?? "").toContain("Cannot resolve effective PAN");

    // Clicking Save anyway must NOT dispatch.
    let dispatched = false;
    el.addEventListener("save", () => (dispatched = true));
    save.click();
    await el.updateComplete;
    expect(dispatched).toBe(false);
  });

  test("no MAC/override + only ecu_id change: Save enabled (non-sensitive)", async () => {
    const el = await mount({
      ecu_id: "",
      mac: "",
      pan_override: "",
      zigbee_type: "apsystems",
    });
    const root = el.shadowRoot!;
    // ECU ID isn't a step-up-gated field; the form should let Save through
    // even with no resolvable effective PAN.
    const save = root.querySelector<HTMLButtonElement>("button.save")!;
    expect(save.disabled).toBe(false);

    let detail: Settings | null = null;
    el.addEventListener("save", (e) => {
      detail = (e as CustomEvent<Settings>).detail;
    });
    save.click();
    await el.updateComplete;
    expect(detail).not.toBeNull();
  });

  test("MAC change to a new colon MAC: Save enabled (will open confirm)", async () => {
    const el = await mount({
      ecu_id: "",
      mac: "aa:bb:cc:dd:00:01",
      pan_override: "",
      zigbee_type: "apsystems",
    });
    const root = el.shadowRoot!;
    setInput(root, "mac", "aa:bb:cc:dd:ee:ff");
    await el.updateComplete;

    const save = root.querySelector<HTMLButtonElement>("button.save")!;
    expect(save.disabled).toBe(false);

    save.click();
    await el.updateComplete;
    // PAN diverges (0001 -> EEFF), so the confirm dialog opens.
    expect(root.querySelector(".backdrop")).not.toBeNull();
  });
});

describe("<settings-form> MAC-change network-drop warning", () => {
  // Same in-place stub as the confirm-dialog suite so api.verifyPassword
  // doesn't reach the network.
  let restorePwd: (() => void) | null = null;

  beforeEach(async () => {
    const mod = await import("../src/api.ts");
    const orig = mod.api.verifyPassword;
    mod.api.verifyPassword = async () => true;
    restorePwd = () => {
      mod.api.verifyPassword = orig;
    };
  });

  afterEach(() => {
    if (restorePwd) restorePwd();
    restorePwd = null;
    document.body.querySelectorAll(".backdrop").forEach((n) => n.remove());
  });

  test("confirm dialog shows the network-drop warning when MAC is the active diff", async () => {
    const el = await mount({
      ecu_id: "",
      mac: "aa:bb:cc:dd:00:01",
      pan_override: "",
      zigbee_type: "apsystems",
    });
    const root = el.shadowRoot!;
    setInput(root, "mac", "aa:bb:cc:dd:ee:ff");
    await el.updateComplete;
    root.querySelector<HTMLButtonElement>("button.save")!.click();
    await el.updateComplete;

    const text = root.textContent ?? "";
    expect(text).toContain("drops the network for a few seconds");
  });

  test("confirm dialog OMITS the warning when the only sensitive change is pan_override", async () => {
    const el = await mount({
      ecu_id: "",
      mac: "aa:bb:cc:dd:ee:ff",
      pan_override: "0DCE",
      zigbee_type: "apsystems",
    });
    const root = el.shadowRoot!;
    setInput(root, "pan_override", "1234");
    await el.updateComplete;
    root.querySelector<HTMLButtonElement>("button.save")!.click();
    await el.updateComplete;

    // The dialog opens (PAN diverges), but the MAC didn't change, so the
    // network-drop warning must not appear.
    expect(root.querySelector(".backdrop")).not.toBeNull();
    const text = root.textContent ?? "";
    expect(text).not.toContain("drops the network for a few seconds");
  });
});

describe("<settings-form> ZigBee channel", () => {
  function withEffectiveChannel(channel?: number): Settings {
    return {
      ecu_id: "e",
      mac: "aa:bb:cc:dd:ee:ff",
      pan_override: "",
      zigbee_type: "apsystems",
      channel: channel,
    };
  }

  test("renders the persisted channel and the effective channel hint", async () => {
    // Effective channel is computed CLIENT-SIDE: settings.channel || 16.
    const el = await mount(withEffectiveChannel(20));
    const root = el.shadowRoot!;
    expect(root.querySelector<HTMLInputElement>("#channel")!.value).toBe("20");
    expect(root.textContent ?? "").toContain("effective: 20");
  });

  test("channel 0 / undefined renders empty (auto) and effective defaults to 16", async () => {
    const el = await mount(withEffectiveChannel(0));
    const root = el.shadowRoot!;
    expect(root.querySelector<HTMLInputElement>("#channel")!.value).toBe("");
    expect(root.textContent ?? "").toContain("effective: 16");
  });

  test("a valid channel persists and is carried in the save detail", async () => {
    const el = await mount(withEffectiveChannel(16));
    const root = el.shadowRoot!;
    setInput(root, "channel", "21");
    await el.updateComplete;

    let detail: Settings | null = null;
    el.addEventListener("save", (e) => (detail = (e as CustomEvent<Settings>).detail));
    root.querySelector<HTMLButtonElement>("button.save")!.click();
    await el.updateComplete;

    expect(detail).not.toBeNull();
    expect(detail!.channel).toBe(21);
  });

  test("empty channel is allowed and saves as 0 (derive/default)", async () => {
    const el = await mount(withEffectiveChannel(16));
    const root = el.shadowRoot!;
    setInput(root, "channel", "");
    await el.updateComplete;

    const save = root.querySelector<HTMLButtonElement>("button.save")!;
    expect(save.disabled).toBe(false);
    expect(root.textContent ?? "").not.toContain("Channel must be");

    let detail: Settings | null = null;
    el.addEventListener("save", (e) => (detail = (e as CustomEvent<Settings>).detail));
    save.click();
    await el.updateComplete;
    expect(detail).not.toBeNull();
    expect(detail!.channel).toBe(0);
  });

  for (const bad of ["10", "27"]) {
    test(`out-of-range channel "${bad}" is rejected: inline error, Save disabled, no dispatch`, async () => {
      const el = await mount(withEffectiveChannel(16));
      const root = el.shadowRoot!;
      setInput(root, "channel", bad);
      await el.updateComplete;

      expect(root.textContent ?? "").toContain("Channel must be empty");
      const save = root.querySelector<HTMLButtonElement>("button.save")!;
      expect(save.disabled).toBe(true);

      let dispatched = false;
      el.addEventListener("save", () => (dispatched = true));
      save.click();
      await el.updateComplete;
      expect(dispatched).toBe(false);
    });
  }

  // A type=number input coerces non-numeric text ("abc") to "" before it
  // reaches the form, so the validator is exercised directly here.
  test("non-numeric channel is rejected by the validator", () => {
    expect(channelInputInvalid("abc")).toBe(true);
    expect(channelInputInvalid("12.5")).toBe(true);
    expect(channelInputInvalid("")).toBe(false);
    expect(channelInputInvalid("11")).toBe(false);
    expect(channelInputInvalid("26")).toBe(false);
    expect(channelInputInvalid("10")).toBe(true);
    expect(channelInputInvalid("27")).toBe(true);
  });
});

describe("<settings-form> MAC strictness (Go-side)", () => {
  test("bare-hex MAC input: inline error + Save disabled, no PAN computed", async () => {
    const el = await mount({
      ecu_id: "",
      mac: "aa:bb:cc:dd:ee:ff",
      pan_override: "",
      zigbee_type: "apsystems",
    });
    const root = el.shadowRoot!;
    setInput(root, "mac", "aabbccddeeff"); // bare hex — rejected by the Go validator
    await el.updateComplete;

    expect(root.textContent ?? "").toContain("Use colon-separated hex");
    const save = root.querySelector<HTMLButtonElement>("button.save")!;
    expect(save.disabled).toBe(true);

    // Save click should be a no-op.
    let dispatched = false;
    el.addEventListener("save", () => (dispatched = true));
    save.click();
    await el.updateComplete;
    expect(dispatched).toBe(false);
  });

  test("colon-separated MAC: accepted, PAN computed", async () => {
    const el = await mount({
      ecu_id: "",
      mac: "aa:bb:cc:dd:00:01",
      pan_override: "",
      zigbee_type: "apsystems",
    });
    const root = el.shadowRoot!;
    setInput(root, "mac", "aa:bb:cc:dd:ee:ff");
    await el.updateComplete;

    expect(root.textContent ?? "").not.toContain("Use colon-separated hex");
    const save = root.querySelector<HTMLButtonElement>("button.save")!;
    expect(save.disabled).toBe(false);

    // Clicking Save opens the confirm dialog (PAN diverges 0001 → EEFF).
    save.click();
    await el.updateComplete;
    expect(root.querySelector(".backdrop")).not.toBeNull();
    expect(root.textContent ?? "").toContain("EEFF");
  });
});

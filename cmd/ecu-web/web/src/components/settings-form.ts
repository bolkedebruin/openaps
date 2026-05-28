import { LitElement, html, css, nothing } from "lit";
import { api, type Settings } from "../api.ts";

/**
 * <settings-form .settings=${s} .hostname=${h}> renders the editable ECU
 * settings. Below each ZigBee-coupled field it shows the effective value (the
 * one the radio is actually using right now). When the operator types a value
 * that would change the effective PAN, Save opens a confirm dialog requiring
 * password re-entry before the change is dispatched.
 *
 * On Save it dispatches a bubbling, composed "save" event whose detail is the
 * Settings object — same contract as before; the dialog is local to the form.
 */
export class SettingsForm extends LitElement {
  static properties = {
    settings: { attribute: false },
    hostname: { attribute: false },
    confirming: { state: true },
    pendingDetail: { state: true },
    pwdError: { state: true },
    pwdBusy: { state: true },
    typedMac: { state: true },
    typedPan: { state: true },
    typedChannel: { state: true },
  };
  declare settings: Settings;
  declare hostname: string;
  declare confirming: boolean;
  declare pendingDetail: Settings | null;
  declare pwdError: string;
  declare pwdBusy: boolean;
  // typedMac / typedPan / typedChannel mirror the live input values so
  // render() can compute the Save-button-disabled state without querying
  // the DOM (which Lit can't depend on for re-renders).
  declare typedMac: string;
  declare typedPan: string;
  declare typedChannel: string;

  constructor() {
    super();
    this.settings = { ecu_id: "", mac: "", pan_override: "", zigbee_type: "apsystems" };
    this.hostname = "";
    this.confirming = false;
    this.pendingDetail = null;
    this.pwdError = "";
    this.pwdBusy = false;
    this.typedMac = "";
    this.typedPan = "";
    this.typedChannel = "";
  }

  willUpdate(changed: Map<string, unknown>) {
    if (changed.has("settings")) {
      // Reset the typed mirrors to the persisted values whenever a fresh
      // settings object lands (initial load, post-save refetch).
      this.typedMac = this.settings.mac ?? "";
      this.typedPan = this.settings.pan_override ?? "";
      this.typedChannel = channelToInput(this.settings.channel);
    }
  }

  static styles = css`
    :host { display: block; }
    .grid { display: grid; gap: 18px; max-width: 460px; }
    label { display: flex; flex-direction: column; gap: 6px; font-size: 13px; color: var(--muted); }
    input, select {
      background: var(--bar-bg);
      border: 1px solid var(--border);
      color: var(--text);
      border-radius: 8px;
      padding: 9px 11px;
      font-size: 14px;
      font-family: inherit;
    }
    input:focus, select:focus { outline: none; border-color: var(--accent); }
    .hint { font-size: 12px; color: var(--muted); margin-top: -2px; }
    .err-inline {
      font-size: 12px;
      color: var(--err);
      margin-top: -2px;
    }
    .banner.err {
      color: var(--err);
      border: 1px solid var(--err);
      background: color-mix(in srgb, var(--err) 12%, transparent);
      border-radius: 8px;
      padding: 9px 11px;
      font-size: 13px;
    }
    button.save:disabled {
      opacity: 0.55;
      cursor: not-allowed;
    }
    .actions { display: flex; gap: 12px; margin-top: 4px; }
    button.save {
      background: var(--accent);
      border: none;
      color: #04121a;
      border-radius: 8px;
      padding: 9px 18px;
      font-size: 14px;
      font-weight: 600;
      cursor: pointer;
    }
    button.save:hover { filter: brightness(1.08); }

    .backdrop {
      position: fixed;
      inset: 0;
      background: rgba(0, 0, 0, 0.55);
      display: flex;
      align-items: center;
      justify-content: center;
      z-index: 1000;
    }
    .dialog {
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: 10px;
      padding: 20px 22px;
      max-width: 440px;
      width: 92%;
      color: var(--text);
      box-shadow: 0 12px 40px rgba(0, 0, 0, 0.5);
    }
    .dialog h3 { margin: 0 0 10px; font-size: 15px; }
    .dialog p { margin: 0 0 14px; font-size: 13px; color: var(--muted); line-height: 1.45; }
    .dialog label { display: block; font-size: 12px; color: var(--muted); margin: 4px 0 6px; }
    .dialog input {
      width: 100%;
      box-sizing: border-box;
      padding: 9px 11px;
      background: var(--bar-bg);
      border: 1px solid var(--border);
      border-radius: 8px;
      color: var(--text);
      font: inherit;
    }
    .dialog .err {
      color: var(--err);
      border: 1px solid var(--err);
      background: color-mix(in srgb, var(--err) 12%, transparent);
      border-radius: 8px;
      padding: 8px 10px;
      font-size: 12px;
      margin-top: 10px;
    }
    .dialog p.warn {
      color: var(--text);
      border: 1px solid var(--accent);
      background: color-mix(in srgb, var(--accent) 10%, transparent);
      border-radius: 8px;
      padding: 8px 10px;
      font-size: 12px;
      margin: 0 0 12px;
    }
    .dialog .row {
      display: flex;
      gap: 10px;
      justify-content: flex-end;
      margin-top: 16px;
    }
    .dialog button {
      padding: 8px 14px;
      border-radius: 8px;
      font-size: 13px;
      font-weight: 600;
      cursor: pointer;
    }
    .dialog button.primary { background: var(--accent); color: #04121a; border: none; }
    .dialog button.secondary { background: transparent; color: var(--muted); border: 1px solid var(--border); }
    .dialog button:disabled { opacity: 0.6; cursor: default; }
  `;

  // currentDetail reads the live form values and returns the would-be Settings.
  private currentDetail(): Settings {
    const root = this.shadowRoot;
    const val = (id: string) =>
      (root?.querySelector<HTMLInputElement | HTMLSelectElement>(`#${id}`)?.value ?? "").trim();
    return {
      ecu_id: val("ecu_id"),
      mac: val("mac"),
      pan_override: val("pan_override"),
      zigbee_type: val("zigbee_type"),
      channel: channelFromInput(val("channel")),
    };
  }

  // computeEffectivePAN derives the effective PAN given typed settings,
  // falling back to the current effective MAC for the empty-MAC case.
  // Returns a 4-uppercase-hex string, or "" when nothing usable is available.
  private computeEffectivePAN(d: Settings): string {
    const pan = panFromOverride(d.pan_override);
    if (pan) return pan;
    const macSource = d.mac || this.settings.effective?.mac || "";
    return panFromMAC(macSource);
  }

  // sensitiveChange reports whether d changes a step-up-gated field (mac
  // or pan_override) relative to the persisted settings. Sensitive
  // changes can't be saved when the backend hasn't resolved an effective
  // PAN — the form would otherwise let the operator re-PAN the radio
  // without ever seeing the post-change PAN.
  private sensitiveChange(d: Settings): boolean {
    return d.mac !== (this.settings.mac ?? "") ||
      d.pan_override !== (this.settings.pan_override ?? "");
  }

  // macInputInvalid reports whether the operator changed the MAC into a
  // non-empty value that isn't a valid colon-separated MAC. (Empty is
  // allowed: the backend treats "" as "use the live MAC.") A change
  // FROM a historically-bare-hex value to itself isn't flagged.
  private macInputInvalid(d: Settings): boolean {
    if (d.mac === (this.settings.mac ?? "")) return false;
    return d.mac !== "" && !isColonMAC(d.mac);
  }

  private save = () => {
    if (channelInputInvalid(this.typedChannel)) {
      // Out-of-range channel: don't dispatch, the inline hint already shows.
      // Save is also disabled in render(); guard here for keyboard Save.
      return;
    }
    const detail = this.currentDetail();
    const sensitive = this.sensitiveChange(detail);
    if (sensitive && this.macInputInvalid(detail)) {
      // Bad input: don't dispatch, the inline validation hint already shows.
      return;
    }
    const currentPAN = (this.settings.effective?.pan ?? "").toUpperCase();
    if (sensitive && !currentPAN) {
      // Fail-closed: with no resolved effective PAN we can't show the
      // post-change PAN in the confirm dialog and can't tell whether the
      // change re-PANs the radio. The save button is disabled in render(),
      // but guard the handler too for keyboard-driven Save attempts.
      return;
    }
    const newPAN = this.computeEffectivePAN(detail);
    if (currentPAN && newPAN && newPAN !== currentPAN) {
      this.pendingDetail = detail;
      this.pwdError = "";
      this.confirming = true;
      // autofocus password input on next frame
      queueMicrotask(() => {
        const root = this.shadowRoot;
        root?.querySelector<HTMLInputElement>("#confirm_pwd")?.focus();
      });
      return;
    }
    this.dispatchSave(detail);
  };

  private dispatchSave(detail: Settings) {
    this.dispatchEvent(
      new CustomEvent<Settings>("save", { detail, bubbles: true, composed: true }),
    );
  }

  private confirmCancel = () => {
    this.confirming = false;
    this.pendingDetail = null;
    this.pwdError = "";
    this.pwdBusy = false;
  };

  private confirmSubmit = async () => {
    if (this.pwdBusy) return;
    const root = this.shadowRoot;
    const pwd = root?.querySelector<HTMLInputElement>("#confirm_pwd")?.value ?? "";
    if (!pwd) {
      this.pwdError = "Password required.";
      return;
    }
    this.pwdBusy = true;
    this.pwdError = "";
    try {
      const ok = await api.verifyPassword(pwd);
      if (!ok) {
        this.pwdError = "Wrong password.";
        return;
      }
      const detail = this.pendingDetail;
      this.confirming = false;
      this.pendingDetail = null;
      if (detail) this.dispatchSave(detail);
    } catch (e) {
      this.pwdError = (e as Error).message || "Verification failed.";
    } finally {
      this.pwdBusy = false;
    }
  };

  private onPwdKey = (e: KeyboardEvent) => {
    if (e.key === "Enter") {
      e.preventDefault();
      void this.confirmSubmit();
    }
  };

  render() {
    const s = this.settings;
    const eff = s.effective ?? {};
    const ecuPlaceholder = "e.g. the serial on the device label";
    const ecuInitial = s.ecu_id || this.hostname || "";
    const macHint = eff.mac ? `effective: ${eff.mac}` : "";
    const panHint = s.pan_override
      ? eff.pan
        ? `effective: ${eff.pan}`
        : ""
      : eff.pan
        ? `effective: ${eff.pan} (from MAC)`
        : "";
    const zbHint = s.zigbee_type ? "" : "effective: apsystems (default)";
    const chHint = eff.channel ? `effective: ${eff.channel}` : "";

    // Derive Save-disabled state from the live typed values. A sensitive
    // change (mac / pan_override differs from what's persisted) requires
    // a resolved effective PAN so the confirm dialog can show the
    // post-change PAN; a MAC change additionally requires a colon-MAC
    // input that matches the Go-side validator.
    const macChanged = this.typedMac !== (s.mac ?? "");
    const panChanged = this.typedPan !== (s.pan_override ?? "");
    const typedSensitive = macChanged || panChanged;
    const macInvalid = macChanged && this.typedMac !== "" && !isColonMAC(this.typedMac);
    const noEffectivePan = !(eff.pan ?? "");
    // The channel is not step-up-gated, but an out-of-range value must still
    // block Save (the Go validator only accepts 0 or 11..26).
    const channelInvalid = channelInputInvalid(this.typedChannel);
    const blockSensitive =
      channelInvalid || (typedSensitive && (macInvalid || noEffectivePan));

    let blockReason = "";
    if (macInvalid) {
      blockReason =
        "MAC must be 6 colon-separated hex octets (e.g. 80:97:1b:03:0d:ce).";
    } else if (typedSensitive && noEffectivePan) {
      blockReason =
        "Cannot resolve effective PAN; refusing to save MAC / PAN-override changes.";
    }

    return html`
      <div class="grid">
        <label>
          ECU ID
          <input
            id="ecu_id"
            type="text"
            placeholder=${ecuPlaceholder}
            .value=${ecuInitial}
          />
          ${!s.ecu_id
            ? html`<div class="hint">Recommended: use the serial on the device label.</div>`
            : nothing}
        </label>
        <label>
          MAC
          <input
            id="mac"
            type="text"
            placeholder="80:97:1b:03:0d:ce"
            pattern="^[0-9a-fA-F]{2}(:[0-9a-fA-F]{2}){5}$"
            .value=${s.mac ?? ""}
            @input=${this.onMacInput}
          />
          ${macHint ? html`<div class="hint">${macHint}</div>` : nothing}
          ${macInvalid
            ? html`<div class="err-inline">Use colon-separated hex (e.g. 80:97:1b:03:0d:ce).</div>`
            : nothing}
        </label>
        <label>
          PAN override
          <input
            id="pan_override"
            type="text"
            placeholder="auto from MAC"
            .value=${s.pan_override ?? ""}
            @input=${this.onPanInput}
          />
          ${panHint ? html`<div class="hint">${panHint}</div>` : nothing}
        </label>
        <label>
          ZigBee channel
          <input
            id="channel"
            type="number"
            min="11"
            max="26"
            step="1"
            placeholder="auto (16)"
            .value=${channelToInput(s.channel)}
            @input=${this.onChannelInput}
          />
          ${chHint ? html`<div class="hint">${chHint}</div>` : nothing}
          ${channelInvalid
            ? html`<div class="err-inline">Channel must be empty (auto) or an integer 11–26.</div>`
            : nothing}
        </label>
        <label>
          ZigBee type
          <select id="zigbee_type" .value=${s.zigbee_type || "apsystems"}>
            <option value="apsystems">apsystems</option>
            <option value="general">general</option>
          </select>
          ${zbHint ? html`<div class="hint">${zbHint}</div>` : nothing}
        </label>
        ${blockReason
          ? html`<div class="banner err">${blockReason}</div>`
          : nothing}
        <div class="actions">
          <button class="save" ?disabled=${blockSensitive} @click=${this.save}>
            Save
          </button>
        </div>
      </div>
      ${this.confirming ? this.renderDialog() : nothing}
    `;
  }

  private onMacInput = (e: Event) => {
    this.typedMac = (e.target as HTMLInputElement).value.trim();
  };

  private onPanInput = (e: Event) => {
    this.typedPan = (e.target as HTMLInputElement).value.trim();
  };

  private onChannelInput = (e: Event) => {
    this.typedChannel = (e.target as HTMLInputElement).value.trim();
  };

  private renderDialog() {
    const current = (this.settings.effective?.pan ?? "").toUpperCase();
    const next = this.pendingDetail ? this.computeEffectivePAN(this.pendingDetail) : "";
    // A MAC change reconfigures eth0 immediately (ip link down/up), so
    // the operator's HTTP session briefly drops. Warn before they commit.
    const macChanging =
      !!this.pendingDetail &&
      (this.pendingDetail.mac ?? "") !== (this.settings.mac ?? "");
    return html`
      <div class="backdrop" @click=${this.onBackdropClick}>
        <div class="dialog" role="dialog" aria-modal="true" @click=${this.stop}>
          <h3>Confirm PAN change</h3>
          <p>
            Effective PAN ${current || "—"} → ${next || "—"}. Inverters bonded to
            ${current || "the current PAN"} may stop responding.
          </p>
          ${macChanging
            ? html`<p class="warn">
                Applying a new MAC drops the network for a few seconds, up to
                ~15 s if the kernel is slow. Your browser may reconnect
                automatically; if not, refresh.
              </p>`
            : nothing}
          <label for="confirm_pwd">Password</label>
          <input
            id="confirm_pwd"
            type="password"
            autocomplete="current-password"
            @keydown=${this.onPwdKey}
            ?disabled=${this.pwdBusy}
          />
          ${this.pwdError ? html`<div class="err">${this.pwdError}</div>` : nothing}
          <div class="row">
            <button class="secondary" @click=${this.confirmCancel} ?disabled=${this.pwdBusy}>
              Cancel
            </button>
            <button class="primary" @click=${this.confirmSubmit} ?disabled=${this.pwdBusy}>
              Confirm
            </button>
          </div>
        </div>
      </div>
    `;
  }

  private onBackdropClick = () => this.confirmCancel();
  private stop = (e: Event) => e.stopPropagation();
}

/** panFromOverride normalises a 1-4 hex override to 4 uppercase hex chars. */
function panFromOverride(s: string): string {
  const t = (s || "").trim().replace(/^0x/i, "");
  if (!t) return "";
  if (!/^[0-9a-fA-F]{1,4}$/.test(t)) return "";
  return t.toUpperCase().padStart(4, "0");
}

/**
 * panFromMAC returns the lower-16 of a MAC as 4 uppercase hex, or "".
 *
 * Mirrors the Go server's panFromMAC: only 6-octet colon-separated MACs
 * are accepted (e.g. 80:97:1b:03:0d:ce). Bare-hex strings are rejected
 * so client and server reach the same verdict on the same input.
 */
function panFromMAC(mac: string): string {
  const t = (mac || "").trim();
  if (!t || !isColonMAC(t)) return "";
  const hex = t.replace(/:/g, "");
  return hex.slice(-4).toUpperCase();
}

/** isColonMAC reports whether s is a 6-octet colon-separated hex MAC. */
function isColonMAC(s: string): boolean {
  return /^[0-9a-fA-F]{2}(:[0-9a-fA-F]{2}){5}$/.test(s);
}

/**
 * channelToInput renders a persisted channel for the numeric input. 0 (or
 * undefined) means "derive/default", shown as an empty field; 11..26 show
 * the number itself.
 */
function channelToInput(ch: number | undefined): string {
  return ch && ch > 0 ? String(ch) : "";
}

/**
 * channelFromInput parses the channel input into the proto value. Empty
 * means "derive/default" → 0; otherwise the integer the operator typed.
 * Invalid input also maps to 0 (the save handler blocks before this runs).
 */
function channelFromInput(s: string): number {
  const t = (s || "").trim();
  if (!t) return 0;
  const n = Number(t);
  return Number.isInteger(n) ? n : 0;
}

/**
 * channelInputInvalid reports whether the typed channel is non-empty and
 * not an integer in 11..26. Empty is valid (= derive/default 16). Mirrors
 * the inv-driver validator that accepts channel ∈ {0, 11..26}.
 */
export function channelInputInvalid(s: string): boolean {
  const t = (s || "").trim();
  if (!t) return false;
  const n = Number(t);
  if (!Number.isInteger(n)) return true;
  return n < 11 || n > 26;
}

customElements.define("settings-form", SettingsForm);

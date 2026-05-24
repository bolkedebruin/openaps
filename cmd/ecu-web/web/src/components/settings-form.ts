import { LitElement, html, css } from "lit";
import type { Settings } from "../api.ts";

/**
 * <settings-form .settings=${s}> renders the editable ECU settings. On
 * Save it reads the current input values and dispatches a bubbling,
 * composed "save" event whose detail is the Settings object.
 */
export class SettingsForm extends LitElement {
  static properties = { settings: { attribute: false } };
  declare settings: Settings;

  constructor() {
    super();
    this.settings = { ecu_id: "", mac: "", pan_override: "", zigbee_type: "apsystems" };
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
  `;

  private save = () => {
    const root = this.shadowRoot;
    if (!root) return;
    const val = (id: string) =>
      (root.querySelector<HTMLInputElement | HTMLSelectElement>(`#${id}`)?.value ?? "").trim();
    const detail: Settings = {
      ecu_id: val("ecu_id"),
      mac: val("mac"),
      pan_override: val("pan_override"),
      zigbee_type: val("zigbee_type"),
    };
    this.dispatchEvent(new CustomEvent<Settings>("save", { detail, bubbles: true, composed: true }));
  };

  render() {
    const s = this.settings;
    return html`
      <div class="grid">
        <label>
          ECU ID
          <input id="ecu_id" type="text" .value=${s.ecu_id ?? ""} />
        </label>
        <label>
          MAC
          <input id="mac" type="text" .value=${s.mac ?? ""} />
        </label>
        <label>
          PAN override
          <input id="pan_override" type="text" placeholder="auto from MAC" .value=${s.pan_override ?? ""} />
        </label>
        <label>
          ZigBee type
          <select id="zigbee_type" .value=${s.zigbee_type || "apsystems"}>
            <option value="apsystems">apsystems</option>
            <option value="general">general</option>
          </select>
        </label>
        <div class="actions">
          <button class="save" @click=${this.save}>Save</button>
        </div>
      </div>
    `;
  }
}

customElements.define("settings-form", SettingsForm);

import { LitElement, html, css, nothing } from "lit";
import { api, type Inverter } from "../api.ts";
import { fmtW, fmtV, fmtHz, fmtPct, fmtA, faultLabels, ageLabel } from "../format.ts";
import "./cap-bar.ts";

/**
 * <inverter-card .inverter=${inv}> renders one inverter: live output vs
 * nameplate, the output-cap control (cap-bar), AC metrics, per-panel DC, and
 * any active fault chips.
 */
export class InverterCard extends LitElement {
  static properties = {
    inverter: { attribute: false },
    name: { type: String },
    profile: { type: String },
    legError: { state: true },
    legBusy: { state: true },
  };

  declare inverter: Inverter;
  declare name: string;
  declare profile: string;
  declare legError: string;
  declare legBusy: boolean;

  constructor() {
    super();
    this.name = "";
    this.profile = "";
    this.legError = "";
    this.legBusy = false;
  }

  private async setLeg(leg: number) {
    const inv = this.inverter;
    if (!inv || inv.phase === leg || this.legBusy) return;
    this.legBusy = true;
    this.legError = "";
    try {
      await api.setInverterPhase(inv.uid, leg);
      inv.phase = leg; // optimistic; the next fleet poll confirms
      this.dispatchEvent(new CustomEvent("inverter-updated", { bubbles: true, composed: true }));
    } catch (e) {
      this.legError = e instanceof Error ? e.message : String(e);
    } finally {
      this.legBusy = false;
    }
  }

  static styles = css`
    :host {
      display: block;
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: 10px;
      padding: 16px;
    }
    .head {
      display: flex;
      align-items: center;
      justify-content: space-between;
      margin-bottom: 12px;
    }
    .model { font-weight: 600; font-size: 15px; }
    .uid { color: var(--muted); font-size: 12px; font-family: var(--mono); }
    .profile {
      display: inline-flex;
      align-items: center;
      gap: 4px;
      margin-top: 6px;
      background: color-mix(in srgb, var(--accent) 16%, transparent);
      color: var(--accent);
      border: 1px solid color-mix(in srgb, var(--accent) 55%, transparent);
      border-radius: 999px;
      padding: 2px 9px;
      font-size: 11px;
      font-weight: 600;
    }
    .dot { width: 9px; height: 9px; border-radius: 50%; display: inline-block; margin-right: 6px; }
    .dot.on { background: var(--ok); box-shadow: 0 0 6px var(--ok); }
    .dot.off { background: var(--muted); }
    .state { font-size: 12px; color: var(--muted); }
    .power { display: flex; align-items: baseline; gap: 8px; }
    .pw { font-size: 28px; font-weight: 700; color: var(--text); }
    .cap { color: var(--muted); font-size: 13px; }
    cap-bar { margin: 10px 0 16px; }
    .metrics { display: grid; grid-template-columns: repeat(3, 1fr); gap: 8px; font-size: 13px; }
    .metric .k { color: var(--muted); font-size: 11px; }
    .metric .v { color: var(--text); font-weight: 600; }
    .panels { margin-top: 14px; display: grid; grid-template-columns: repeat(auto-fill, minmax(76px, 1fr)); gap: 6px; }
    .panel { background: var(--bar-bg); border-radius: 6px; padding: 6px 8px; font-size: 11px; }
    .panel .pi { color: var(--muted); }
    .panel .pw { font-size: 13px; }
    .chips { margin-top: 12px; display: flex; flex-wrap: wrap; gap: 6px; }
    .chip {
      background: color-mix(in srgb, var(--err) 20%, transparent);
      color: var(--err);
      border: 1px solid var(--err);
      border-radius: 999px;
      padding: 2px 8px;
      font-size: 11px;
    }
    .leg { margin-top: 14px; display: flex; align-items: center; gap: 10px; }
    .leg .k { color: var(--muted); font-size: 11px; }
    .legbtns { display: inline-flex; gap: 4px; }
    .legbtn {
      background: var(--bar-bg);
      color: var(--text);
      border: 1px solid var(--border);
      border-radius: 6px;
      padding: 3px 10px;
      font-size: 12px;
      font-weight: 600;
      cursor: pointer;
    }
    .legbtn.sel {
      background: color-mix(in srgb, var(--accent) 20%, transparent);
      color: var(--accent);
      border-color: color-mix(in srgb, var(--accent) 55%, transparent);
    }
    .legbtn:disabled { opacity: 0.5; cursor: default; }
    .three { color: var(--muted); font-size: 12px; }
    .leg-err { margin-top: 6px; color: var(--err); font-size: 11px; }
  `;

  render() {
    const inv = this.inverter;
    if (!inv) return nothing;
    const faults = faultLabels(inv.faults);
    return html`
      <div class="head">
        <div>
          <div class="model">${this.name || inv.model || "unknown"}</div>
          <div class="uid">${this.name ? `${inv.model} · ${inv.uid}` : inv.uid}</div>
          ${this.profile
            ? html`<div class="profile" title="Local Site profile active">⚙ ${this.profile}</div>`
            : nothing}
        </div>
        <div class="state">
          <span class="dot ${inv.online ? "on" : "off"}"></span>
          ${inv.online ? "online" : "offline"} · ${ageLabel(inv.age_s)}
        </div>
      </div>

      <div class="power">
        <span class="pw">${fmtW(inv.active_power_w)}</span>
        <span class="cap">/ ${fmtW(inv.nameplate_w)} · ${fmtPct(inv.load_pct)}</span>
      </div>
      <cap-bar .inverter=${inv}></cap-bar>

      <div class="metrics">
        <div class="metric"><div class="k">Grid</div><div class="v">${fmtV(inv.grid_v)}</div></div>
        <div class="metric"><div class="k">Freq</div><div class="v">${fmtHz(inv.freq_hz)}</div></div>
        <div class="metric"><div class="k">RSSI / LQI</div><div class="v">${inv.rssi} / ${inv.lqi}</div></div>
      </div>

      ${inv.panels?.length
        ? html`<div class="panels">
            ${inv.panels.map(
              (p) => html`<div class="panel">
                <div class="pi">DC ${p.index + 1}</div>
                <div class="pw">${fmtW(p.w)}</div>
                <div>${fmtV(p.dc_v)} · ${fmtA(p.dc_a)}</div>
              </div>`,
            )}
          </div>`
        : nothing}

      ${faults.length
        ? html`<div class="chips">
            ${faults.map((f) => html`<span class="chip">${f}</span>`)}
          </div>`
        : nothing}

      <div class="leg">
        <span class="k">Grid leg</span>
        ${inv.three_phase
          ? html`<span class="three">3-phase · L1·L2·L3</span>`
          : html`<span class="legbtns">
              ${[1, 2, 3].map(
                (l) => html`<button
                  class="legbtn ${inv.phase === l ? "sel" : ""}"
                  ?disabled=${this.legBusy}
                  @click=${() => this.setLeg(l)}
                >
                  L${l}
                </button>`,
              )}
            </span>`}
      </div>
      ${this.legError ? html`<div class="leg-err">${this.legError}</div>` : nothing}
    `;
  }
}

customElements.define("inverter-card", InverterCard);

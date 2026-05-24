import { LitElement, html, css, nothing } from "lit";
import type { Inverter } from "../api.ts";
import { fmtW, fmtV, fmtHz, fmtPct, fmtA, loadClass, faultLabels, ageLabel } from "../format.ts";

/**
 * <inverter-card .inverter=${inv}> renders one inverter: live output vs
 * nameplate cap, AC metrics, per-panel DC, and any active fault chips.
 */
export class InverterCard extends LitElement {
  static properties = {
    inverter: { attribute: false },
    name: { type: String },
  };

  declare inverter: Inverter;
  declare name: string;

  constructor() {
    super();
    this.name = "";
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
    .model {
      font-weight: 600;
      font-size: 15px;
    }
    .uid {
      color: var(--muted);
      font-size: 12px;
      font-family: var(--mono);
    }
    .dot {
      width: 9px;
      height: 9px;
      border-radius: 50%;
      display: inline-block;
      margin-right: 6px;
    }
    .dot.on {
      background: var(--ok);
      box-shadow: 0 0 6px var(--ok);
    }
    .dot.off {
      background: var(--muted);
    }
    .state {
      font-size: 12px;
      color: var(--muted);
    }
    .power {
      display: flex;
      align-items: baseline;
      gap: 8px;
    }
    .pw {
      font-size: 28px;
      font-weight: 700;
      color: var(--text);
    }
    .cap {
      color: var(--muted);
      font-size: 13px;
    }
    .bar {
      height: 8px;
      background: var(--bar-bg);
      border-radius: 4px;
      overflow: hidden;
      margin: 10px 0 14px;
    }
    .fill {
      height: 100%;
      border-radius: 4px;
      transition: width 0.4s ease;
    }
    .fill.low { background: var(--ok); }
    .fill.mid { background: var(--accent); }
    .fill.high { background: var(--warn); }
    .fill.idle { background: var(--muted); }
    .metrics {
      display: grid;
      grid-template-columns: repeat(3, 1fr);
      gap: 8px;
      font-size: 13px;
    }
    .metric .k { color: var(--muted); font-size: 11px; }
    .metric .v { color: var(--text); font-weight: 600; }
    .panels {
      margin-top: 14px;
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(76px, 1fr));
      gap: 6px;
    }
    .panel {
      background: var(--bar-bg);
      border-radius: 6px;
      padding: 6px 8px;
      font-size: 11px;
    }
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
  `;

  render() {
    const inv = this.inverter;
    if (!inv) return nothing;
    const lc = loadClass(inv.load_pct);
    const faults = faultLabels(inv.faults);
    const width = Math.max(0, Math.min(100, inv.load_pct));
    return html`
      <div class="head">
        <div>
          <div class="model">${this.name || inv.model || "unknown"}</div>
          <div class="uid">${this.name ? `${inv.model} · ${inv.uid}` : inv.uid}</div>
        </div>
        <div class="state">
          <span class="dot ${inv.online ? "on" : "off"}"></span>
          ${inv.online ? "online" : "offline"} · ${ageLabel(inv.age_s)}
        </div>
      </div>

      <div class="power">
        <span class="pw">${fmtW(inv.active_power_w)}</span>
        <span class="cap">/ ${inv.nameplate_w} W · ${fmtPct(inv.load_pct)}</span>
      </div>
      <div class="bar"><div class="fill ${lc}" style="width:${width}%"></div></div>

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
    `;
  }
}

customElements.define("inverter-card", InverterCard);

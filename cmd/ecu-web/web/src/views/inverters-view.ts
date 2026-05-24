import { LitElement, html, css } from "lit";
import type { Fleet } from "../api.ts";
import { fmtW, fmtV, fmtHz, fmtPct } from "../format.ts";

/**
 * <inverters-view .fleet=${} .names=${}> — dense per-inverter table.
 * The first column is an editable label that, on change, dispatches a
 * bubbling "rename" event ({uid, name}) for the shell to persist.
 */
export class InvertersView extends LitElement {
  static properties = {
    fleet: { attribute: false },
    names: { attribute: false },
  };
  declare fleet: Fleet | null;
  declare names: Record<string, string>;

  constructor() {
    super();
    this.fleet = null;
    this.names = {};
  }

  private rename(uid: string, e: Event) {
    const name = (e.target as HTMLInputElement).value;
    this.dispatchEvent(new CustomEvent("rename", { detail: { uid, name }, bubbles: true, composed: true }));
  }

  static styles = css`
    :host { display: block; }
    table { width: 100%; border-collapse: collapse; font-size: 13px; }
    th, td { text-align: left; padding: 10px 12px; border-bottom: 1px solid var(--border); }
    th { color: var(--muted); text-transform: uppercase; font-size: 11px; letter-spacing: 0.04em; }
    td { color: var(--text); }
    .uid { font-family: var(--mono); color: var(--muted); font-size: 11px; }
    .name-in {
      background: transparent;
      border: 1px solid transparent;
      border-radius: 6px;
      color: var(--text);
      font: inherit;
      padding: 3px 6px;
      width: 150px;
    }
    .name-in:hover { border-color: var(--border); }
    .name-in:focus { outline: none; border-color: var(--accent); background: var(--bar-bg); }
    .dot { width: 8px; height: 8px; border-radius: 50%; display: inline-block; margin-right: 6px; }
    .dot.on { background: var(--ok); }
    .dot.off { background: var(--muted); }
    .num { text-align: right; font-variant-numeric: tabular-nums; }
    .fw { font-variant-numeric: tabular-nums; color: var(--muted); }
    .fault { color: var(--err); }
    .empty { color: var(--muted); padding: 32px; text-align: center; }
  `;

  render() {
    const f = this.fleet;
    if (!f || f.inverters.length === 0) {
      return html`<div class="empty">No inverters discovered yet.</div>`;
    }
    return html`
      <table>
        <thead>
          <tr>
            <th>Inverter ID</th><th>Name</th><th>Model</th><th>Firmware</th><th>Status</th>
            <th class="num">Output</th><th class="num">Load</th>
            <th class="num">Grid</th><th class="num">Freq</th>
            <th class="num">Panels</th><th class="num">Faults</th>
          </tr>
        </thead>
        <tbody>
          ${f.inverters.map((inv) => {
            const nFaults = inv.faults ? Object.values(inv.faults).filter(Boolean).length : 0;
            return html`<tr>
              <td class="uid">${inv.uid}</td>
              <td>
                <input
                  class="name-in"
                  .value=${this.names?.[inv.uid] ?? ""}
                  placeholder="add a name"
                  @change=${(e: Event) => this.rename(inv.uid, e)}
                />
              </td>
              <td>${inv.model || "—"}</td>
              <td class="fw">${inv.sw_version || "—"}</td>
              <td>
                <span class="dot ${inv.online ? "on" : "off"}"></span>${inv.online ? "online" : "offline"}
              </td>
              <td class="num">${fmtW(inv.active_power_w)} / ${inv.nameplate_w} W</td>
              <td class="num">${fmtPct(inv.load_pct)}</td>
              <td class="num">${fmtV(inv.grid_v)}</td>
              <td class="num">${fmtHz(inv.freq_hz)}</td>
              <td class="num">${inv.panels?.length ?? 0}</td>
              <td class="num ${nFaults ? "fault" : ""}">${nFaults || "—"}</td>
            </tr>`;
          })}
        </tbody>
      </table>
    `;
  }
}

customElements.define("inverters-view", InvertersView);

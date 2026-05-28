import { LitElement, html, css, nothing } from "lit";
import { api, type Fleet, type SystemStatus } from "../api.ts";
import { fmtWh, fmtW } from "../format.ts";
import { capFloorW, capCeilW, readbackCapW } from "../power.ts";
import type { PowerPoint } from "../components/power-chart.ts";
import "../components/fleet-gauge.ts";
import "../components/stat-card.ts";
import "../components/inverter-card.ts";
import "../components/ecu-clients-card.ts";
import "../components/power-chart.ts";

/**
 * <dashboard-view .fleet=${} .system=${}> is the landing screen: an ECU
 * + connected-clients summary, the fleet output gauge, a multi-day power
 * chart, energy stats, and the per-inverter cards. It fetches the power
 * history itself (the gauge/cards stay live via the SSE-fed fleet prop).
 */
export class DashboardView extends LitElement {
  static properties = {
    fleet: { attribute: false },
    system: { attribute: false },
    names: { attribute: false },
    profiles: { attribute: false },
    history: { state: true },
    arrayPendingCap: { state: true },
    arrayBusy: { state: true },
    arrayError: { state: true },
  };

  declare fleet: Fleet | null;
  declare system: SystemStatus | null;
  declare names: Record<string, string>;
  declare profiles: Record<string, string>;
  declare history: PowerPoint[];
  declare arrayPendingCap: number | null; // optimistic array cap until read-back
  declare arrayBusy: boolean;
  declare arrayError: string;

  private timer: ReturnType<typeof setInterval> | null = null;

  constructor() {
    super();
    this.fleet = null;
    this.system = null;
    this.names = {};
    this.profiles = {};
    this.history = [];
    this.arrayPendingCap = null;
    this.arrayBusy = false;
    this.arrayError = "";
  }

  private setArrayCap = async (e: Event) => {
    const watts = Math.round(Number((e.target as HTMLInputElement).value));
    if (!Number.isFinite(watts) || watts <= 0) return;
    this.arrayPendingCap = watts;
    this.arrayBusy = true;
    this.arrayError = "";
    try {
      const r = await api.setPower({ array: true, watts });
      const failed = (r.results ?? []).filter((x) => !x.ok);
      if (failed.length) {
        this.arrayError = `${failed.length} inverter(s) failed`;
      } else {
        const applied = (r.results ?? []).reduce((s, x) => s + x.applied_watts, 0);
        if (applied) this.arrayPendingCap = applied;
      }
    } catch (err) {
      this.arrayError = (err as Error).message || "failed";
    } finally {
      this.arrayBusy = false;
    }
  };

  connectedCallback(): void {
    super.connectedCallback();
    void this.loadHistory();
    this.timer = setInterval(() => void this.loadHistory(), 60000);
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    if (this.timer) clearInterval(this.timer);
    this.timer = null;
  }

  private async loadHistory() {
    try {
      this.history = await api.history();
    } catch {
      /* keep last history on a transient fetch error */
    }
  }

  // chartPoints is the recorded history plus a live "now" tip so the
  // chart's right edge tracks the SSE fleet value between history refreshes.
  private chartPoints(): PowerPoint[] {
    if (!this.fleet) return this.history;
    return [...this.history, { t: Date.now(), w: this.fleet.active_power_w }];
  }

  static styles = css`
    :host { display: block; }
    .grid {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 16px;
      margin-bottom: 16px;
    }
    .panel {
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: 10px;
      padding: 16px;
    }
    h2 { font-size: 13px; text-transform: uppercase; letter-spacing: 0.05em; color: var(--muted); margin: 0 0 14px; }
    .chart { margin-bottom: 16px; }
    .stats { display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px; margin-bottom: 16px; }
    .online { text-align: center; color: var(--muted); font-size: 12px; margin-top: 10px; }
    .arrcap { margin-top: 16px; }
    .arrcap label { display: block; color: var(--muted); font-size: 11px; margin-bottom: 6px; }
    .arrcap-row { display: flex; align-items: center; gap: 10px; }
    .arrcap input {
      width: 130px;
      box-sizing: border-box;
      padding: 9px 12px;
      background: var(--bar-bg);
      border: 1px solid var(--border);
      border-radius: 8px;
      color: var(--text);
      font-size: 14px;
    }
    .arrcap input:disabled { opacity: 0.6; }
    .arrcap-max { color: var(--muted); font-size: 13px; }
    .caperr { color: var(--err); font-size: 12px; margin-top: 8px; }
    .cards { display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: 16px; }
    .empty { color: var(--muted); padding: 32px; text-align: center; }
    @media (max-width: 720px) { .grid, .stats { grid-template-columns: 1fr; } }
  `;

  render() {
    const f = this.fleet;
    if (!f) return html`<div class="empty">Waiting for inv-driver…</div>`;
    const arrMax = f.inverters.reduce((s, inv) => s + capCeilW(inv), 0);
    const arrFloor = f.inverters.reduce((s, inv) => s + capFloorW(inv), 0);
    const arrReadback = f.inverters.reduce((s, inv) => s + (readbackCapW(inv) ?? capCeilW(inv)), 0);
    const arrCap = this.arrayPendingCap ?? arrReadback;
    return html`
      <div class="grid">
        <div class="panel">
          <h2>Array output</h2>
          <fleet-gauge .power=${f.active_power_w} .cap=${f.nameplate_total_w}></fleet-gauge>
          <div class="online">${f.online_count} / ${f.inverter_count} inverters online</div>
          ${arrMax > 0
            ? html`<div class="arrcap">
                <label for="arrcap">Total output cap</label>
                <div class="arrcap-row">
                  <input
                    id="arrcap"
                    type="number"
                    min=${arrFloor}
                    max=${arrMax}
                    step="10"
                    .value=${String(arrCap)}
                    ?disabled=${f.online_count === 0 || this.arrayBusy}
                    @change=${this.setArrayCap}
                  />
                  <span class="arrcap-max">W / ${fmtW(arrMax)}</span>
                </div>
                ${this.arrayError ? html`<div class="caperr">⚠ ${this.arrayError}</div>` : nothing}
              </div>`
            : nothing}
        </div>
        <div class="panel">
          <h2>ECU &amp; clients</h2>
          <ecu-clients-card .system=${this.system}></ecu-clients-card>
        </div>
      </div>

      <div class="panel chart">
        <h2>Output</h2>
        <power-chart .points=${this.chartPoints()}></power-chart>
      </div>

      <div class="stats">
        <stat-card label="Today" value=${fmtWh(f.today_wh)}></stat-card>
        <stat-card label="This month" value=${fmtWh(f.month_wh)}></stat-card>
        <stat-card label="This year" value=${fmtWh(f.year_wh)}></stat-card>
        <stat-card label="Lifetime" value=${fmtWh(f.lifetime_wh)}></stat-card>
      </div>

      <h2>Inverters</h2>
      ${f.inverters.length
        ? html`<div class="cards">
            ${f.inverters.map(
              (inv) => html`<inverter-card
                .inverter=${inv}
                .name=${this.names?.[inv.uid] ?? ""}
                .profile=${this.profiles?.[inv.uid] ?? ""}
              ></inverter-card>`,
            )}
          </div>`
        : html`<div class="empty">No inverters discovered yet.</div>`}
      ${nothing}
    `;
  }
}

customElements.define("dashboard-view", DashboardView);

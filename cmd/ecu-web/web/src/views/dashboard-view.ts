import { LitElement, html, css, nothing } from "lit";
import { api, type Fleet, type SystemStatus } from "../api.ts";
import { fmtWh } from "../format.ts";
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
    history: { state: true },
  };

  declare fleet: Fleet | null;
  declare system: SystemStatus | null;
  declare names: Record<string, string>;
  declare history: PowerPoint[];

  private timer: ReturnType<typeof setInterval> | null = null;

  constructor() {
    super();
    this.fleet = null;
    this.system = null;
    this.names = {};
    this.history = [];
  }

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
    .cards { display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: 16px; }
    .empty { color: var(--muted); padding: 32px; text-align: center; }
    @media (max-width: 720px) { .grid, .stats { grid-template-columns: 1fr; } }
  `;

  render() {
    const f = this.fleet;
    if (!f) return html`<div class="empty">Waiting for inv-driver…</div>`;
    return html`
      <div class="grid">
        <div class="panel">
          <h2>Array output</h2>
          <fleet-gauge .power=${f.active_power_w} .cap=${f.nameplate_total_w}></fleet-gauge>
          <div class="online">${f.online_count} / ${f.inverter_count} inverters online</div>
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
              (inv) => html`<inverter-card .inverter=${inv} .name=${this.names?.[inv.uid] ?? ""}></inverter-card>`,
            )}
          </div>`
        : html`<div class="empty">No inverters discovered yet.</div>`}
      ${nothing}
    `;
  }
}

customElements.define("dashboard-view", DashboardView);

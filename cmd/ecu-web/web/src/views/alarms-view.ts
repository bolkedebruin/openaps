import { LitElement, html, css, nothing, type PropertyValues } from "lit";
import { api, type Event, type Fleet } from "../api.ts";
import { faultLabels } from "../format.ts";
import "../components/events-table.ts";

interface Alarm {
  uid: string;
  model: string;
  label: string;
  severity: "fault" | "warning";
}

const REFRESH_MS = 30_000;
const WINDOW_MS = 24 * 60 * 60 * 1000;
const FETCH_LIMIT = 100;

/**
 * <alarms-view .fleet=${}> derives the live alarm list from the current
 * fleet (active inverter fault bits + offline inverters), and below it
 * shows a "Recent (24h)" log of fault_raised events from /api/events.
 */
export class AlarmsView extends LitElement {
  static properties = {
    fleet: { attribute: false },
    recent: { state: true },
    recentLoading: { state: true },
    recentError: { state: true },
  };
  declare fleet: Fleet | null;
  declare recent: Event[];
  declare recentLoading: boolean;
  declare recentError: string;

  private timer: ReturnType<typeof setInterval> | null = null;

  constructor() {
    super();
    this.fleet = null;
    this.recent = [];
    this.recentLoading = false;
    this.recentError = "";
  }

  static styles = css`
    :host { display: block; }
    .row {
      display: flex;
      align-items: center;
      gap: 12px;
      background: var(--surface);
      border: 1px solid var(--border);
      border-left-width: 3px;
      border-radius: 8px;
      padding: 12px 14px;
      margin-bottom: 8px;
    }
    .row.fault { border-left-color: var(--err); }
    .row.warning { border-left-color: var(--warn); }
    .sev {
      font-size: 11px;
      text-transform: uppercase;
      letter-spacing: 0.05em;
      width: 64px;
    }
    .row.fault .sev { color: var(--err); }
    .row.warning .sev { color: var(--warn); }
    .label { color: var(--text); flex: 1; }
    .uid { font-family: var(--mono); color: var(--muted); font-size: 12px; }
    .ok { color: var(--muted); padding: 32px; text-align: center; }
    .ok .big { color: var(--ok); font-size: 16px; }

    .section { margin-top: 24px; }
    .section h3 {
      font-size: 12px;
      text-transform: uppercase;
      letter-spacing: 0.06em;
      color: var(--muted);
      margin: 0 0 10px 2px;
      font-weight: 600;
    }
    .section .err { color: var(--muted); font-size: 12px; margin: 0 2px 8px; }
    .panel { background: var(--surface); border: 1px solid var(--border); border-radius: 10px; overflow: hidden; }
    .empty { color: var(--muted); padding: 24px; text-align: center; font-size: 13px; }
  `;

  connectedCallback(): void {
    super.connectedCallback();
    void this.loadRecent();
    this.timer = setInterval(() => void this.loadRecent(), REFRESH_MS);
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    if (this.timer) {
      clearInterval(this.timer);
      this.timer = null;
    }
  }

  updated(changed: PropertyValues<this>): void {
    // SSE-driven fleet updates from the parent should land a fresh trip in
    // the Recent log within ~1s without waiting for the 30s tick.
    if (changed.has("fleet")) {
      void this.loadRecent();
    }
  }

  private async loadRecent(): Promise<void> {
    this.recentLoading = true;
    try {
      const res = await api.events({
        kind: "fault_raised",
        since_ms: Date.now() - WINDOW_MS,
        limit: FETCH_LIMIT,
      });
      this.recent = res.events ?? [];
      this.recentError = res.error ?? "";
    } catch (e) {
      this.recentError = (e as Error).message || "failed to load events";
    } finally {
      this.recentLoading = false;
    }
  }

  private alarms(): Alarm[] {
    const out: Alarm[] = [];
    for (const inv of this.fleet?.inverters ?? []) {
      for (const label of faultLabels(inv.faults)) {
        out.push({ uid: inv.uid, model: inv.model, label, severity: "fault" });
      }
      if (!inv.online) {
        out.push({ uid: inv.uid, model: inv.model, label: "Inverter offline", severity: "warning" });
      }
    }
    return out;
  }

  private renderLive() {
    const alarms = this.alarms();
    if (alarms.length === 0) {
      return html`<div class="ok"><div class="big">✓ No active alarms</div><div>All inverters reporting healthy.</div></div>`;
    }
    return html`${alarms.map(
      (a) => html`<div class="row ${a.severity}">
        <span class="sev">${a.severity}</span>
        <span class="label">${a.label} <span style="color:var(--muted)">· ${a.model || "?"}</span></span>
        <span class="uid">${a.uid}</span>
      </div>`,
    )}`;
  }

  private renderRecent() {
    return html`
      <section class="section">
        <h3>Recent (24h)</h3>
        ${this.recentError ? html`<div class="err">⚠ ${this.recentError}</div>` : nothing}
        ${this.recent.length === 0
          ? html`<div class="panel"><div class="empty">No fault events in the last 24h.</div></div>`
          : html`<div class="panel"><events-table .events=${this.recent}></events-table></div>`}
      </section>
    `;
  }

  render() {
    return html`${this.renderLive()}${this.renderRecent()}`;
  }
}

customElements.define("alarms-view", AlarmsView);

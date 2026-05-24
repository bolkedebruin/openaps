import { LitElement, html, css } from "lit";
import type { Fleet } from "../api.ts";
import { faultLabels } from "../format.ts";

interface Alarm {
  uid: string;
  model: string;
  label: string;
  severity: "fault" | "warning";
}

/**
 * <alarms-view .fleet=${}> derives the live alarm list from the current
 * fleet: active inverter fault bits (severity fault) and offline
 * inverters (severity warning).
 */
export class AlarmsView extends LitElement {
  static properties = { fleet: { attribute: false } };
  declare fleet: Fleet | null;

  constructor() {
    super();
    this.fleet = null;
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
  `;

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

  render() {
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
}

customElements.define("alarms-view", AlarmsView);

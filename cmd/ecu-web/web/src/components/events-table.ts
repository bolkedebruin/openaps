import { LitElement, html, css, nothing } from "lit";
import type { Event } from "../api.ts";
import { fmtTime, severityClass, humanizeFault } from "../format.ts";

/** <events-table .events=${[]}> renders the append-only event log. */
export class EventsTable extends LitElement {
  static properties = { events: { attribute: false } };
  declare events: Event[];

  constructor() {
    super();
    this.events = [];
  }

  static styles = css`
    :host { display: block; }
    table { width: 100%; border-collapse: collapse; font-size: 13px; }
    th, td { text-align: left; padding: 9px 12px; border-bottom: 1px solid var(--border); vertical-align: top; }
    th { color: var(--muted); text-transform: uppercase; font-size: 11px; letter-spacing: 0.04em; }
    td { color: var(--text); }
    .time { color: var(--muted); white-space: nowrap; font-variant-numeric: tabular-nums; }
    .uid { font-family: var(--mono); color: var(--muted); font-size: 12px; }
    .detail { color: var(--muted); }
    .sev {
      font-size: 10px;
      text-transform: uppercase;
      letter-spacing: 0.04em;
      border-radius: 999px;
      padding: 1px 8px;
      border: 1px solid var(--border);
    }
    .sev.info { color: var(--muted); }
    .sev.warn { color: var(--warn); border-color: var(--warn); }
    .sev.err { color: var(--err); border-color: var(--err); }
    .empty { color: var(--muted); padding: 32px; text-align: center; }
  `;

  render() {
    if (!this.events || this.events.length === 0) {
      return html`<div class="empty">No events recorded.</div>`;
    }
    return html`
      <table>
        <thead>
          <tr><th>Time</th><th>Severity</th><th>Event</th><th>Inverter</th><th>Detail</th></tr>
        </thead>
        <tbody>
          ${this.events.map(
            (e) => html`<tr>
              <td class="time">${fmtTime(e.ts_ms)}</td>
              <td><span class="sev ${severityClass(e.severity)}">${e.severity}</span></td>
              <td>${humanizeFault(e.kind)}</td>
              <td class="uid">${e.inverter_uid || "—"}</td>
              <td class="detail">${e.detail || (e.raw_hex ? e.raw_hex : nothing)}</td>
            </tr>`,
          )}
        </tbody>
      </table>
    `;
  }
}

customElements.define("events-table", EventsTable);

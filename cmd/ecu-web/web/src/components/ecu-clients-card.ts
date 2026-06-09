import { LitElement, html, css, nothing } from "lit";
import type { SystemStatus } from "../api.ts";

/**
 * <ecu-clients-card .system=${}> shows ECU identity (id/model/firmware)
 * and the live set of UDS peers connected to inv-driver (bus backend,
 * Modbus adapter, this web console…), each with role and a controller
 * badge.
 */
export class EcuClientsCard extends LitElement {
  static properties = { system: { attribute: false } };
  declare system: SystemStatus | null;

  constructor() {
    super();
    this.system = null;
  }

  static styles = css`
    :host { display: block; }
    .id {
      display: grid;
      grid-template-columns: auto 1fr;
      gap: 4px 12px;
      font-size: 13px;
      margin-bottom: 14px;
      padding-bottom: 14px;
      border-bottom: 1px solid var(--border);
    }
    .id .k { color: var(--muted); }
    .id .v { color: var(--text); font-family: var(--mono); }
    .peers { display: flex; flex-direction: column; gap: 8px; }
    .peer { display: flex; align-items: center; gap: 8px; font-size: 13px; }
    .dot { width: 9px; height: 9px; border-radius: 50%; flex: none; }
    .dot.on { background: var(--ok); box-shadow: 0 0 6px var(--ok); }
    .dot.off { background: var(--err); }
    .name { color: var(--text); flex: 1; }
    .role {
      font-size: 10px;
      text-transform: uppercase;
      letter-spacing: 0.04em;
      color: var(--muted);
      border: 1px solid var(--border);
      border-radius: 999px;
      padding: 1px 7px;
    }
    .ctl { color: var(--accent); border-color: var(--accent); }
    .hub { color: var(--ok); border-color: var(--ok); }
    .ver { color: var(--muted); font-size: 11px; font-family: var(--mono); min-width: 0; }
    .warn { color: var(--warn); font-size: 12px; margin-top: 10px; }
    .empty { color: var(--muted); font-size: 13px; }
  `;

  private idRow(k: string, v: string) {
    return v ? html`<div class="k">${k}</div><div class="v">${v}</div>` : nothing;
  }

  // clients collapses the raw per-connection peer list to one row per
  // backend (a backend may hold several connections, e.g. a subscriber
  // plus a short-lived controller), so the list is stable and readable.
  private clients() {
    const by = new Map<string, { backend: string; version: string; controller: boolean; conns: number }>();
    for (const p of this.system?.peers ?? []) {
      const c = by.get(p.backend) ?? { backend: p.backend, version: p.version, controller: false, conns: 0 };
      c.conns++;
      c.controller = c.controller || p.controller;
      if (p.version) c.version = p.version;
      by.set(p.backend, c);
    }
    return [...by.values()].sort((a, b) => a.backend.localeCompare(b.backend));
  }

  render() {
    const sys = this.system;
    const ecu = sys?.ecu;
    const clients = this.clients();
    const hasId = !!(ecu && (ecu.ecu_id || ecu.hostname));
    return html`
      ${hasId
        ? html`<div class="id">
            ${this.idRow("ECU ID", ecu!.ecu_id)}
            ${this.idRow("Host", ecu!.hostname)}
          </div>`
        : nothing}

      <div class="peers">
        <div class="peer">
          <span class="dot ${sys?.invdriver_connected ? "on" : "off"}"></span>
          <span class="name">inv-driver</span>
          <span class="role hub">hub</span>
          ${!sys?.invdriver_connected ? html`<span class="role">offline</span>` : nothing}
        </div>
        ${clients.map(
          (c) => html`<div class="peer">
            <span class="dot on"></span>
            <span class="name">${c.backend || "(unnamed)"}</span>
            ${c.controller ? html`<span class="role ctl">ctrl</span>` : nothing}
            ${c.conns > 1 ? html`<span class="role">${c.conns} conns</span>` : nothing}
            <span class="ver">${c.version || ""}</span>
          </div>`,
        )}
      </div>

      ${sys?.status_error
        ? html`<div class="warn">⚠ ${sys.status_error}</div>`
        : nothing}
    `;
  }
}

customElements.define("ecu-clients-card", EcuClientsCard);

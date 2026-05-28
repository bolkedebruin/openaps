import { LitElement, html, css, nothing } from "lit";
import { api, pairingActive, type Fleet, type PairingStatus } from "../api.ts";
import { fmtW, fmtV, fmtHz, fmtPct } from "../format.ts";
import "../components/cap-input.ts";
import "../components/pairing-scan-panel.ts";
import "../components/pairing-progress-drawer.ts";

/**
 * <inverters-view .fleet=${} .names=${}> — the Inverters screen. It renders
 * the dense per-inverter table (editable label, encryption badge, per-row
 * Replace button) and owns the commissioning controls: a scan/add panel, a
 * fleet re-key action (broadcast, confirm-gated), and a progress drawer that
 * polls GET /api/pairing/status (~1s) while an op is active.
 *
 * The label edit dispatches a bubbling "rename" event ({uid, name}) the shell
 * persists; everything pairing-related is handled inside this view.
 */
export class InvertersView extends LitElement {
  static properties = {
    fleet: { attribute: false },
    names: { attribute: false },
    // pairing UI state
    status: { state: true },
    drawerOpen: { state: true },
    busy: { state: true },
    aborting: { state: true },
    notice: { state: true },
  };
  declare fleet: Fleet | null;
  declare names: Record<string, string>;
  declare status: PairingStatus | null;
  declare drawerOpen: boolean;
  declare busy: boolean;
  declare aborting: boolean;
  declare notice: string;

  private pollTimer: ReturnType<typeof setInterval> | null = null;

  constructor() {
    super();
    this.fleet = null;
    this.names = {};
    this.status = null;
    this.drawerOpen = false;
    this.busy = false;
    this.aborting = false;
    this.notice = "";
  }

  connectedCallback(): void {
    super.connectedCallback();
    // One status fetch on entry so a drawer re-opens if an op is already
    // running (e.g. started in another tab).
    void this.fetchStatus();
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    this.stopPoll();
  }

  private rename(uid: string, e: Event) {
    const name = (e.target as HTMLInputElement).value;
    this.dispatchEvent(new CustomEvent("rename", { detail: { uid, name }, bubbles: true, composed: true }));
  }

  // encBadge renders the per-inverter encryption indicator. encrypted===true
  // → AES lock; ===false → plaintext warning; undefined → neutral "unknown"
  // (the field isn't yet surfaced on the snapshot for all units).
  private encBadge(enc: boolean | undefined) {
    if (enc === true) {
      return html`<span class="enc enc-ok" title="AES-encrypted link">🔒 AES</span>`;
    }
    if (enc === false) {
      return html`<span class="enc enc-warn" title="Plaintext link — misconfigured or foreign unit">⚠ plaintext</span>`;
    }
    return html`<span class="enc enc-unknown" title="Encryption state unknown">—</span>`;
  }

  // --- pairing orchestration ---

  private async fetchStatus(): Promise<void> {
    try {
      const r = await api.pairingStatus();
      this.status = r.status ?? null;
      if (pairingActive(this.status)) {
        this.drawerOpen = true;
        this.startPoll();
      } else {
        this.stopPoll();
      }
    } catch {
      /* leave prior status */
    }
  }

  private startPoll() {
    if (this.pollTimer) return;
    this.pollTimer = setInterval(() => void this.fetchStatus(), 1000);
  }

  private stopPoll() {
    if (this.pollTimer) clearInterval(this.pollTimer);
    this.pollTimer = null;
  }

  // applyResp adopts the status from a start/abort response and opens the
  // drawer + poll loop when an op is now active.
  private applyResp(status: PairingStatus | null | undefined) {
    this.status = status ?? null;
    this.drawerOpen = true;
    if (pairingActive(this.status)) this.startPoll();
  }

  private onScan = async (e: Event) => {
    const { slow } = (e as CustomEvent<{ slow: boolean }>).detail;
    if (slow && !confirm(
      "Slow scan sweeps ZigBee channels 11–26 on PAN 0xFFFF and pauses fleet " +
      "telemetry for ~30 seconds. Continue?")) return;
    this.busy = true;
    this.notice = "";
    try {
      const r = await api.pairingScan({ slow });
      if (!r.ok) throw new Error(r.error || "scan rejected");
      this.applyResp(r.status);
    } catch (err) {
      this.notice = String((err as Error).message || err);
    } finally {
      this.busy = false;
    }
  };

  private onAdd = async (e: Event) => {
    const { serial } = (e as CustomEvent<{ serial: string }>).detail;
    this.busy = true;
    this.notice = "";
    try {
      const r = await api.pairingAdd(serial);
      if (!r.ok) throw new Error(r.error || "add rejected");
      this.applyResp(r.status);
    } catch (err) {
      this.notice = String((err as Error).message || err);
    } finally {
      this.busy = false;
    }
  };

  private onReplace = async (uid: string) => {
    const newSerial = prompt(
      `Replace inverter ${uid}.\n\nEnter the replacement's 12-digit serial, ` +
      `or leave blank to scan for it. The new unit inherits this one's grid ` +
      `profile, power cap and array slot.`,
    );
    if (newSerial === null) return; // cancelled
    const serial = newSerial.replace(/\D/g, "");
    if (serial !== "" && serial.length !== 12) {
      this.notice = "Replacement serial must be 12 digits (or blank to scan).";
      return;
    }
    this.busy = true;
    this.notice = "";
    try {
      const r = await api.pairingReplace(uid, serial);
      if (!r.ok) throw new Error(r.error || "replace rejected");
      this.applyResp(r.status);
    } catch (err) {
      this.notice = String((err as Error).message || err);
    } finally {
      this.busy = false;
    }
  };

  private onRekey = async () => {
    const pan = prompt(
      "Fleet re-key BROADCASTS a new PAN to every inverter (0x22) and moves " +
      "the radio to it. Telemetry pauses while it runs; on failure the old " +
      "PAN is restored.\n\nEnter the new PAN (1–4 hex digits, e.g. 0DCE):",
    );
    if (pan === null) return;
    const newPan = pan.trim();
    if (!/^[0-9a-fA-F]{1,4}$/.test(newPan)) {
      this.notice = "PAN must be 1–4 hexadecimal digits.";
      return;
    }
    this.busy = true;
    this.notice = "";
    try {
      const r = await api.pairingRekey(newPan, 0);
      if (!r.ok) throw new Error(r.error || "re-key rejected");
      this.applyResp(r.status);
    } catch (err) {
      const msg = String((err as Error).message || err);
      this.notice = /step-up/i.test(msg)
        ? "Re-key needs a password confirm. Confirm your password (Settings) then retry."
        : msg;
    } finally {
      this.busy = false;
    }
  };

  private onAbort = async () => {
    this.aborting = true;
    try {
      const r = await api.pairingAbort();
      this.status = r.status ?? this.status;
    } catch (err) {
      this.notice = String((err as Error).message || err);
    } finally {
      this.aborting = false;
      void this.fetchStatus();
    }
  };

  private onCloseDrawer = () => {
    if (pairingActive(this.status)) return; // can't dismiss a running op
    this.drawerOpen = false;
  };

  static styles = css`
    :host { display: block; }
    .controls {
      display: flex; align-items: flex-start; justify-content: space-between;
      gap: 16px; flex-wrap: wrap; margin-bottom: 20px;
    }
    .rekey {
      display: flex; flex-direction: column; gap: 6px; align-items: flex-end;
    }
    button.rekey-btn {
      background: transparent; border: 1px solid var(--err); color: var(--err);
      border-radius: 8px; padding: 8px 16px; font-size: 13px; font-weight: 600; cursor: pointer;
      white-space: nowrap;
    }
    button.rekey-btn:hover:not(:disabled) { background: color-mix(in srgb, var(--err) 12%, transparent); }
    button.rekey-btn:disabled { opacity: 0.45; cursor: not-allowed; }
    .rekey .hint { font-size: 11px; color: var(--muted); max-width: 220px; text-align: right; }
    .notice {
      color: var(--err); font-size: 13px; margin-bottom: 16px;
      border: 1px solid color-mix(in srgb, var(--err) 40%, transparent);
      border-radius: 8px; padding: 8px 10px;
    }
    .table-wrap { overflow-x: auto; }
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
    .capcell { white-space: nowrap; }
    .fw { font-variant-numeric: tabular-nums; color: var(--muted); }
    .fault { color: var(--err); }
    .empty { color: var(--muted); padding: 32px; text-align: center; }
    .enc { font-size: 11px; white-space: nowrap; }
    .enc-ok { color: var(--ok); }
    .enc-warn { color: var(--err); }
    .enc-unknown { color: var(--muted); }
    button.replace {
      background: transparent;
      border: 1px solid var(--border);
      color: var(--muted);
      border-radius: 6px;
      padding: 4px 10px;
      font-size: 12px;
      cursor: pointer;
      white-space: nowrap;
    }
    button.replace:hover { color: var(--text); border-color: var(--muted); }
  `;

  private renderTable() {
    const f = this.fleet;
    if (!f || f.inverters.length === 0) {
      return html`<div class="empty">No inverters discovered yet.</div>`;
    }
    return html`
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Inverter ID</th><th>Name</th><th>Model</th><th>Firmware</th><th>Status</th>
              <th>Encryption</th>
              <th class="num">Output</th><th class="num">Load</th><th>Output cap</th>
              <th class="num">Grid</th><th class="num">Freq</th>
              <th class="num">Panels</th><th class="num">Faults</th><th></th>
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
                <td>${this.encBadge(inv.encrypted)}</td>
                <td class="num">${fmtW(inv.active_power_w)} / ${fmtW(inv.nameplate_w)}</td>
                <td class="num">${fmtPct(inv.load_pct)}</td>
                <td class="capcell"><cap-input .inverter=${inv}></cap-input></td>
                <td class="num">${fmtV(inv.grid_v)}</td>
                <td class="num">${fmtHz(inv.freq_hz)}</td>
                <td class="num">${inv.panels?.length ?? 0}</td>
                <td class="num ${nFaults ? "fault" : ""}">${nFaults || "—"}</td>
                <td>
                  <button class="replace" title="Replace this inverter with a new unit"
                    ?disabled=${this.busy}
                    @click=${() => this.onReplace(inv.uid)}>Replace</button>
                </td>
              </tr>`;
            })}
          </tbody>
        </table>
      </div>
    `;
  }

  render() {
    return html`
      <div class="controls">
        <pairing-scan-panel
          .busy=${this.busy}
          @scan=${this.onScan}
          @add=${this.onAdd}
        ></pairing-scan-panel>
        <div class="rekey">
          <button class="rekey-btn" ?disabled=${this.busy} @click=${this.onRekey}>Fleet re-key…</button>
          <span class="hint">Broadcasts a new PAN to the whole fleet. Confirmation required.</span>
        </div>
      </div>

      ${this.notice ? html`<div class="notice" role="alert">${this.notice}</div>` : nothing}

      ${this.renderTable()}

      <pairing-progress-drawer
        .open=${this.drawerOpen}
        .status=${this.status}
        .aborting=${this.aborting}
        @abort=${this.onAbort}
        @close=${this.onCloseDrawer}
      ></pairing-progress-drawer>
    `;
  }
}

customElements.define("inverters-view", InvertersView);

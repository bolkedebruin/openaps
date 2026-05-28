import { LitElement, html, css, nothing } from "lit";

/**
 * <pairing-scan-panel> — the commissioning controls for adding inverters.
 * It is presentational: it emits bubbling/composed events the view owns —
 *   "scan"   detail {slow:boolean}      → start a discovery scan
 *   "add"    detail {serial:string}     → add a known 12-digit serial
 * The Fast/Slow toggle warns that Slow (a channel sweep) pauses telemetry
 * for roughly the dwell × channel count while the radio is off the
 * operating PAN.
 */
export class PairingScanPanel extends LitElement {
  static properties = {
    busy: { attribute: false },
    slow: { state: true },
    serial: { state: true },
  };

  declare busy: boolean;
  declare slow: boolean;
  declare serial: string;

  constructor() {
    super();
    this.busy = false;
    this.slow = false;
    this.serial = "";
  }

  static styles = css`
    :host { display: block; }
    .panel { display: grid; gap: 16px; max-width: 520px; }
    fieldset { border: 1px solid var(--border); border-radius: 10px; padding: 14px 16px; margin: 0; }
    legend { color: var(--muted); font-size: 12px; padding: 0 6px; text-transform: uppercase; letter-spacing: 0.04em; }
    .row { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
    .toggle { display: inline-flex; border: 1px solid var(--border); border-radius: 8px; overflow: hidden; }
    .toggle button {
      background: transparent; border: none; color: var(--muted);
      padding: 7px 16px; font: inherit; font-size: 13px; cursor: pointer;
    }
    .toggle button.sel { background: var(--accent); color: #04121a; font-weight: 600; }
    .warn {
      color: var(--err); font-size: 12px; background: color-mix(in srgb, var(--err) 12%, transparent);
      border: 1px solid color-mix(in srgb, var(--err) 40%, transparent);
      border-radius: 8px; padding: 8px 10px;
    }
    input.serial {
      background: var(--bar-bg); border: 1px solid var(--border); color: var(--text);
      border-radius: 8px; padding: 8px 11px; font: inherit; font-size: 14px;
      font-family: var(--mono); width: 170px; letter-spacing: 0.02em;
    }
    input.serial:focus { outline: none; border-color: var(--accent); }
    button.go {
      background: var(--accent); border: none; color: #04121a; border-radius: 8px;
      padding: 8px 18px; font-size: 14px; font-weight: 600; cursor: pointer; white-space: nowrap;
    }
    button.go:hover:not(:disabled) { filter: brightness(1.08); }
    button.go:disabled { opacity: 0.45; cursor: not-allowed; }
    .hint { font-size: 12px; color: var(--muted); }
  `;

  private startScan = () => {
    if (this.busy) return;
    this.dispatchEvent(
      new CustomEvent("scan", { detail: { slow: this.slow }, bubbles: true, composed: true }),
    );
  };

  private onSerialInput = (e: Event) => {
    // Keep digits only — serials are 12-digit decimal.
    this.serial = (e.target as HTMLInputElement).value.replace(/\D/g, "").slice(0, 12);
  };

  private addById = () => {
    if (this.busy || this.serial.length !== 12) return;
    this.dispatchEvent(
      new CustomEvent("add", { detail: { serial: this.serial }, bubbles: true, composed: true }),
    );
    this.serial = "";
  };

  render() {
    const serialValid = this.serial.length === 12;
    return html`
      <div class="panel">
        <fieldset>
          <legend>Scan for inverters</legend>
          <div class="row">
            <div class="toggle" role="group" aria-label="Scan speed">
              <button
                class=${!this.slow ? "sel" : ""}
                aria-pressed=${!this.slow}
                ?disabled=${this.busy}
                @click=${() => (this.slow = false)}
              >Fast</button>
              <button
                class=${this.slow ? "sel" : ""}
                aria-pressed=${this.slow}
                ?disabled=${this.busy}
                @click=${() => (this.slow = true)}
              >Slow</button>
            </div>
            <button class="go" ?disabled=${this.busy} @click=${this.startScan}>
              ${this.busy ? "Scanning…" : "Scan"}
            </button>
          </div>
          ${this.slow
            ? html`<p class="warn" role="alert">
                Slow scan sweeps the radio across ZigBee channels 11–26 on PAN 0xFFFF.
                This pauses telemetry for ~30 seconds while the module is off the
                operating PAN. Use for commissioning only.
              </p>`
            : html`<p class="hint">
                Fast scan solicits new inverters on PAN 0xFFFF on the current channel.
                Telemetry briefly pauses while the radio is parked for discovery.
              </p>`}
        </fieldset>

        <fieldset>
          <legend>Add by serial</legend>
          <div class="row">
            <input
              class="serial"
              inputmode="numeric"
              placeholder="12-digit serial"
              .value=${this.serial}
              ?disabled=${this.busy}
              @input=${this.onSerialInput}
              @keydown=${(e: KeyboardEvent) => { if (e.key === "Enter") this.addById(); }}
            />
            <button class="go" ?disabled=${this.busy || !serialValid} @click=${this.addById}>Add</button>
          </div>
          ${this.serial.length > 0 && !serialValid
            ? html`<p class="hint">Serial must be exactly 12 digits (${this.serial.length}/12).</p>`
            : nothing}
        </fieldset>
      </div>
    `;
  }
}

customElements.define("pairing-scan-panel", PairingScanPanel);

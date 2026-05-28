import { LitElement, html, css, nothing } from "lit";
import { api, type Inverter } from "../api.ts";
import { capFloorW, capCeilW, readbackCapW } from "../power.ts";

/**
 * <cap-input .inverter=${inv}> shows the output cap as an editable number —
 * "<cap> / <nameplate> W" — and commits a new cap via api.setPower on change.
 * Used in the dense inverter list (no slider). nameplate = uncapped.
 */
export class CapInput extends LitElement {
  static properties = {
    inverter: { attribute: false },
    pendingCap: { state: true },
    busy: { state: true },
    error: { state: true },
  };

  declare inverter: Inverter;
  declare pendingCap: number | null; // optimistic cap until read-back confirms
  declare busy: boolean;
  declare error: string;

  constructor() {
    super();
    this.pendingCap = null;
    this.busy = false;
    this.error = "";
  }

  static styles = css`
    :host { display: inline-block; }
    .row { display: flex; align-items: center; gap: 6px; white-space: nowrap; }
    input {
      width: 76px;
      box-sizing: border-box;
      padding: 5px 7px;
      background: var(--bar-bg);
      border: 1px solid var(--border);
      border-radius: 6px;
      color: var(--text);
      font: inherit;
      text-align: right;
      font-variant-numeric: tabular-nums;
    }
    input:focus { outline: none; border-color: var(--accent); }
    input:disabled { opacity: 0.6; }
    .max { color: var(--muted); font-variant-numeric: tabular-nums; }
    .err { color: var(--err); }
  `;

  private commit = async (e: Event) => {
    const watts = Math.round(Number((e.target as HTMLInputElement).value));
    if (!Number.isFinite(watts) || watts <= 0) return;
    this.pendingCap = watts;
    this.busy = true;
    this.error = "";
    try {
      const r = await api.setPower({ uid: this.inverter.uid, watts });
      const res = r.results?.[0];
      if (res && !res.ok) this.error = res.error || "failed";
      else if (res) this.pendingCap = res.applied_watts;
    } catch (err) {
      this.error = (err as Error).message || "failed";
    } finally {
      this.busy = false;
    }
  };

  render() {
    const inv = this.inverter;
    if (!inv) return nothing;
    const ceil = capCeilW(inv);
    if (ceil <= 0) return html`<span class="max">—</span>`;
    const cap = this.pendingCap ?? readbackCapW(inv) ?? ceil;
    return html`
      <div class="row">
        <input
          type="number"
          min=${capFloorW(inv)}
          max=${ceil}
          step="10"
          .value=${String(Math.round(cap))}
          ?disabled=${!inv.online || this.busy}
          @change=${this.commit}
          title="output cap, watts"
        />
        <span class="max">/ ${Math.round(ceil)} W</span>
        ${this.error ? html`<span class="err" title=${this.error}>⚠</span>` : nothing}
      </div>
    `;
  }
}

customElements.define("cap-input", CapInput);

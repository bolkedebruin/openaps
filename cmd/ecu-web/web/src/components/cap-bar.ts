import { LitElement, html, css, nothing } from "lit";
import { api, type Inverter } from "../api.ts";
import { fmtW, loadClass } from "../format.ts";
import { capFloorW, capCeilW, readbackCapW } from "../power.ts";

/**
 * <cap-bar .inverter=${inv}> renders an inverter's live-output bar with the
 * output cap marked by a thin red line + a draggable red triangle caret, plus
 * the cap value. Dragging commits a new cap via api.setPower (optimistic until
 * the read-back confirms). Shared by the dashboard cards and the inverter list
 * so the cap is visible and settable in both. The far-right edge is nameplate
 * (uncapped).
 */
export class CapBar extends LitElement {
  static properties = {
    inverter: { attribute: false },
    pendingCap: { state: true },
    busy: { state: true },
    error: { state: true },
  };

  declare inverter: Inverter;
  declare pendingCap: number | null; // optimistic/in-drag cap until read-back
  declare busy: boolean;
  declare error: string;

  private dragging = false;

  constructor() {
    super();
    this.pendingCap = null;
    this.busy = false;
    this.error = "";
  }

  static styles = css`
    :host { display: block; }
    .row { display: flex; align-items: center; gap: 10px; }
    .barwrap { position: relative; height: 20px; flex: 1; min-width: 90px; touch-action: none; cursor: pointer; }
    .barwrap.off { cursor: default; opacity: 0.6; }
    .bar { height: 8px; background: var(--bar-bg); border-radius: 4px; position: relative; overflow: hidden; }
    .fill { height: 100%; border-radius: 4px; transition: width 0.4s ease; }
    .fill.low { background: var(--ok); }
    .fill.mid { background: var(--accent); }
    .fill.high { background: var(--warn); }
    .fill.idle { background: var(--muted); }
    .capline { position: absolute; top: -1px; height: 10px; width: 2px; background: var(--err); transform: translateX(-1px); pointer-events: none; }
    .caret {
      position: absolute; top: 11px;
      width: 0; height: 0;
      border-left: 5px solid transparent;
      border-right: 5px solid transparent;
      border-bottom: 7px solid var(--err);
      transform: translateX(-5px);
      pointer-events: none;
    }
    .capval { color: var(--err); font-size: 13px; font-weight: 600; white-space: nowrap; font-variant-numeric: tabular-nums; }
    .caperr { color: var(--err); font-size: 12px; margin-top: 4px; }
  `;

  // capFromEvent maps a pointer x-position on the bar to a cap in watts,
  // clamped to [floor, nameplate]. The far-right edge is nameplate (uncapped).
  private capFromEvent(e: PointerEvent): number {
    const bar = this.renderRoot.querySelector(".bar") as HTMLElement | null;
    const ceil = capCeilW(this.inverter);
    if (!bar) return this.pendingCap ?? ceil;
    const r = bar.getBoundingClientRect();
    const frac = Math.max(0, Math.min(1, (e.clientX - r.left) / r.width));
    return Math.min(ceil, Math.max(capFloorW(this.inverter), Math.round(frac * ceil)));
  }

  private onDown = (e: PointerEvent) => {
    if (!this.inverter?.online || this.busy) return;
    e.preventDefault();
    this.dragging = true;
    try {
      (e.currentTarget as HTMLElement).setPointerCapture?.(e.pointerId);
    } catch {
      /* synthetic event / no active pointer */
    }
    this.pendingCap = this.capFromEvent(e);
  };
  private onMove = (e: PointerEvent) => {
    if (this.dragging) this.pendingCap = this.capFromEvent(e);
  };
  private onUp = (e: PointerEvent) => {
    if (!this.dragging) return;
    this.dragging = false;
    try {
      (e.currentTarget as HTMLElement).releasePointerCapture?.(e.pointerId);
    } catch {
      /* ignore */
    }
    void this.commitCap();
  };

  private async commitCap() {
    const watts = this.pendingCap;
    if (watts == null) return;
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
  }

  render() {
    const inv = this.inverter;
    if (!inv) return nothing;
    const lc = loadClass(inv.load_pct);
    const liveWidth = Math.max(0, Math.min(100, inv.load_pct));
    const ceil = capCeilW(inv);
    if (ceil <= 0) {
      // No nameplate → show the live bar only (not settable).
      return html`<div class="bar"><div class="fill ${lc}" style="width:${liveWidth}%"></div></div>`;
    }
    const cap = this.pendingCap ?? readbackCapW(inv) ?? ceil;
    const capPct = Math.max(0, Math.min(100, (cap / ceil) * 100));
    return html`
      <div class="row">
        <div
          class="barwrap ${inv.online ? "" : "off"}"
          @pointerdown=${this.onDown}
          @pointermove=${this.onMove}
          @pointerup=${this.onUp}
          title="drag to set the output cap"
        >
          <div class="bar"><div class="fill ${lc}" style="width:${liveWidth}%"></div></div>
          <div class="capline" style="left:${capPct}%"></div>
          <div class="caret" style="left:${capPct}%"></div>
        </div>
        <span class="capval" title="output cap">▼ ${fmtW(cap)}</span>
      </div>
      ${this.error ? html`<div class="caperr">⚠ ${this.error}</div>` : nothing}
    `;
  }
}

customElements.define("cap-bar", CapBar);

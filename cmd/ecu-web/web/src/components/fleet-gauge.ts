import { LitElement, html, css } from "lit";
import { fmtW, fmtPct, loadClass } from "../format.ts";

/**
 * <fleet-gauge .power=${w} .cap=${w}> renders the array's live output as
 * a semicircular gauge against nameplate capacity.
 */
export class FleetGauge extends LitElement {
  static properties = {
    power: { type: Number },
    cap: { type: Number },
  };

  declare power: number;
  declare cap: number;

  constructor() {
    super();
    this.power = 0;
    this.cap = 0;
  }

  static styles = css`
    :host { display: block; text-align: center; }
    .wrap { position: relative; width: 220px; margin: 0 auto; }
    svg { width: 100%; height: auto; display: block; }
    .track { stroke: var(--bar-bg); }
    .arc { stroke-linecap: round; transition: stroke-dashoffset 0.5s ease, stroke 0.3s; }
    .arc.low { stroke: var(--ok); }
    .arc.mid { stroke: var(--accent); }
    .arc.high { stroke: var(--warn); }
    .arc.idle { stroke: var(--muted); }
    .center {
      position: absolute;
      left: 0;
      right: 0;
      bottom: 10%;
    }
    .big { font-size: 30px; font-weight: 700; color: var(--text); }
    .sub { font-size: 13px; color: var(--muted); margin-top: 2px; }
  `;

  // 0..100, clamped.
  private pct(): number {
    if (!(this.cap > 0)) return 0;
    return Math.max(0, Math.min(100, (this.power / this.cap) * 100));
  }

  render() {
    const pct = this.pct();
    const lc = loadClass(pct);
    // Semicircle: radius 90, half-circumference = π*r ≈ 282.74.
    const r = 90;
    const half = Math.PI * r;
    const offset = half * (1 - pct / 100);
    return html`
      <div class="wrap">
        <svg viewBox="0 0 200 120" role="img" aria-label="fleet output gauge">
          <path
            class="track"
            d="M10 110 A 90 90 0 0 1 190 110"
            fill="none"
            stroke-width="14"
          />
          <path
            class="arc ${lc}"
            d="M10 110 A 90 90 0 0 1 190 110"
            fill="none"
            stroke-width="14"
            stroke-dasharray="${half}"
            stroke-dashoffset="${offset}"
          />
        </svg>
        <div class="center">
          <div class="big">${fmtW(this.power)}</div>
          <div class="sub">${fmtPct(pct)} of ${fmtW(this.cap)}</div>
        </div>
      </div>
    `;
  }
}

customElements.define("fleet-gauge", FleetGauge);

import { LitElement, html, css, svg, nothing } from "lit";
import { fmtNum } from "../format.ts";

/**
 * <freq-watt-chart> plots the over-frequency power-reduction (droop) curve:
 * 100% power up to the start frequency, then falling by `slope` %Pref/Hz.
 * The over-frequency trip is marked in red — power should reach the floor by
 * curtailment before the trip fires. Dependency-free SVG; updates live.
 */
export class FreqWattChart extends LitElement {
  static properties = {
    deadband: { type: Number }, // start frequency (Hz)
    slope: { type: Number }, // %Pref per Hz
    trip: { type: Number }, // over-frequency trip (Hz), optional
    nominal: { type: Number },
  };
  declare deadband?: number;
  declare slope?: number;
  declare trip?: number;
  declare nominal: number;

  constructor() {
    super();
    this.nominal = 50;
  }

  static styles = css`
    :host { display: block; }
    svg { width: 100%; height: auto; }
    .frame { stroke: var(--border); fill: none; }
    .grid { stroke: color-mix(in srgb, var(--border) 60%, transparent); }
    .curve { stroke: var(--accent); stroke-width: 2; fill: none; }
    .dead { stroke: var(--muted); stroke-dasharray: 3 3; }
    .trip { stroke: var(--err); stroke-width: 1.5; }
    text { fill: var(--muted); font-size: 9px; }
    .lbl { fill: var(--text); }
    .empty { color: var(--muted); font-size: 12px; padding: 8px 0; }
  `;

  render() {
    const db = this.deadband, sl = this.slope, trip = this.trip, nom = this.nominal;
    if (db === undefined || sl === undefined || sl <= 0) {
      return html`<div class="empty">Set the curtailment start frequency and slope to preview the curve.</div>`;
    }
    const zeroF = db + 100 / sl; // frequency where power reaches 0%
    const xmin = nom - 0.3;
    const xmax = Math.max(trip ?? 0, zeroF, db + 1.5, nom + 1.5) + 0.2;
    const W = 480, H = 170, pl = 36, pr = 12, pt = 10, pb = 24;
    const X = (f: number) => pl + ((f - xmin) / (xmax - xmin)) * (W - pl - pr);
    const Y = (p: number) => pt + ((100 - p) / 100) * (H - pt - pb);

    const endF = Math.min(zeroF, xmax);
    const endP = Math.max(0, 100 - sl * (endF - db));
    const poly = [
      [xmin, 100],
      [db, 100],
      [endF, endP],
      ...(zeroF < xmax ? [[xmax, 0] as [number, number]] : []),
    ].map(([f, p]) => `${X(f).toFixed(1)},${Y(p).toFixed(1)}`).join(" ");

    const xticks = [];
    for (let f = Math.ceil(xmin * 2) / 2; f <= xmax; f += 0.5) xticks.push(f);

    return html`
      <svg viewBox="0 0 ${W} ${H}" role="img" aria-label="Frequency-Watt curtailment curve">
        ${[0, 50, 100].map(
          (p) => svg`<line class="grid" x1=${pl} y1=${Y(p)} x2=${W - pr} y2=${Y(p)} />
            <text x=${pl - 4} y=${Y(p) + 3} text-anchor="end">${p}%</text>`,
        )}
        ${xticks.map(
          (f) => svg`<text x=${X(f)} y=${H - pb + 12} text-anchor="middle">${f.toFixed(1)}</text>`,
        )}
        <line class="frame" x1=${pl} y1=${pt} x2=${pl} y2=${H - pb} />
        <line class="frame" x1=${pl} y1=${H - pb} x2=${W - pr} y2=${H - pb} />
        <line class="dead" x1=${X(db)} y1=${pt} x2=${X(db)} y2=${H - pb} />
        <text class="lbl" x=${X(db)} y=${pt + 8} text-anchor="middle">start ${fmtNum(db)} Hz</text>
        ${zeroF <= xmax
          ? svg`<line class="dead" x1=${X(zeroF)} y1=${pt} x2=${X(zeroF)} y2=${H - pb} />
              <text class="lbl" x=${X(zeroF)} y=${pt + 8} text-anchor="middle">0% at ${fmtNum(zeroF)} Hz</text>`
          : nothing}
        ${trip !== undefined && trip >= xmin && trip <= xmax
          ? svg`<line class="trip" x1=${X(trip)} y1=${pt} x2=${X(trip)} y2=${H - pb} />
              <text x=${X(trip)} y=${H - pb - 4} text-anchor="middle" fill="var(--err)">trip ${fmtNum(trip)} Hz</text>`
          : nothing}
        <polyline class="curve" points=${poly} />
        <text x=${(W) / 2} y=${H - 2} text-anchor="middle">Power vs frequency · slope ${fmtNum(sl)} %Pref/Hz</text>
      </svg>
    `;
  }
}

customElements.define("freq-watt-chart", FreqWattChart);

import { LitElement, html, css, svg, nothing } from "lit";

export interface TripMarker {
  value: number;
  label: string;
  kind: "under" | "over" | "curve";
}

/**
 * <trip-line> draws a horizontal value axis (volts or Hz) with trip thresholds
 * and curve breakpoints marked, and a green "operating band" between the
 * highest under-limit and the lowest over-limit. Dependency-free SVG.
 */
export class TripLine extends LitElement {
  static properties = {
    unit: { type: String },
    nominal: { type: Number },
    markers: { attribute: false },
  };
  declare unit: string;
  declare nominal?: number;
  declare markers: TripMarker[];

  constructor() {
    super();
    this.unit = "";
    this.markers = [];
  }

  static styles = css`
    :host { display: block; }
    svg { width: 100%; height: auto; }
    .axis { stroke: var(--border); }
    .band { fill: color-mix(in srgb, var(--ok) 16%, transparent); }
    .under { stroke: var(--accent); }
    .over { stroke: var(--err); }
    .curve { stroke: var(--muted); stroke-dasharray: 2 2; }
    .nom { stroke: var(--ok); }
    text { font-size: 9px; fill: var(--muted); }
    .empty { color: var(--muted); font-size: 12px; padding: 6px 0; }
  `;

  render() {
    const ms = (this.markers ?? []).filter((m) => Number.isFinite(m.value));
    if (!ms.length) return html`<div class="empty">No thresholds set.</div>`;
    const vals = ms.map((m) => m.value).concat(this.nominal !== undefined ? [this.nominal] : []);
    let lo = Math.min(...vals), hi = Math.max(...vals);
    const pad = (hi - lo) * 0.14 || 1;
    lo -= pad; hi += pad;
    const W = 480, H = 70, pl = 10, pr = 10, axisY = 34;
    const X = (v: number) => pl + ((v - lo) / (hi - lo)) * (W - pl - pr);

    const unders = ms.filter((m) => m.kind === "under").map((m) => m.value);
    const overs = ms.filter((m) => m.kind === "over").map((m) => m.value);
    const bandLo = unders.length ? Math.max(...unders) : lo;
    const bandHi = overs.length ? Math.min(...overs) : hi;

    return html`
      <svg viewBox="0 0 ${W} ${H}" role="img" aria-label="Trip thresholds">
        ${bandHi > bandLo
          ? svg`<rect class="band" x=${X(bandLo)} y=${axisY - 8} width=${X(bandHi) - X(bandLo)} height=16 />`
          : nothing}
        <line class="axis" x1=${pl} y1=${axisY} x2=${W - pr} y2=${axisY} />
        ${this.nominal !== undefined
          ? svg`<line class="nom" x1=${X(this.nominal)} y1=${axisY - 9} x2=${X(this.nominal)} y2=${axisY + 9} />
              <text x=${X(this.nominal)} y=${axisY + 20} text-anchor="middle" fill="var(--ok)">${this.nominal} ${this.unit}</text>`
          : nothing}
        ${ms.map((m, i) => {
          const cls = m.kind;
          const up = i % 2 === 0;
          const ty = up ? axisY - 12 : axisY + 22;
          return svg`<line class=${cls} x1=${X(m.value)} y1=${axisY - 7} x2=${X(m.value)} y2=${axisY + 7} />
            <text x=${X(m.value)} y=${ty} text-anchor="middle">${m.label} ${m.value}</text>`;
        })}
      </svg>
    `;
  }
}

customElements.define("trip-line", TripLine);

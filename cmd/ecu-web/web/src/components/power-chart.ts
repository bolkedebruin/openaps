import { LitElement, html, css, svg, nothing } from "lit";
import { fmtW, fmtTime } from "../format.ts";

export interface PowerPoint {
  t: number; // unix ms
  w: number;
}

export interface ChartPaths {
  line: string;
  area: string;
  max: number;
}

/**
 * chartPaths maps a time series to SVG path strings in a width×height
 * box. x spans first→last timestamp; y is scaled to the series peak
 * (min 1 W). Returns empty paths for fewer than two points.
 */
export function chartPaths(points: PowerPoint[], width: number, height: number): ChartPaths {
  if (points.length < 2) return { line: "", area: "", max: 0 };
  const t0 = points[0].t;
  const span = Math.max(1, points[points.length - 1].t - t0);
  const max = Math.max(1, ...points.map((p) => p.w));
  const xy = (p: PowerPoint): [number, number] => [
    ((p.t - t0) / span) * width,
    height - (p.w / max) * height,
  ];
  let line = "";
  for (let i = 0; i < points.length; i++) {
    const [x, y] = xy(points[i]);
    line += `${i === 0 ? "M" : "L"}${x.toFixed(1)} ${y.toFixed(1)} `;
  }
  const [x0] = xy(points[0]);
  const [xn] = xy(points[points.length - 1]);
  const area = `${line}L${xn.toFixed(1)} ${height} L${x0.toFixed(1)} ${height} Z`;
  return { line: line.trim(), area, max };
}

const W = 600;
const H = 160;

/** <power-chart .points=${[{t,w}]}> draws fleet output over time. */
export class PowerChart extends LitElement {
  static properties = {
    points: { attribute: false },
    hoverIdx: { state: true },
  };
  declare points: PowerPoint[];
  declare hoverIdx: number; // -1 = no hover

  constructor() {
    super();
    this.points = [];
    this.hoverIdx = -1;
  }

  static styles = css`
    :host { display: block; }
    .empty { color: var(--muted); text-align: center; padding: 48px 0; font-size: 13px; }
    .wrap { position: relative; }
    svg { width: 100%; height: 160px; display: block; }
    .area { fill: url(#pc-grad); }
    .line { fill: none; stroke: var(--accent); stroke-width: 2; vector-effect: non-scaling-stroke; }
    .cross { stroke: var(--muted); stroke-width: 1; vector-effect: non-scaling-stroke; opacity: 0.6; }
    .cursor { fill: var(--accent); stroke: var(--bg); stroke-width: 1.5; vector-effect: non-scaling-stroke; }
    .tip {
      position: absolute;
      transform: translate(-50%, -118%);
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: 6px;
      padding: 4px 8px;
      font-size: 12px;
      color: var(--text);
      white-space: nowrap;
      pointer-events: none;
      box-shadow: 0 2px 8px rgba(0, 0, 0, 0.4);
    }
    .tip .t { color: var(--muted); }
    .tip .w { font-weight: 600; }
    .labels { display: flex; justify-content: space-between; font-size: 12px; color: var(--muted); margin-top: 6px; }
    .cur { color: var(--text); font-weight: 600; }
  `;

  private onMove = (e: MouseEvent) => {
    const n = this.points.length;
    if (n < 2) return;
    const el = e.currentTarget as SVGElement;
    const w = el.clientWidth || 1;
    const frac = Math.min(1, Math.max(0, e.offsetX / w));
    this.hoverIdx = Math.round(frac * (n - 1));
  };

  private onLeave = () => {
    this.hoverIdx = -1;
  };

  render() {
    const pts = this.points ?? [];
    if (pts.length < 2) {
      return html`<div class="empty">Collecting power history…</div>`;
    }
    const { line, area, max } = chartPaths(pts, W, H);
    const cur = pts[pts.length - 1].w;

    // Hovered point geometry (same mapping as chartPaths).
    const i = this.hoverIdx;
    const hovering = i >= 0 && i < pts.length;
    const t0 = pts[0].t;
    const span = Math.max(1, pts[pts.length - 1].t - t0);
    const hx = hovering ? ((pts[i].t - t0) / span) * W : 0;
    const hy = hovering ? H - (pts[i].w / max) * H : 0;

    return html`
      <div class="wrap">
        <svg
          viewBox="0 0 ${W} ${H}"
          preserveAspectRatio="none"
          role="img"
          aria-label="fleet output over time"
          @mousemove=${this.onMove}
          @mouseleave=${this.onLeave}
        >
          <defs>
            <linearGradient id="pc-grad" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stop-color="var(--accent)" stop-opacity="0.35" />
              <stop offset="100%" stop-color="var(--accent)" stop-opacity="0" />
            </linearGradient>
          </defs>
          ${svg`<path class="area" d=${area} />`}
          ${svg`<path class="line" d=${line} />`}
          ${hovering
            ? svg`<line class="cross" x1=${hx} y1="0" x2=${hx} y2=${H} /><circle class="cursor" cx=${hx} cy=${hy} r="3.5" />`
            : nothing}
        </svg>
        ${hovering
          ? html`<div class="tip" style="left:${(hx / W) * 100}%; top:${hy}px">
              <span class="w">${fmtW(pts[i].w)}</span>
              <span class="t">· ${fmtTime(pts[i].t)}</span>
            </div>`
          : nothing}
      </div>
      <div class="labels">
        <span>now <span class="cur">${fmtW(cur)}</span></span>
        <span>peak ${fmtW(max)}</span>
      </div>
    `;
  }
}

customElements.define("power-chart", PowerChart);

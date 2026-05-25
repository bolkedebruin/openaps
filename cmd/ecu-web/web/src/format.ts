// Pure presentation helpers. Kept free of DOM/Lit so they unit-test
// directly and the components stay thin.

/**
 * Round a value to at most 3 decimals and drop trailing zeros, so raw device
 * floats (e.g. 16.569348154600167, 52.00002496) read cleanly (16.569, 52).
 */
export function fmtNum(v: number): string {
  if (!Number.isFinite(v)) return "";
  return String(Number(v.toFixed(3)));
}

/** Format watts, switching to kW at 1000. */
export function fmtW(w: number): string {
  if (!Number.isFinite(w)) return "—";
  if (Math.abs(w) >= 1000) return `${(w / 1000).toFixed(2)} kW`;
  return `${Math.round(w)} W`;
}

/** Format watt-hours, scaling to kWh / MWh. */
export function fmtWh(wh: number): string {
  if (!Number.isFinite(wh)) return "—";
  const a = Math.abs(wh);
  if (a >= 1e6) return `${(wh / 1e6).toFixed(2)} MWh`;
  if (a >= 1000) return `${(wh / 1000).toFixed(2)} kWh`;
  return `${Math.round(wh)} Wh`;
}

export function fmtPct(p: number): string {
  return Number.isFinite(p) ? `${p.toFixed(0)}%` : "—";
}

export function fmtV(v: number): string {
  return v > 0 ? `${v.toFixed(1)} V` : "—";
}

export function fmtHz(h: number): string {
  return h > 0 ? `${h.toFixed(2)} Hz` : "—";
}

export function fmtA(a: number): string {
  return Number.isFinite(a) ? `${a.toFixed(2)} A` : "—";
}

export type Load = "idle" | "low" | "mid" | "high";

/** Bucket a load percentage for colour coding. */
export function loadClass(pct: number): Load {
  if (!(pct > 0)) return "idle";
  if (pct < 40) return "low";
  if (pct < 85) return "mid";
  return "high";
}

/** Human-friendly "n ago" from an age in seconds. */
export function ageLabel(s: number): string {
  if (!Number.isFinite(s) || s < 0) return "—";
  if (s < 60) return `${Math.round(s)}s ago`;
  if (s < 3600) return `${Math.round(s / 60)}m ago`;
  return `${Math.round(s / 3600)}h ago`;
}

/** Turn a snake_case fault key into a Title Case label. */
export function humanizeFault(key: string): string {
  return key.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());
}

/** Active (true) fault keys of a faults object, humanised. */
export function faultLabels(faults: Record<string, boolean> | undefined): string[] {
  if (!faults) return [];
  return Object.keys(faults)
    .filter((k) => faults[k])
    .map(humanizeFault);
}

/** Absolute local timestamp from unix ms. */
export function fmtTime(ms: number): string {
  if (!ms) return "—";
  return new Date(ms).toLocaleString(undefined, { hour12: false });
}

export type Severity = "info" | "warn" | "err";

/** Bucket a free-form severity string for colour coding. */
export function severityClass(sev: string): Severity {
  const s = (sev || "").toLowerCase();
  if (s === "error" || s === "critical" || s === "crit" || s === "fault") return "err";
  if (s === "warn" || s === "warning") return "warn";
  return "info";
}

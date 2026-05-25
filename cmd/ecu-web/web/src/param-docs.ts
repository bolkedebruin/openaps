// Human-facing documentation for grid-protection parameters, the SunSpec
// model groups they belong to, and the cross-parameter constraints that must
// hold for a coherent profile. Pure presentation/validation content — the
// authoritative parameter set (codes, units, capability) comes from inv-driver.

export interface ParamDoc {
  label: string; // plain-language name
  desc: string; // one-line explanation
}

// Keyed by APsystems aps_code. Codes without an entry fall back to a
// prettified long_name (see prettifyName).
export const PARAM_DOCS: Record<string, ParamDoc> = {
  // --- Voltage trips (MustTrip) ---
  AC: { label: "Undervoltage trip — stage 2", desc: "Disconnect when AC voltage drops to this lower-stage level." },
  AQ: { label: "Undervoltage trip — deep", desc: "Disconnect quickly when voltage falls this far below nominal." },
  AH: { label: "Undervoltage trip — fast", desc: "Fast disconnect on a severe undervoltage." },
  AD: { label: "Overvoltage trip — slow", desc: "Disconnect when AC voltage rises above this (slower stage)." },
  AY: { label: "Overvoltage trip — slow (stage 2)", desc: "Second slower overvoltage disconnect threshold." },
  AB: { label: "10-minute mean overvoltage", desc: "Trips if the 10-minute average voltage exceeds this (EN 50549 sustained-overvoltage limit)." },
  AI: { label: "Overvoltage trip — fast", desc: "Fast disconnect on a severe overvoltage." },
  // --- Frequency trips (MustTrip) ---
  AE: { label: "Underfrequency trip — slow", desc: "Disconnect when grid frequency falls below this (slower stage)." },
  AJ: { label: "Underfrequency trip — fast", desc: "Fast disconnect on a severe underfrequency." },
  AF: { label: "Overfrequency trip — slow", desc: "Disconnect when grid frequency rises above this (slower stage)." },
  AK: { label: "Overfrequency trip — fast", desc: "Fast disconnect on a severe overfrequency." },
  // --- Clearance / trip times (MustTrip) ---
  BB: { label: "Undervoltage 1 — clearance time", desc: "How long the undervoltage condition must persist before tripping." },
  BD: { label: "Undervoltage 2 — clearance time", desc: "Clearance delay for the second undervoltage stage." },
  BC: { label: "Overvoltage 1 — clearance time", desc: "How long the overvoltage condition must persist before tripping." },
  BE: { label: "Overvoltage 2 — clearance time", desc: "Clearance delay for the second overvoltage stage." },
  BH: { label: "Underfrequency 1 — clearance time", desc: "Clearance delay for the first underfrequency stage." },
  BJ: { label: "Underfrequency 2 — clearance time", desc: "Clearance delay for the second underfrequency stage." },
  BI: { label: "Overfrequency 1 — clearance time", desc: "Clearance delay for the first overfrequency stage." },
  BK: { label: "Overfrequency 2 — clearance time", desc: "Clearance delay for the second overfrequency stage." },
  // --- Enter service (DEREnterService) ---
  BN: { label: "Enter-service voltage — lower", desc: "Voltage must be above this before the inverter reconnects." },
  BO: { label: "Enter-service voltage — upper", desc: "Voltage must be below this before the inverter reconnects." },
  BP: { label: "Enter-service frequency — lower", desc: "Frequency must be above this before the inverter reconnects." },
  BQ: { label: "Enter-service frequency — upper", desc: "Frequency must be below this before the inverter reconnects." },
  AG: { label: "Grid-recovery delay", desc: "Wait time after the grid is healthy before reconnecting." },
  AS: { label: "Power ramp time", desc: "Time taken to ramp output back up after reconnecting." },
  // --- Frequency-Watt droop (DERFreqDroop) ---
  CV: { label: "Curtailment enable (droop)", desc: "Enables the over-frequency droop power reduction (0 = off, 1 = on)." },
  CA: { label: "Curtailment start (droop deadband)", desc: "Over-frequency droop: power reduction begins at this frequency (deadband end)." },
  DD: { label: "Curtailment slope (droop)", desc: "Over-frequency droop gradient: % of rated power reduced per Hz above the start." },
  CG: { label: "Curtailment response time (droop)", desc: "Filter/response time of the droop control loop." },
  // --- Legacy Frequency-Watt curve (CrvSet) ---
  DH: { label: "Under-freq curve — low", desc: "Legacy frequency-Watt curve: lower frequency point of the under-frequency response." },
  DI: { label: "Under-freq curve — high", desc: "Legacy frequency-Watt curve: upper frequency point of the under-frequency response." },
  CB: { label: "Over-freq curve — start", desc: "Legacy frequency-Watt curve: over-frequency power reduction begins at this frequency." },
  CC: { label: "Over-freq curve — end", desc: "Legacy frequency-Watt curve: over-frequency reduction reaches its limit at this frequency." },
};

export interface GroupDoc {
  label: string; // section title
  tip: string; // legend tooltip
}

// Keyed by the SunSpec model group name inv-driver reports.
export const GROUP_DOCS: Record<string, GroupDoc> = {
  DERFreqDroop: {
    label: "Frequency-Watt droop",
    tip: "Linearly reduces active power as frequency rises above a deadband — over-frequency curtailment (SunSpec DERFreqDroop, model 711).",
  },
  CrvSet: {
    label: "Frequency-Watt curve",
    tip: "Legacy point-based power-versus-frequency response curve (model 134).",
  },
  MustTrip: {
    label: "Trip thresholds",
    tip: "Voltage and frequency limits that disconnect the inverter from the grid when crossed (protection trips).",
  },
  DEREnterService: {
    label: "Enter service",
    tip: "The voltage/frequency window and timing the inverter must satisfy before (re)connecting after a trip.",
  },
};

// Preferred section order, most-used first.
export const GROUP_ORDER = ["DERFreqDroop", "CrvSet", "MustTrip", "DEREnterService"];

// Groups collapsed by default (the long, less-frequently-edited ones).
export const GROUP_COLLAPSED_BY_DEFAULT = new Set(["MustTrip", "DEREnterService"]);

export function prettifyName(longName: string, apsCode: string): string {
  if (!longName) return apsCode;
  return longName
    .replace(/_/g, " ")
    .replace(/\b\w/g, (c) => c.toUpperCase());
}

export function paramLabel(apsCode: string, longName?: string): string {
  return PARAM_DOCS[apsCode]?.label ?? prettifyName(longName ?? "", apsCode);
}

export function paramDesc(apsCode: string): string {
  return PARAM_DOCS[apsCode]?.desc ?? "";
}

// Cross-parameter ordering constraints come from the server (single source of
// truth in Go; also enforced there), passed in from the /api/profiles payload.
import type { ConflictRule } from "./api.ts";

/**
 * conflicts evaluates the server-provided rules against the effective values
 * (override if set, else default) and returns a message per violation.
 */
export function conflicts(
  rules: ConflictRule[],
  effective: (apsCode: string) => number | undefined,
): string[] {
  const out: string[] = [];
  for (const r of rules) {
    const a = effective(r.left);
    const b = effective(r.right);
    if (a !== undefined && b !== undefined && !(a < b)) out.push(r.message);
  }
  return out;
}

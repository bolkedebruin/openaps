import type { Inverter } from "./api.ts";

// MIN_PANEL_W / MAX_PANEL_W mirror the codec's per-panel set-power envelope
// (codec.MinPanelLimitW / MaxPanelLimitW). The cap is modelled as a fraction
// of nameplate: per-panel MAX_PANEL_W (500) = uncapped = full nameplate output,
// so the cap maps linearly onto the load bar (% of nameplate) and always
// reaches the end — independent of how many panels telemetry reports.
export const MIN_PANEL_W = 20;
export const MAX_PANEL_W = 500;

// capCeilW is "possible" — the nameplate rating (the top of the cap range).
export function capCeilW(inv: Inverter): number {
  return inv.nameplate_w || 0;
}

// capFloorW is the lowest cap the inverter accepts: the per-panel floor as a
// fraction of nameplate (20/500 of nameplate).
export function capFloorW(inv: Inverter): number {
  return Math.round((capCeilW(inv) * MIN_PANEL_W) / MAX_PANEL_W);
}

// readbackCapW is the inverter's stored output cap in AC watts, from the
// protection read-back (code "DA", per-panel where 500 = full): cap =
// (DA / 500) × nameplate. Returns undefined when no read-back is available, so
// callers can fall back to nameplate (uncapped).
export function readbackCapW(inv: Inverter): number | undefined {
  const perPanel = inv.protection?.["DA"];
  if (perPanel == null) return undefined;
  return Math.round((perPanel / MAX_PANEL_W) * capCeilW(inv));
}

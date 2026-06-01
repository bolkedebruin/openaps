// Typed client for the ecu-web JSON/SSE API. The browser only ever
// talks to ecu-web; ecu-web is the one that talks to inv-driver.

export interface Panel {
  index: number;
  dc_v: number;
  dc_a: number;
  w: number;
}

export interface Inverter {
  uid: string;
  short_addr: number;
  model: string;
  model_code: number;
  phase: number;
  sw_version: number;
  online: boolean;
  last_seen_ms: number;
  age_s: number;
  active_power_w: number;
  nameplate_w: number;
  load_pct: number;
  grid_v: number;
  bus_v: number;
  freq_hz: number;
  reactive_var: number;
  rssi: number;
  lqi: number;
  panels: Panel[];
  faults?: Record<string, boolean>;
  protection?: Record<string, number>;
  zigbee_bound?: boolean;
  turned_off?: boolean;
  // encrypted is the last-observed frame type per inverter: true = AES
  // (CC EE/FC FC), false = plaintext (a misconfigured/foreign unit). It is
  // absent until inv-driver surfaces it on the fleet snapshot (the proto
  // Telemetry/InverterInfo path is owned by another agent), so the UI treats
  // `undefined` as "unknown" and renders a neutral badge.
  encrypted?: boolean;
}

export interface Fleet {
  ts_ms: number;
  nameplate_total_w: number;
  inverter_count: number;
  online_count: number;
  active_power_w: number;
  lifetime_wh: number;
  today_wh: number;
  month_wh: number;
  year_wh: number;
  inverters: Inverter[];
}

export interface AuthStatus {
  configured: boolean;
  authenticated: boolean;
  recovery_set?: boolean;
}

export interface EcuIdentity {
  ecu_id: string;
  hostname: string;
}

export interface Peer {
  backend: string;
  version: string;
  hostname: string;
  role: string;
  connected_at_ms: number;
  peer_uid: number;
  controller: boolean;
}

export interface SystemStatus {
  invdriver_connected: boolean;
  sse_clients: number;
  ecu?: EcuIdentity;
  peers: Peer[];
  status_error?: string;
}

// errorMessage extracts a human-readable error from a non-OK response.
// Many endpoints return a JSON envelope like {"ok":false,"error":"..."}
// even on 4xx; surface that string instead of the raw JSON blob.
async function errorMessage(res: Response, path: string): Promise<string> {
  const text = (await res.text()).trim();
  if (text) {
    try {
      const obj = JSON.parse(text) as { error?: unknown };
      if (typeof obj?.error === "string" && obj.error) return obj.error;
    } catch {
      // not JSON — fall through to raw text
    }
    return text;
  }
  return `${path}: ${res.status}`;
}

async function getJSON<T>(path: string): Promise<T> {
  const res = await fetch(path, { credentials: "same-origin" });
  if (!res.ok) throw new Error(await errorMessage(res, path));
  return (await res.json()) as T;
}

async function postJSON(path: string, body: unknown): Promise<void> {
  const res = await fetch(path, {
    method: "POST",
    credentials: "same-origin",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok) throw new Error(await errorMessage(res, path));
}

async function postJSONResult<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(path, {
    method: "POST",
    credentials: "same-origin",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok) throw new Error(await errorMessage(res, path));
  return (await res.json()) as T;
}

async function putJSON<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(path, {
    method: "PUT",
    credentials: "same-origin",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok) throw new Error(await errorMessage(res, path));
  return (await res.json()) as T;
}

export interface Event {
  id: number;
  ts_ms: number;
  inverter_uid?: string;
  kind: string;
  severity: string;
  short_addr?: number;
  detail?: string;
  raw_hex?: string;
  by?: string; // originating backend (Hello name)
}

export interface EventsResult {
  events: Event[];
  error?: string;
}

export interface EventsQuery {
  since_ms?: number;
  kind?: string;
  severity?: string;
  inverter_uid?: string;
  limit?: number;
}

export interface Settings {
  ecu_id: string;
  mac: string;
  pan_override: string;
  zigbee_type: string;
  channel?: number; // ZigBee channel: 0 = derive/default, else 11..26
  inverter_names?: Record<string, string>;
}

export interface SettingsResult {
  settings?: Settings;
  error?: string;
}

export interface PowerResult {
  results: { uid: string; ok: boolean; applied_watts: number; error?: string }[];
  error?: string;
}

export interface GridProfileSummary {
  id: string;
  vnom_v: number;
  source_ref?: string;
  point_count: number;
}

export interface ParamInfo {
  aps_code: string;
  long_name?: string;
  unit: string;
  group: string;
  model: number;
  polarity?: "under" | "over" | "";
}

export interface OverlayPoint {
  aps_code: string;
  value: number;
  unit?: string;
}

export interface LocalSiteProfile {
  id: string;
  uids: string[];
  points: OverlayPoint[];
}

export interface ProfileInverter {
  uid: string;
  model: string;
  model_code: number;
  writable_codes: string[];
  current: Record<string, number>; // aps_code -> the inverter's current value
}

export interface BaseDefault {
  value: number;
  unit: string;
  min?: number;
  max?: number;
}

export interface ConflictRule {
  left: string;
  right: string;
  message: string;
}

export interface ProfilesState {
  base: {
    active_base: string;
    reconciler_ready: boolean;
    profiles: GridProfileSummary[];
  };
  base_defaults: Record<string, BaseDefault>;
  overlays: LocalSiteProfile[];
  inverters: ProfileInverter[];
  params: ParamInfo[];
  conflict_rules: ConflictRule[];
  error?: string;
}

export interface ApplyResult {
  uid: string;
  ok: boolean;
  error?: string;
}

// OverlayApplyResponse is the synchronous response shape of DELETE
// /api/profiles/overlay: one per-UID outcome per target inverter.
export interface OverlayApplyResponse {
  id: string;
  results: ApplyResult[];
}

// OverlayQueuedResponse is the asynchronous response shape of PUT
// /api/profiles/overlay (HTTP 202): the overlay is persisted, the per-UID
// reconcile runs in the background on inv-driver, and outcomes land in the
// audit-events log under by="inv-driver". `failed` carries any uids whose
// persist-and-queue step itself errored (so the response is "partial": some
// uids queued, others rejected up front). When `uids` is empty the response
// is HTTP 400 with the same shape.
export interface OverlayQueuedResponse {
  id: string;
  status: "queued";
  uids: string[];
  failed?: { uid: string; error: string }[];
}

// --- Pairing ---

// PairingStage is the coarse phase of an in-flight pairing op. "" / "done" /
// "aborted" / "error" are terminal (no op running, or the last op's outcome).
export type PairingStage =
  | ""
  | "scan"
  | "bind"
  | "migrate"
  | "configure"
  | "rekey"
  | "change_channel"
  | "done"
  | "aborted"
  | "error";

// PairingPerInverter is one inverter's sub-status within the active op.
export interface PairingPerInverter {
  serial: string;
  short_addr?: number;
  state: string; // e.g. found / binding / migrating / configured / failed
  encrypted?: boolean;
}

// PairingSweep is the channel range a slow scan / rekey is currently parked on.
export interface PairingSweep {
  chan: number;
  chan_lo: number;
  chan_hi: number;
}

// PairingStatus mirrors inv-driver's in-memory progress snapshot (the contract
// shape). It is the source of truth for the progress drawer. An idle driver
// returns op="" / stage="".
export interface PairingStatus {
  op: string; // scan / add / replace / rekey / "" when idle
  stage: PairingStage;
  total: number;
  done: number;
  current_serial?: string;
  substep?: string;
  sweep?: PairingSweep;
  per_inverter?: PairingPerInverter[];
  message?: string;
  error?: string;
  started_ms?: number;
  updated_ms?: number;
}

// PairingResp wraps the status JSON returned by every pairing endpoint.
export interface PairingResp {
  ok: boolean;
  error?: string;
  status?: PairingStatus;
}

// pairingActive reports whether an op is currently running (a non-terminal
// stage). The drawer polls while this is true.
export function pairingActive(st: PairingStatus | null | undefined): boolean {
  if (!st || !st.op) return false;
  return st.stage !== "" && st.stage !== "done" && st.stage !== "aborted" && st.stage !== "error";
}

async function delJSON<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(path, {
    method: "DELETE",
    credentials: "same-origin",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok) throw new Error(await errorMessage(res, path));
  return (await res.json()) as T;
}

export const api = {
  authStatus: () => getJSON<AuthStatus>("/api/auth/status"),
  setup: (password: string) =>
    postJSONResult<{ ok: boolean; recovery_code: string }>("/api/auth/setup", { password }),
  login: (password: string) => postJSON("/api/auth/login", { password }),
  logout: () => postJSON("/api/auth/logout", {}),
  recover: (recovery_code: string, password: string) =>
    postJSONResult<{ ok: boolean; recovery_code: string }>("/api/auth/recover", {
      recovery_code,
      password,
    }),
  changePassword: (current_password: string, new_password: string) =>
    postJSON("/api/auth/change-password", { current_password, new_password }),
  regenerateRecovery: () =>
    postJSONResult<{ recovery_code: string }>("/api/auth/recovery", {}),
  fleet: () => getJSON<Fleet>("/api/fleet"),
  system: () => getJSON<SystemStatus>("/api/system"),
  history: () => getJSON<{ t: number; w: number }[]>("/api/history"),
  events: (q: EventsQuery = {}) => {
    const p = new URLSearchParams();
    if (q.since_ms) p.set("since_ms", String(q.since_ms));
    if (q.kind) p.set("kind", q.kind);
    if (q.severity) p.set("severity", q.severity);
    if (q.inverter_uid) p.set("inverter_uid", q.inverter_uid);
    if (q.limit) p.set("limit", String(q.limit));
    const qs = p.toString();
    return getJSON<EventsResult>("/api/events" + (qs ? `?${qs}` : ""));
  },
  getSettings: async (): Promise<SettingsResult> => {
    const r = await getJSON<Settings & { error?: string }>("/api/settings");
    if (r.error) return { error: r.error };
    return {
      settings: {
        ecu_id: r.ecu_id,
        mac: r.mac,
        pan_override: r.pan_override,
        zigbee_type: r.zigbee_type,
        channel: r.channel,
        inverter_names: r.inverter_names ?? {},
      },
    };
  },
  saveSettings: (s: Settings) => putJSON<Settings>("/api/settings", s),
  // verifyPassword: confirms the operator's password without changing it.
  // Returns true on 200, false on 401, throws otherwise.
  verifyPassword: async (password: string): Promise<boolean> => {
    const res = await fetch("/api/auth/verify", {
      method: "POST",
      credentials: "same-origin",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ password }),
    });
    if (res.status === 200) return true;
    if (res.status === 401) return false;
    const text = await res.text();
    throw new Error(text.trim() || `/api/auth/verify: ${res.status}`);
  },
  setPower: (req: { uid?: string; array?: boolean; watts: number }) =>
    postJSONResult<PowerResult>("/api/power", req),
  profiles: () => getJSON<ProfilesState>("/api/profiles"),
  overlays: () => getJSON<LocalSiteProfile[]>("/api/overlays"),
  selectBase: (id: string) => postJSON("/api/profiles/base", { id }),
  saveOverlay: (p: { id: string; uids: string[]; points: OverlayPoint[] }) =>
    putJSON<OverlayQueuedResponse>("/api/profiles/overlay", p),
  deleteOverlay: (id: string, uids: string[]) =>
    delJSON<OverlayApplyResponse>("/api/profiles/overlay", { id, uids }),
  // --- Pairing ---
  pairingScan: (req: { slow?: boolean; chan_lo?: number; chan_hi?: number; dwell_ms?: number } = {}) =>
    postJSONResult<PairingResp>("/api/pairing/scan", req),
  pairingAdd: (serial: string) => postJSONResult<PairingResp>("/api/pairing/add", { serial }),
  pairingReplace: (old_uid: string, new_serial: string) =>
    postJSONResult<PairingResp>("/api/pairing/replace", { old_uid, new_serial }),
  pairingRekey: (new_pan: string, channel = 0) =>
    postJSONResult<PairingResp>("/api/pairing/rekey", { new_pan, channel }),
  pairingChangeChannel: (channel: number) =>
    postJSONResult<PairingResp>("/api/pairing/change-channel", { channel }),
  pairingAbort: () => postJSONResult<PairingResp>("/api/pairing/abort", {}),
  pairingStatus: () => getJSON<PairingResp>("/api/pairing/status"),
};

/**
 * Subscribe to the live fleet stream. Returns a close function.
 * Reconnection is handled natively by EventSource.
 */
export function streamFleet(
  onFleet: (f: Fleet) => void,
  onError?: () => void,
): () => void {
  const es = new EventSource("/api/stream");
  es.addEventListener("fleet", (e) => {
    try {
      onFleet(JSON.parse((e as MessageEvent).data) as Fleet);
    } catch {
      /* ignore malformed frame */
    }
  });
  es.onerror = () => onError?.();
  return () => es.close();
}

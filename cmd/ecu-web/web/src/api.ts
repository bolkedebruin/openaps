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

async function getJSON<T>(path: string): Promise<T> {
  const res = await fetch(path, { credentials: "same-origin" });
  if (!res.ok) throw new Error(`${path}: ${res.status}`);
  return (await res.json()) as T;
}

async function postJSON(path: string, body: unknown): Promise<void> {
  const res = await fetch(path, {
    method: "POST",
    credentials: "same-origin",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text.trim() || `${path}: ${res.status}`);
  }
}

async function putJSON<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(path, {
    method: "PUT",
    credentials: "same-origin",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text.trim() || `${path}: ${res.status}`);
  }
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
  inverter_names?: Record<string, string>;
}

export interface SettingsResult {
  settings?: Settings;
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
}

export interface ProfilesState {
  base: {
    active_base: string;
    reconciler_ready: boolean;
    profiles: GridProfileSummary[];
  };
  overlays: LocalSiteProfile[];
  inverters: ProfileInverter[];
  params: ParamInfo[];
  error?: string;
}

export interface ApplyResult {
  uid: string;
  ok: boolean;
  error?: string;
}

export interface OverlayApplyResponse {
  id: string;
  results: ApplyResult[];
}

async function delJSON<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(path, {
    method: "DELETE",
    credentials: "same-origin",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text.trim() || `${path}: ${res.status}`);
  }
  return (await res.json()) as T;
}

export const api = {
  authStatus: () => getJSON<AuthStatus>("/api/auth/status"),
  setup: (password: string) => postJSON("/api/auth/setup", { password }),
  login: (password: string) => postJSON("/api/auth/login", { password }),
  logout: () => postJSON("/api/auth/logout", {}),
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
        inverter_names: r.inverter_names ?? {},
      },
    };
  },
  saveSettings: (s: Settings) => putJSON<Settings>("/api/settings", s),
  profiles: () => getJSON<ProfilesState>("/api/profiles"),
  selectBase: (id: string) => postJSON("/api/profiles/base", { id }),
  saveOverlay: (p: { id: string; uids: string[]; points: OverlayPoint[] }) =>
    putJSON<OverlayApplyResponse>("/api/profiles/overlay", p),
  deleteOverlay: (id: string, uids: string[]) =>
    delJSON<OverlayApplyResponse>("/api/profiles/overlay", { id, uids }),
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

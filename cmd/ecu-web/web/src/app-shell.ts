import { LitElement, html, css, nothing } from "lit";
import { api, streamFleet, type Fleet, type SystemStatus, type Settings } from "./api.ts";
import "./views/login-view.ts";
import "./views/dashboard-view.ts";
import "./views/inverters-view.ts";
import "./views/alarms-view.ts";
import "./views/events-view.ts";
import "./views/profiles-view.ts";
import "./views/settings-view.ts";

interface NavItem {
  id: string;
  label: string;
  icon: string;
}

const NAV: NavItem[] = [
  { id: "dashboard", label: "Dashboard", icon: "▮▮" },
  { id: "inverters", label: "Inverters", icon: "⌁" },
  { id: "alarms", label: "Alarms", icon: "!" },
  { id: "events", label: "Events", icon: "≣" },
  { id: "profiles", label: "Profiles", icon: "⛭" },
  { id: "settings", label: "Settings", icon: "⚙" },
];

/**
 * <ecu-app> is the top-level shell. It owns auth state, the hash route,
 * and the two live data feeds (SSE fleet stream + periodic system poll),
 * passing them down to the active view.
 */
export class EcuApp extends LitElement {
  static properties = {
    ready: { state: true },
    authed: { state: true },
    configured: { state: true },
    route: { state: true },
    fleet: { state: true },
    system: { state: true },
    names: { state: true },
  };

  declare ready: boolean;
  declare authed: boolean;
  declare configured: boolean;
  declare route: string;
  declare fleet: Fleet | null;
  declare system: SystemStatus | null;
  declare names: Record<string, string>;

  private closeSSE: (() => void) | null = null;
  private sysTimer: ReturnType<typeof setInterval> | null = null;
  private settingsCache: Settings | null = null;

  constructor() {
    super();
    this.ready = false;
    this.authed = false;
    this.configured = true;
    this.route = "dashboard";
    this.fleet = null;
    this.system = null;
    this.names = {};
  }

  static styles = css`
    :host { display: block; }
    .layout { display: grid; grid-template-columns: 220px 1fr; min-height: 100vh; }
    nav {
      background: var(--surface);
      border-right: 1px solid var(--border);
      padding: 20px 12px;
    }
    .brand {
      font-weight: 800;
      letter-spacing: 0.06em;
      color: var(--accent);
      padding: 0 12px 20px;
      font-size: 16px;
    }
    a.item {
      display: flex;
      align-items: center;
      gap: 10px;
      padding: 10px 12px;
      border-radius: 8px;
      color: var(--muted);
      text-decoration: none;
      font-size: 14px;
      margin-bottom: 2px;
    }
    a.item:hover { background: var(--bar-bg); color: var(--text); }
    a.item.active { background: color-mix(in srgb, var(--accent) 18%, transparent); color: var(--accent); }
    .ic { width: 18px; text-align: center; opacity: 0.8; }
    main { padding: 24px 28px; }
    .topbar {
      display: flex;
      align-items: center;
      justify-content: space-between;
      margin-bottom: 22px;
    }
    h1 { font-size: 20px; margin: 0; color: var(--text); }
    .right { display: flex; align-items: center; gap: 16px; }
    .conn { font-size: 12px; color: var(--muted); display: flex; align-items: center; gap: 6px; }
    .dot { width: 8px; height: 8px; border-radius: 50%; }
    .dot.on { background: var(--ok); box-shadow: 0 0 6px var(--ok); }
    .dot.off { background: var(--err); }
    button.logout {
      background: transparent;
      border: 1px solid var(--border);
      color: var(--muted);
      border-radius: 8px;
      padding: 6px 12px;
      font-size: 13px;
      cursor: pointer;
    }
    button.logout:hover { color: var(--text); border-color: var(--muted); }
    @media (max-width: 720px) { .layout { grid-template-columns: 1fr; } nav { display: none; } }
  `;

  connectedCallback(): void {
    super.connectedCallback();
    window.addEventListener("hashchange", this.onHash);
    this.onHash();
    void this.init();
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    window.removeEventListener("hashchange", this.onHash);
    this.stopStreams();
  }

  private onHash = () => {
    const id = (location.hash.replace(/^#\/?/, "") || "dashboard").split("/")[0];
    this.route = NAV.some((n) => n.id === id) ? id : "dashboard";
  };

  private async init() {
    try {
      const st = await api.authStatus();
      this.configured = st.configured;
      this.authed = st.authenticated;
      if (this.authed) this.startStreams();
    } catch {
      /* leave unauthed */
    } finally {
      this.ready = true;
    }
  }

  private onAuthed = async () => {
    this.authed = true;
    this.startStreams();
  };

  private logout = async () => {
    try {
      await api.logout();
    } catch {
      /* ignore */
    }
    this.authed = false;
    this.stopStreams();
    this.fleet = null;
    this.system = null;
  };

  private startStreams() {
    this.stopStreams();
    this.closeSSE = streamFleet((f) => {
      this.fleet = f;
    });
    const poll = () => api.system().then((s) => (this.system = s)).catch(() => {});
    void poll();
    this.sysTimer = setInterval(poll, 5000);
    void this.fetchSettings();
  }

  private async fetchSettings() {
    try {
      const r = await api.getSettings();
      if (r.settings) {
        this.settingsCache = r.settings;
        this.names = r.settings.inverter_names ?? {};
      }
    } catch {
      /* names stay as-is */
    }
  }

  // onRename persists an inverter label edit. It merges into the full
  // current settings so the other fields aren't wiped by the whole-object PUT.
  private onRename = async (e: Event) => {
    const { uid, name } = (e as CustomEvent<{ uid: string; name: string }>).detail;
    const base = this.settingsCache ?? { ecu_id: "", mac: "", pan_override: "", zigbee_type: "" };
    const names: Record<string, string> = { ...(base.inverter_names ?? {}) };
    if (name.trim()) names[uid] = name.trim();
    else delete names[uid];
    const next: Settings = { ...base, inverter_names: names };
    try {
      await api.saveSettings(next);
      this.settingsCache = next;
      this.names = names;
    } catch {
      /* keep prior names on failure */
    }
  };

  private stopStreams() {
    this.closeSSE?.();
    this.closeSSE = null;
    if (this.sysTimer) clearInterval(this.sysTimer);
    this.sysTimer = null;
  }

  private activeView() {
    switch (this.route) {
      case "inverters":
        return html`<inverters-view
          .fleet=${this.fleet}
          .names=${this.names}
          @rename=${this.onRename}
        ></inverters-view>`;
      case "alarms":
        return html`<alarms-view .fleet=${this.fleet}></alarms-view>`;
      case "events":
        return html`<events-view></events-view>`;
      case "profiles":
        return html`<profiles-view></profiles-view>`;
      case "settings":
        return html`<settings-view></settings-view>`;
      default:
        return html`<dashboard-view .fleet=${this.fleet} .system=${this.system} .names=${this.names}></dashboard-view>`;
    }
  }

  render() {
    if (!this.ready) return nothing;
    if (!this.authed) {
      return html`<login-view .configured=${this.configured} @authed=${this.onAuthed}></login-view>`;
    }
    const title = NAV.find((n) => n.id === this.route)?.label ?? "Dashboard";
    const connected = this.system?.invdriver_connected ?? false;
    return html`
      <div class="layout">
        <nav>
          <div class="brand">ECU CONSOLE</div>
          ${NAV.map(
            (n) => html`<a
              class="item ${this.route === n.id ? "active" : ""}"
              href="#/${n.id}"
            ><span class="ic">${n.icon}</span>${n.label}</a>`,
          )}
        </nav>
        <main>
          <div class="topbar">
            <h1>${title}</h1>
            <div class="right">
              <span class="conn">
                <span class="dot ${connected ? "on" : "off"}"></span>
                inv-driver ${connected ? "connected" : "down"}
              </span>
              <button class="logout" @click=${this.logout}>Sign out</button>
            </div>
          </div>
          ${this.activeView()}
        </main>
      </div>
    `;
  }
}

customElements.define("ecu-app", EcuApp);

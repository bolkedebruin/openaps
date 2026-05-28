import { LitElement, html, css, nothing } from "lit";
import { api, streamFleet, type Fleet, type SystemStatus, type Settings } from "./api.ts";
import "./views/login-view.ts";
import "./views/dashboard-view.ts";
import "./views/inverters-view.ts";
import "./views/alarms-view.ts";
import "./views/events-view.ts";
import "./views/profiles-view.ts";
import "./views/settings-view.ts";
import "./components/app-nav.ts";
import type { NavItem } from "./components/app-nav.ts";

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
    customProfiles: { state: true },
    navOpen: { state: true },
  };

  declare ready: boolean;
  declare authed: boolean;
  declare configured: boolean;
  declare route: string;
  declare fleet: Fleet | null;
  declare system: SystemStatus | null;
  declare names: Record<string, string>;
  declare customProfiles: Record<string, string>; // inverter uid -> active Local Site profile name
  declare navOpen: boolean; // mobile drawer open state

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
    this.customProfiles = {};
    this.navOpen = false;
  }

  static styles = css`
    :host { display: block; }
    .layout { display: grid; grid-template-columns: 220px 1fr; min-height: 100vh; }
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
    .titlewrap { display: flex; align-items: center; gap: 12px; min-width: 0; }
    h1 { white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
    button.hamburger {
      display: none;
      background: transparent;
      border: 1px solid var(--border);
      color: var(--text);
      border-radius: 8px;
      padding: 5px 10px;
      font-size: 17px;
      line-height: 1;
      cursor: pointer;
    }
    @media (max-width: 720px) {
      .layout {
        grid-template-columns: 1fr;
        /* On mobile, app-nav is position:fixed → its grid row is empty.
           Without this, the two implicit rows stretch (align-content: normal
           ≈ stretch) and split min-height:100vh 50/50, pushing main into the
           vertical middle. Pin row 1 to content (0) and row 2 to 1fr so main
           top-aligns and fills the viewport. */
        grid-template-rows: auto 1fr;
      }
      button.hamburger { display: inline-flex; }
      main { padding: 18px 16px; }
    }
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
    this.navOpen = false; // close the mobile drawer after navigating
    // Refresh the custom-profile flags when returning to the dashboard, so an
    // overlay just created/cleared on the Profiles screen is reflected.
    if (this.route === "dashboard" && this.authed) void this.fetchOverlays();
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
    this.configured = true; // setup/recovery may have just configured the password
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
    void this.fetchOverlays();
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

  // fetchOverlays maps each inverter uid to the Local Site profile active on
  // it, so the dashboard can flag inverters with a custom profile. Overlays
  // change rarely, so this is fetched on connect and on entering the dashboard.
  private async fetchOverlays() {
    try {
      const overlays = await api.overlays();
      const map: Record<string, string> = {};
      for (const o of overlays) for (const uid of o.uids) map[uid] = o.id;
      this.customProfiles = map;
    } catch {
      /* keep prior map */
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
        return html`<dashboard-view
          .fleet=${this.fleet}
          .system=${this.system}
          .names=${this.names}
          .profiles=${this.customProfiles}
        ></dashboard-view>`;
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
        <app-nav
          .items=${NAV}
          .route=${this.route}
          .open=${this.navOpen}
          @close=${() => (this.navOpen = false)}
        ></app-nav>
        <main>
          <div class="topbar">
            <div class="titlewrap">
              <button class="hamburger" aria-label="Menu" aria-expanded=${this.navOpen} @click=${() => (this.navOpen = !this.navOpen)}>☰</button>
              <h1>${title}</h1>
            </div>
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

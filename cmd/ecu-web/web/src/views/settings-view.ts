import { LitElement, html, css, nothing } from "lit";
import { api, type Settings, type GridProfileState } from "../api.ts";
import "../components/settings-form.ts";
import "../components/grid-profile-form.ts";

/**
 * <settings-view> fetches the current ECU settings, renders them in an
 * editable form, and writes changes back through inv-driver on save. It
 * also shows the active grid profile and lets the operator select a new
 * base, which inv-driver applies to every inverter.
 */
export class SettingsView extends LitElement {
  static properties = {
    settings: { state: true },
    error: { state: true },
    notice: { state: true },
    loading: { state: true },
    saving: { state: true },
    grid: { state: true },
    gridError: { state: true },
    gridNotice: { state: true },
    gridBusy: { state: true },
  };

  declare settings: Settings | null;
  declare error: string;
  declare notice: string;
  declare loading: boolean;
  declare saving: boolean;
  declare grid: GridProfileState | null;
  declare gridError: string;
  declare gridNotice: string;
  declare gridBusy: boolean;

  constructor() {
    super();
    this.settings = null;
    this.error = "";
    this.notice = "";
    this.loading = false;
    this.saving = false;
    this.grid = null;
    this.gridError = "";
    this.gridNotice = "";
    this.gridBusy = false;
  }

  static styles = css`
    :host { display: block; }
    .panel {
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: 10px;
      padding: 24px;
      max-width: 560px;
      margin-bottom: 20px;
    }
    h2 { font-size: 15px; margin: 0 0 16px; color: var(--text); }
    .banner { border-radius: 8px; padding: 10px 12px; font-size: 13px; margin-bottom: 16px; }
    .banner.ok { color: var(--ok); border: 1px solid var(--ok); background: color-mix(in srgb, var(--ok) 12%, transparent); }
    .banner.err { color: var(--err); border: 1px solid var(--err); background: color-mix(in srgb, var(--err) 12%, transparent); }
    .loading { color: var(--muted); font-size: 13px; }
  `;

  connectedCallback(): void {
    super.connectedCallback();
    void this.load();
    void this.loadGrid();
  }

  private async load() {
    this.loading = true;
    try {
      const res = await api.getSettings();
      this.settings = res.settings ?? null;
      this.error = res.error ?? "";
    } catch (e) {
      this.error = (e as Error).message;
    } finally {
      this.loading = false;
    }
  }

  private async loadGrid() {
    try {
      const g = await api.gridProfile();
      this.grid = g;
      this.gridError = g.error ?? "";
    } catch (e) {
      this.gridError = (e as Error).message;
    }
  }

  private onSave = async (e: CustomEvent<Settings>) => {
    this.saving = true;
    this.notice = "";
    this.error = "";
    try {
      this.settings = await api.saveSettings(e.detail);
      this.notice = "Settings saved.";
    } catch (err) {
      this.error = (err as Error).message;
    } finally {
      this.saving = false;
      await this.load();
    }
  };

  private onApplyProfile = async (e: CustomEvent<string>) => {
    const id = e.detail;
    if (!window.confirm(`Apply grid profile "${id}" to every inverter? This writes grid-protection settings across the whole fleet.`)) {
      return;
    }
    this.gridBusy = true;
    this.gridNotice = "";
    this.gridError = "";
    try {
      await api.selectGridProfile(id);
      this.gridNotice = `Grid profile "${id}" applied.`;
    } catch (err) {
      this.gridError = (err as Error).message;
    } finally {
      this.gridBusy = false;
      await this.loadGrid();
    }
  };

  render() {
    return html`
      <div class="panel">
        <h2>Grid profile</h2>
        ${this.gridNotice ? html`<div class="banner ok">${this.gridNotice}</div>` : nothing}
        ${this.gridError ? html`<div class="banner err">⚠ ${this.gridError}</div>` : nothing}
        ${this.grid
          ? html`<grid-profile-form
              .profiles=${this.grid.profiles ?? []}
              .activeBase=${this.grid.active_base ?? ""}
              .reconcilerReady=${this.grid.reconciler_ready ?? false}
              .busy=${this.gridBusy}
              @apply=${this.onApplyProfile}
            ></grid-profile-form>`
          : this.gridError
            ? nothing
            : html`<div class="loading">Loading…</div>`}
      </div>

      <div class="panel">
        <h2>ECU settings</h2>
        ${this.notice ? html`<div class="banner ok">${this.notice}</div>` : nothing}
        ${this.error ? html`<div class="banner err">⚠ ${this.error}</div>` : nothing}
        ${this.loading && !this.settings
          ? html`<div class="loading">Loading…</div>`
          : html`<settings-form
              .settings=${this.settings ?? { ecu_id: "", mac: "", pan_override: "", zigbee_type: "apsystems" }}
              @save=${this.onSave}
            ></settings-form>`}
      </div>
    `;
  }
}

customElements.define("settings-view", SettingsView);

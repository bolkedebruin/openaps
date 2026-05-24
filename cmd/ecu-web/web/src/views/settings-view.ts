import { LitElement, html, css, nothing } from "lit";
import { api, type Settings } from "../api.ts";
import "../components/settings-form.ts";

/**
 * <settings-view> fetches the current ECU settings, renders them in an
 * editable form, and writes changes back through inv-driver on save.
 */
export class SettingsView extends LitElement {
  static properties = {
    settings: { state: true },
    error: { state: true },
    notice: { state: true },
    loading: { state: true },
    saving: { state: true },
  };

  declare settings: Settings | null;
  declare error: string;
  declare notice: string;
  declare loading: boolean;
  declare saving: boolean;

  constructor() {
    super();
    this.settings = null;
    this.error = "";
    this.notice = "";
    this.loading = false;
    this.saving = false;
  }

  static styles = css`
    :host { display: block; }
    .panel {
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: 10px;
      padding: 24px;
      max-width: 560px;
    }
    .banner { border-radius: 8px; padding: 10px 12px; font-size: 13px; margin-bottom: 16px; }
    .banner.ok { color: var(--ok); border: 1px solid var(--ok); background: color-mix(in srgb, var(--ok) 12%, transparent); }
    .banner.err { color: var(--err); border: 1px solid var(--err); background: color-mix(in srgb, var(--err) 12%, transparent); }
    .loading { color: var(--muted); font-size: 13px; }
  `;

  connectedCallback(): void {
    super.connectedCallback();
    void this.load();
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

  render() {
    return html`
      ${this.notice ? html`<div class="banner ok">${this.notice}</div>` : nothing}
      ${this.error ? html`<div class="banner err">⚠ ${this.error}</div>` : nothing}
      <div class="panel">
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

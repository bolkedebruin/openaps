import { LitElement, html, css, nothing } from "lit";
import { api, type ProfilesState, type LocalSiteProfile } from "../api.ts";
import { fmtNum } from "../format.ts";
import { validateOverlay } from "../schemas/overlay.ts";
import "../components/grid-profile-form.ts";
import "../components/local-site-profile-form.ts";
import type { OverlayDraft } from "../components/local-site-profile-form.ts";

/**
 * <profiles-view> is the grid-configuration home. It owns the fleet-wide base
 * profile selector (moved from Settings) and the Local Site profiles: named
 * overlays the operator applies to individual inverters or groups.
 */
export class ProfilesView extends LitElement {
  static properties = {
    data: { state: true },
    names: { state: true },
    error: { state: true },
    notice: { state: true },
    baseBusy: { state: true },
    overlayBusy: { state: true },
    editing: { state: true },
    editingExisting: { state: true },
  };

  declare data: ProfilesState | null;
  declare names: Record<string, string>;
  declare error: string;
  declare notice: string;
  declare baseBusy: boolean;
  declare overlayBusy: boolean;
  declare editing: LocalSiteProfile | null; // the profile being edited, or null when not editing
  declare editingExisting: boolean;

  constructor() {
    super();
    this.data = null;
    this.names = {};
    this.error = "";
    this.notice = "";
    this.baseBusy = false;
    this.overlayBusy = false;
    this.editing = null;
    this.editingExisting = false;
  }

  static styles = css`
    :host { display: block; }
    .cols {
      display: grid;
      grid-template-columns: minmax(0, 1fr) 320px;
      gap: 20px;
      align-items: start;
      max-width: 1200px;
    }
    @media (max-width: 900px) { .cols { grid-template-columns: 1fr; } }
    .panel {
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: 10px;
      padding: 24px;
      margin-bottom: 20px;
      min-width: 0;
    }
    h2 { font-size: 15px; margin: 0 0 16px; color: var(--text); }
    .row { display: flex; align-items: center; justify-content: space-between; margin-bottom: 16px; }
    .banner { border-radius: 8px; padding: 10px 12px; font-size: 13px; margin-bottom: 16px; }
    .banner.ok { color: var(--ok); border: 1px solid var(--ok); background: color-mix(in srgb, var(--ok) 12%, transparent); }
    .banner.err { color: var(--err); border: 1px solid var(--err); background: color-mix(in srgb, var(--err) 12%, transparent); }
    .loading { color: var(--muted); font-size: 13px; }
    .hdr-actions { display: flex; gap: 10px; }
    button.primary { background: var(--accent); border: none; color: #04121a; border-radius: 8px; padding: 8px 14px; font-size: 13px; font-weight: 600; cursor: pointer; }
    button.primary:hover { filter: brightness(1.08); }
    button.ghost { background: transparent; border: 1px solid var(--border); color: var(--text); border-radius: 8px; padding: 8px 14px; font-size: 13px; font-weight: 600; cursor: pointer; }
    button.ghost:hover { border-color: var(--muted); }
    .cards { display: grid; gap: 12px; }
    .card { border: 1px solid var(--border); border-radius: 8px; padding: 14px 16px; }
    .card .title { font-size: 14px; font-weight: 600; color: var(--text); }
    .card .meta { font-size: 12px; color: var(--muted); margin-top: 4px; }
    .chips { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 8px; }
    .chip { font-size: 12px; background: var(--bar-bg); border: 1px solid var(--border); border-radius: 12px; padding: 2px 9px; color: var(--text); }
    .cardactions { display: flex; gap: 10px; margin-top: 12px; }
    .cardactions button { font-size: 12px; border-radius: 6px; padding: 5px 12px; cursor: pointer; border: 1px solid var(--border); background: transparent; color: var(--text); }
    .cardactions button.del { color: var(--err); border-color: color-mix(in srgb, var(--err) 50%, var(--border)); }
    .empty { color: var(--muted); font-size: 13px; }
  `;

  connectedCallback(): void {
    super.connectedCallback();
    void this.load();
  }

  private async load() {
    try {
      const [data, settings] = await Promise.all([api.profiles(), api.getSettings()]);
      this.data = data;
      this.error = data.error ?? "";
      this.names = settings.settings?.inverter_names ?? {};
    } catch (e) {
      this.error = (e as Error).message;
    }
  }

  private invName(uid: string): string {
    if (this.names[uid]) return this.names[uid];
    const inv = this.data?.inverters.find((i) => i.uid === uid);
    return inv?.model || uid;
  }

  private onSelectBase = async (e: CustomEvent<string>) => {
    const id = e.detail;
    if (!window.confirm(`Apply base grid profile "${id}" to every inverter? This writes grid-protection settings across the whole fleet.`)) return;
    this.baseBusy = true;
    this.notice = "";
    this.error = "";
    try {
      await api.selectBase(id);
      await this.load(); // refresh before showing the result so load() can't clobber it
      // The fleet reconcile runs asynchronously on inv-driver; progress and the
      // per-inverter outcome land in the Events log, not this HTTP response.
      this.notice = `Base profile "${id}" selected — reconciling the fleet now. See Events for per-inverter progress and results.`;
    } catch (err) {
      this.error = (err as Error).message;
    } finally {
      this.baseBusy = false;
    }
  };

  private newProfile() {
    this.editing = { id: "", uids: [], points: [] };
    this.editingExisting = false;
    this.notice = "";
    this.error = "";
  }

  private editProfile(p: LocalSiteProfile) {
    this.editing = p;
    this.editingExisting = true;
    this.notice = "";
    this.error = "";
  }

  private onCancelEdit = () => {
    this.editing = null;
  };

  // exportProfile downloads a Local Site profile as a .json file (overlay only:
  // id, target uids, and {aps_code, value} points).
  private exportProfile(p: LocalSiteProfile) {
    const data = { id: p.id, uids: p.uids, points: p.points.map((pt) => ({ aps_code: pt.aps_code, value: pt.value })) };
    const blob = new Blob([JSON.stringify(data, null, 2)], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `${p.id || "profile"}.json`;
    a.click();
    URL.revokeObjectURL(url);
  }

  private triggerImport = () => {
    (this.shadowRoot?.querySelector("#importfile") as HTMLInputElement | null)?.click();
  };

  private onImportFile = async (e: Event) => {
    const input = e.target as HTMLInputElement;
    const file = input.files?.[0];
    input.value = ""; // allow re-importing the same file
    if (!file) return;
    let raw: unknown;
    try {
      raw = JSON.parse(await file.text());
    } catch (err) {
      this.error = "Import failed: " + (err as Error).message;
      return;
    }
    const result = validateOverlay(raw);
    if (!result.ok) {
      const shown = result.errors.slice(0, 3).join("; ");
      const extra = result.errors.length > 3 ? ` (+${result.errors.length - 3} more)` : "";
      this.error = "Import failed: " + shown + extra;
      return;
    }
    this.editing = result.profile;
    this.editingExisting = false;
    this.error = "";
    const base = `Imported "${result.profile.id}" — review the targets and values, then Save.`;
    const warn = result.warnings.length > 0 ? ` — Note: ${result.warnings.join("; ")}` : "";
    this.notice = base + warn;
  };

  private onSaveOverlay = async (e: CustomEvent<OverlayDraft>) => {
    const d = e.detail;
    if (!window.confirm(`Apply Local Site profile "${d.id}" to ${d.uids.length} inverter(s)? This writes grid-protection parameters to each.`)) return;
    this.overlayBusy = true;
    this.notice = "";
    this.error = "";
    try {
      const resp = await api.saveOverlay(d);
      this.editing = null;
      await this.load(); // refresh before reporting so load() can't clobber the result
      const n = resp.uids.length;
      this.notice = `Overlay "${resp.id}" queued for ${n} inverter${n === 1 ? "" : "s"} — see Events for application results.`;
    } catch (err) {
      this.error = (err as Error).message;
    } finally {
      this.overlayBusy = false;
    }
  };

  private deleteProfile = async (p: LocalSiteProfile) => {
    if (!window.confirm(`Delete Local Site profile "${p.id}" and clear it from ${p.uids.length} inverter(s)?`)) return;
    this.overlayBusy = true;
    this.notice = "";
    this.error = "";
    try {
      const resp = await api.deleteOverlay(p.id, p.uids);
      if (this.editing?.id === p.id) this.editing = null;
      await this.load(); // refresh before reporting so load() can't clobber the result
      // Clearing reconciles each inverter back to the base asynchronously on
      // inv-driver; outcomes land in the Events log.
      const n = resp.uids.length;
      let note = `Profile "${p.id}" cleared from ${n} inverter${n === 1 ? "" : "s"} — reconciling back to the base profile now. See Events for results.`;
      if (resp.failed && resp.failed.length > 0) {
        const detail = resp.failed.map((f) => `${this.invName(f.uid)}: ${f.error || "rejected"}`).join("; ");
        note += ` Not queued on ${resp.failed.length} inverter(s): ${detail}`;
      }
      this.notice = note;
    } catch (err) {
      this.error = (err as Error).message;
    } finally {
      this.overlayBusy = false;
    }
  };

  private renderBase() {
    const b = this.data?.base;
    return html`
      <div class="panel">
        <h2>Base grid profile</h2>
        <grid-profile-form
          .profiles=${b?.profiles ?? []}
          .activeBase=${b?.active_base ?? ""}
          .reconcilerReady=${b?.reconciler_ready ?? false}
          .busy=${this.baseBusy}
          @apply=${this.onSelectBase}
        ></grid-profile-form>
      </div>
    `;
  }

  private renderLocalSite() {
    const d = this.data;
    return html`
      <div class="panel">
        <div class="row">
          <h2 style="margin:0">Local Site profiles</h2>
          ${this.editing === null
            ? html`<div class="hdr-actions">
                <button class="ghost" @click=${this.triggerImport}>Import</button>
                <button class="primary" @click=${() => this.newProfile()}>+ New profile</button>
              </div>`
            : nothing}
        </div>
        <input id="importfile" type="file" accept=".json,application/json" hidden @change=${this.onImportFile} />

        ${this.editing !== null
          ? html`<local-site-profile-form
              .params=${d?.params ?? []}
              .inverters=${d?.inverters ?? []}
              .defaults=${d?.base_defaults ?? {}}
              .rules=${d?.conflict_rules ?? []}
              .names=${this.names}
              .profile=${this.editing}
              .editing=${this.editingExisting}
              .busy=${this.overlayBusy}
              @save=${this.onSaveOverlay}
              @cancel=${this.onCancelEdit}
            ></local-site-profile-form>`
          : this.renderCards()}
      </div>
    `;
  }

  private renderCards() {
    const overlays = this.data?.overlays ?? [];
    if (overlays.length === 0) {
      return html`<div class="empty">No Local Site profiles yet. Create one to override grid-protection parameters on specific inverters.</div>`;
    }
    return html`<div class="cards">
      ${overlays.map(
        (p) => html`<div class="card">
          <div class="title">${p.id}</div>
          <div class="meta">Targets: ${p.uids.map((u) => this.invName(u)).join(", ") || "none"}</div>
          <div class="chips">
            ${p.points.map((pt) => html`<span class="chip">${pt.aps_code} = ${fmtNum(pt.value)}${pt.unit ? ` ${pt.unit}` : ""}</span>`)}
          </div>
          <div class="cardactions">
            <button @click=${() => this.editProfile(p)}>Edit</button>
            <button @click=${() => this.exportProfile(p)}>Export</button>
            <button class="del" @click=${() => this.deleteProfile(p)}>Delete</button>
          </div>
        </div>`,
      )}
    </div>`;
  }

  render() {
    return html`
      ${this.notice ? html`<div class="banner ok">${this.notice}</div>` : nothing}
      ${this.error ? html`<div class="banner err">⚠ ${this.error}</div>` : nothing}
      ${this.data === null
        ? html`<div class="panel"><div class="loading">Loading…</div></div>`
        : html`<div class="cols">
            <div>${this.renderLocalSite()}</div>
            <div>${this.renderBase()}</div>
          </div>`}
    `;
  }
}

customElements.define("profiles-view", ProfilesView);

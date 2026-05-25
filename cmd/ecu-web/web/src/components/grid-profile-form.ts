import { LitElement, html, css, nothing } from "lit";
import type { GridProfileSummary } from "../api.ts";

/**
 * <grid-profile-form> renders the active grid profile and a dropdown to
 * pick a different base profile. It is presentational: clicking Apply
 * dispatches a bubbling, composed "apply" event whose detail is the
 * selected profile id. The view owns the confirmation and the write.
 */
export class GridProfileForm extends LitElement {
  static properties = {
    profiles: { attribute: false },
    activeBase: { attribute: false },
    reconcilerReady: { attribute: false },
    busy: { attribute: false },
    selected: { state: true },
  };

  declare profiles: GridProfileSummary[];
  declare activeBase: string;
  declare reconcilerReady: boolean;
  declare busy: boolean;
  declare selected: string;

  constructor() {
    super();
    this.profiles = [];
    this.activeBase = "";
    this.reconcilerReady = true;
    this.busy = false;
    this.selected = "";
  }

  static styles = css`
    :host { display: block; }
    .grid { display: grid; gap: 16px; max-width: 460px; }
    .active { font-size: 14px; color: var(--text); }
    .active .muted { color: var(--muted); }
    .active .none { color: var(--muted); font-style: italic; }
    label { display: flex; flex-direction: column; gap: 6px; font-size: 13px; color: var(--muted); min-width: 0; }
    select {
      width: 100%;
      max-width: 100%;
      box-sizing: border-box;
      background: var(--bar-bg);
      border: 1px solid var(--border);
      color: var(--text);
      border-radius: 8px;
      padding: 9px 11px;
      font-size: 14px;
      font-family: inherit;
    }
    select:focus { outline: none; border-color: var(--accent); }
    .actions { display: flex; align-items: center; gap: 12px; margin-top: 4px; }
    button.apply {
      background: var(--accent);
      border: none;
      color: #04121a;
      border-radius: 8px;
      padding: 9px 18px;
      font-size: 14px;
      font-weight: 600;
      cursor: pointer;
    }
    button.apply:hover:not(:disabled) { filter: brightness(1.08); }
    button.apply:disabled { opacity: 0.45; cursor: not-allowed; }
    .hint { font-size: 12px; color: var(--muted); }
  `;

  private onChange = (e: Event) => {
    this.selected = (e.target as HTMLSelectElement).value;
  };

  private apply = () => {
    const id = this.effectiveSelected();
    if (!id || id === this.activeBase) return;
    this.dispatchEvent(new CustomEvent<string>("apply", { detail: id, bubbles: true, composed: true }));
  };

  // effectiveSelected is the dropdown value: the operator's pick, falling
  // back to the active base before they touch the control.
  private effectiveSelected(): string {
    return this.selected || this.activeBase;
  }

  private labelFor(p: GridProfileSummary): string {
    const parts = [`${p.vnom_v} V`];
    if (p.source_ref) parts.push(p.source_ref);
    parts.push(`${p.point_count} pts`);
    return `${p.id} — ${parts.join(" · ")}`;
  }

  render() {
    const sel = this.effectiveSelected();
    const active = this.profiles.find((p) => p.id === this.activeBase);
    const canApply = !this.busy && this.reconcilerReady && sel !== "" && sel !== this.activeBase;

    return html`
      <div class="grid">
        <div class="active">
          <span class="muted">Active profile:</span>
          ${this.activeBase
            ? html` <strong>${this.activeBase}</strong>${active
                ? html` <span class="muted">(${active.vnom_v} V · ${active.point_count} pts)</span>`
                : nothing}`
            : html` <span class="none">none selected</span>`}
        </div>

        <label>
          Base profile
          <select id="profile" .value=${sel} @change=${this.onChange} ?disabled=${this.busy}>
            ${this.activeBase ? nothing : html`<option value="" disabled selected>Select a profile…</option>`}
            ${this.profiles.map(
              (p) => html`<option value=${p.id} ?selected=${p.id === sel}>${this.labelFor(p)}</option>`,
            )}
          </select>
        </label>

        <div class="actions">
          <button class="apply" @click=${this.apply} ?disabled=${!canApply}>
            ${this.busy ? "Applying…" : "Apply"}
          </button>
          ${!this.reconcilerReady
            ? html`<span class="hint">reconciler not ready</span>`
            : sel && sel !== this.activeBase
              ? html`<span class="hint">applies to all inverters</span>`
              : nothing}
        </div>
      </div>
    `;
  }
}

customElements.define("grid-profile-form", GridProfileForm);

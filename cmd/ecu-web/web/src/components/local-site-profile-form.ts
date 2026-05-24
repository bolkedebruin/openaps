import { LitElement, html, css, nothing, type PropertyValues } from "lit";
import type { ParamInfo, ProfileInverter, LocalSiteProfile, OverlayPoint } from "../api.ts";

export interface OverlayDraft {
  id: string;
  uids: string[];
  points: OverlayPoint[];
}

/**
 * <local-site-profile-form> edits one Local Site profile: a name, the target
 * inverters (multi-select), and parameter override values. A parameter is only
 * editable when it is writable on EVERY selected target (the capability
 * intersection), so a saved profile never silently no-ops on a target. It is
 * presentational: Save dispatches a composed "save" event with an OverlayDraft;
 * Cancel dispatches "cancel". The view owns confirmation and the write.
 */
export class LocalSiteProfileForm extends LitElement {
  static properties = {
    params: { attribute: false },
    inverters: { attribute: false },
    profile: { attribute: false },
    names: { attribute: false },
    busy: { attribute: false },
    editing: { attribute: false },
    name: { state: true },
    selectedUids: { state: true },
    values: { state: true },
    localError: { state: true },
  };

  declare params: ParamInfo[];
  declare inverters: ProfileInverter[];
  declare profile: LocalSiteProfile | null;
  declare names: Record<string, string>;
  declare busy: boolean;
  declare editing: boolean;
  declare name: string;
  declare selectedUids: string[];
  declare values: Record<string, string>;
  declare localError: string;

  constructor() {
    super();
    this.params = [];
    this.inverters = [];
    this.profile = null;
    this.names = {};
    this.busy = false;
    this.editing = false;
    this.name = "";
    this.selectedUids = [];
    this.values = {};
    this.localError = "";
  }

  static styles = css`
    :host { display: block; }
    .grid { display: grid; gap: 18px; }
    label.field { display: flex; flex-direction: column; gap: 6px; font-size: 13px; color: var(--muted); }
    input[type="text"], input[type="number"] {
      background: var(--bar-bg); border: 1px solid var(--border); color: var(--text);
      border-radius: 8px; padding: 8px 10px; font-size: 14px; font-family: inherit;
    }
    input:focus { outline: none; border-color: var(--accent); }
    input:disabled { opacity: 0.4; }
    fieldset { border: 1px solid var(--border); border-radius: 8px; padding: 12px 14px; margin: 0; }
    legend { font-size: 12px; color: var(--muted); padding: 0 6px; }
    .targets { display: flex; flex-wrap: wrap; gap: 14px; }
    .target { display: flex; align-items: center; gap: 6px; font-size: 14px; color: var(--text); }
    table { width: 100%; border-collapse: collapse; font-size: 13px; }
    th { text-align: left; color: var(--muted); font-weight: 500; padding: 4px 8px; border-bottom: 1px solid var(--border); }
    td { padding: 4px 8px; border-bottom: 1px solid color-mix(in srgb, var(--border) 50%, transparent); }
    td.val input { width: 110px; }
    tr.off td { color: var(--muted); }
    .pcode { color: var(--muted); font-variant-numeric: tabular-nums; }
    .unit { color: var(--muted); }
    .actions { display: flex; gap: 12px; align-items: center; }
    button { border-radius: 8px; padding: 9px 18px; font-size: 14px; font-weight: 600; cursor: pointer; border: none; }
    button.save { background: var(--accent); color: #04121a; }
    button.save:hover:not(:disabled) { filter: brightness(1.08); }
    button.cancel { background: transparent; border: 1px solid var(--border); color: var(--text); }
    button:disabled { opacity: 0.45; cursor: not-allowed; }
    .err { color: var(--err); font-size: 13px; }
    .hint { color: var(--muted); font-size: 12px; }
    .tablewrap { max-height: 320px; overflow: auto; border: 1px solid var(--border); border-radius: 8px; }
  `;

  willUpdate(changed: PropertyValues): void {
    if (changed.has("profile")) {
      const p = this.profile;
      this.name = p?.id ?? "";
      this.selectedUids = [...(p?.uids ?? [])];
      const v: Record<string, string> = {};
      for (const pt of p?.points ?? []) v[pt.aps_code] = String(pt.value);
      this.values = v;
      this.localError = "";
    }
  }

  // effectiveWritable is the set of codes writable on EVERY selected target.
  private effectiveWritable(): Set<string> {
    if (!this.selectedUids.length) return new Set();
    const sets = this.selectedUids.map(
      (uid) => new Set(this.inverters.find((i) => i.uid === uid)?.writable_codes ?? []),
    );
    let acc = sets[0];
    for (const s of sets.slice(1)) acc = new Set([...acc].filter((c) => s.has(c)));
    return acc;
  }

  private label(inv: ProfileInverter): string {
    return this.names[inv.uid] || inv.model || inv.uid;
  }

  private toggleTarget(uid: string, on: boolean) {
    this.selectedUids = on
      ? [...this.selectedUids, uid]
      : this.selectedUids.filter((u) => u !== uid);
  }

  private setValue(code: string, raw: string) {
    this.values = { ...this.values, [code]: raw };
  }

  private save = () => {
    const writable = this.effectiveWritable();
    const points: OverlayPoint[] = this.params
      .filter((p) => writable.has(p.aps_code))
      .map((p) => ({ p, raw: (this.values[p.aps_code] ?? "").trim() }))
      .filter((x) => x.raw !== "" && !Number.isNaN(Number(x.raw)))
      .map((x) => ({ aps_code: x.p.aps_code, value: Number(x.raw) }));

    if (!this.name.trim()) return void (this.localError = "Profile name is required.");
    if (!this.selectedUids.length) return void (this.localError = "Select at least one target inverter.");
    if (!points.length) return void (this.localError = "Set at least one parameter value.");

    this.localError = "";
    const detail: OverlayDraft = { id: this.name.trim(), uids: this.selectedUids, points };
    this.dispatchEvent(new CustomEvent<OverlayDraft>("save", { detail, bubbles: true, composed: true }));
  };

  private cancel = () => {
    this.dispatchEvent(new CustomEvent("cancel", { bubbles: true, composed: true }));
  };

  render() {
    const writable = this.effectiveWritable();
    const haveTargets = this.selectedUids.length > 0;
    return html`
      <div class="grid">
        <label class="field">
          Profile name
          <input
            type="text"
            .value=${this.name}
            ?disabled=${this.editing}
            placeholder="e.g. victron-shift"
            @input=${(e: Event) => (this.name = (e.target as HTMLInputElement).value)}
          />
        </label>

        <fieldset>
          <legend>Target inverters</legend>
          <div class="targets">
            ${this.inverters.length === 0
              ? html`<span class="hint">No inverters seen yet.</span>`
              : this.inverters.map(
                  (inv) => html`<label class="target">
                    <input
                      type="checkbox"
                      .checked=${this.selectedUids.includes(inv.uid)}
                      @change=${(e: Event) => this.toggleTarget(inv.uid, (e.target as HTMLInputElement).checked)}
                    />
                    ${this.label(inv)} <span class="pcode">${inv.model}</span>
                  </label>`,
                )}
          </div>
        </fieldset>

        <fieldset>
          <legend>Parameters</legend>
          ${!haveTargets
            ? html`<span class="hint">Select a target to choose editable parameters.</span>`
            : html`<div class="tablewrap">
                <table>
                  <thead>
                    <tr><th>Parameter</th><th>Code</th><th>Override</th></tr>
                  </thead>
                  <tbody>
                    ${this.params.map((p) => {
                      const on = writable.has(p.aps_code);
                      return html`<tr class=${on ? "" : "off"}>
                        <td>${p.long_name || p.aps_code} <span class="hint">${p.group}</span></td>
                        <td class="pcode">${p.aps_code}</td>
                        <td class="val">
                          <input
                            type="number"
                            step="any"
                            ?disabled=${!on}
                            .value=${this.values[p.aps_code] ?? ""}
                            placeholder=${on ? "—" : "n/a"}
                            @input=${(e: Event) => this.setValue(p.aps_code, (e.target as HTMLInputElement).value)}
                          />
                          <span class="unit">${p.unit}</span>
                        </td>
                      </tr>`;
                    })}
                  </tbody>
                </table>
              </div>`}
          ${haveTargets && this.selectedUids.length > 1
            ? html`<div class="hint">Greyed rows are not writable on every selected target.</div>`
            : nothing}
        </fieldset>

        ${this.localError ? html`<div class="err">⚠ ${this.localError}</div>` : nothing}

        <div class="actions">
          <button class="save" @click=${this.save} ?disabled=${this.busy}>
            ${this.busy ? "Applying…" : "Save & apply"}
          </button>
          <button class="cancel" @click=${this.cancel} ?disabled=${this.busy}>Cancel</button>
          <span class="hint">applies to the selected inverters</span>
        </div>
      </div>
    `;
  }
}

customElements.define("local-site-profile-form", LocalSiteProfileForm);

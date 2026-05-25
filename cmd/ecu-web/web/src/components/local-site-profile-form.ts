import { LitElement, html, css, nothing, type PropertyValues } from "lit";
import type { ParamInfo, ProfileInverter, LocalSiteProfile, OverlayPoint, BaseDefault, ConflictRule } from "../api.ts";
import {
  GROUP_DOCS,
  GROUP_ORDER,
  GROUP_COLLAPSED_BY_DEFAULT,
  paramLabel,
  paramDesc,
  conflicts,
} from "../param-docs.ts";
import { fmtNum } from "../format.ts";
import "./freq-watt-chart.ts";
import "./trip-line.ts";
import type { TripMarker } from "./trip-line.ts";

export interface OverlayDraft {
  id: string;
  uids: string[];
  points: OverlayPoint[];
}

/**
 * <local-site-profile-form> edits one Local Site profile: a name, the target
 * inverters, and parameter overrides grouped by function. Each parameter shows
 * a plain-language label, its base-profile default, and an override input; a
 * value outside the base profile's range is warned, and cross-parameter
 * conflicts (e.g. a slope start past its end) block Save. Presentational: Save
 * emits an OverlayDraft; Cancel emits "cancel".
 */
export class LocalSiteProfileForm extends LitElement {
  static properties = {
    params: { attribute: false },
    inverters: { attribute: false },
    defaults: { attribute: false },
    rules: { attribute: false },
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
  declare defaults: Record<string, BaseDefault>;
  declare rules: ConflictRule[];
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
    this.defaults = {};
    this.rules = [];
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

    details.group { border: 1px solid var(--border); border-radius: 8px; overflow: hidden; }
    details.group + details.group { margin-top: 10px; }
    summary { list-style: none; cursor: pointer; padding: 10px 14px; display: flex; align-items: center; gap: 10px; background: var(--bar-bg); }
    summary::-webkit-details-marker { display: none; }
    summary .gname { font-size: 14px; font-weight: 600; color: var(--text); }
    summary .gcount { font-size: 12px; color: var(--muted); margin-left: auto; }
    .gdesc { padding: 8px 14px 0; font-size: 12px; color: var(--muted); }
    .viz { padding: 10px 14px; border-bottom: 1px solid var(--border); }
    .viz:empty { display: none; }

    table { width: 100%; border-collapse: collapse; font-size: 13px; }
    th { text-align: left; color: var(--muted); font-weight: 500; padding: 6px 14px; border-bottom: 1px solid var(--border); }
    td { padding: 6px 14px; border-bottom: 1px solid color-mix(in srgb, var(--border) 50%, transparent); vertical-align: top; }
    td.val input { width: 110px; }
    tr.off td { color: var(--muted); }
    tr.over td { background: color-mix(in srgb, var(--accent) 9%, transparent); }
    .plabel { color: var(--text); }
    .pdesc { color: var(--muted); font-size: 11px; margin-top: 2px; max-width: 320px; }
    .pcode { color: var(--muted); font-variant-numeric: tabular-nums; font-size: 11px; }
    .def { color: var(--muted); font-variant-numeric: tabular-nums; white-space: nowrap; }
    .unit { color: var(--muted); }
    .otag {
      margin-left: 8px; font-size: 10px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.04em;
      color: var(--accent); border: 1px solid color-mix(in srgb, var(--accent) 55%, transparent); border-radius: 999px; padding: 1px 6px;
    }
    .rotag {
      margin-left: 8px; font-size: 10px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.04em;
      color: var(--muted); border: 1px solid var(--border); border-radius: 999px; padding: 1px 6px;
    }
    .src { font-size: 10px; color: var(--muted); border: 1px solid var(--border); border-radius: 4px; padding: 0 3px; cursor: help; }
    button.clear {
      margin-left: 6px; background: transparent; border: 1px solid var(--border); color: var(--muted);
      border-radius: 6px; width: 24px; height: 24px; font-size: 12px; cursor: pointer; vertical-align: middle;
    }
    button.clear:hover { color: var(--text); border-color: var(--muted); }
    .warn { display: block; margin-top: 4px; font-size: 11px; color: var(--warn); }

    .conflicts { border-radius: 8px; padding: 10px 12px; font-size: 13px; color: var(--err);
      border: 1px solid var(--err); background: color-mix(in srgb, var(--err) 12%, transparent); }
    .conflicts ul { margin: 6px 0 0; padding-left: 18px; }

    .actions { display: flex; align-items: center; gap: 12px; }
    button { border-radius: 8px; padding: 9px 18px; font-size: 14px; font-weight: 600; cursor: pointer; border: none; }
    button.save { background: var(--accent); color: #04121a; }
    button.save:hover:not(:disabled) { filter: brightness(1.08); }
    button.cancel { background: transparent; border: 1px solid var(--border); color: var(--text); }
    button:disabled { opacity: 0.45; cursor: not-allowed; }
    .err { color: var(--err); font-size: 13px; }
    .hint { color: var(--muted); font-size: 12px; }
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

  private effectiveWritable(): Set<string> {
    if (!this.selectedUids.length) return new Set();
    const sets = this.selectedUids.map(
      (uid) => new Set(this.inverters.find((i) => i.uid === uid)?.writable_codes ?? []),
    );
    let acc = sets[0];
    for (const s of sets.slice(1)) acc = new Set([...acc].filter((c) => s.has(c)));
    return acc;
  }

  // targetDefault is the value a parameter defaults to for the selected
  // target(s): the base grid profile value if it defines one, else the
  // inverter's own current value — but only when every selected target agrees
  // (otherwise it "varies" and there is no single default).
  private targetDefault(code: string): { value: number; source: "base" | "inverter" } | undefined {
    const b = this.defaults[code];
    if (b) return { value: b.value, source: "base" };
    if (!this.selectedUids.length) return undefined;
    let v: number | undefined;
    for (const uid of this.selectedUids) {
      const cv = this.inverters.find((i) => i.uid === uid)?.current?.[code];
      if (cv === undefined) return undefined;
      if (v === undefined) v = cv;
      else if (Math.abs(cv - v) > 1e-6) return undefined; // varies across targets
    }
    return v === undefined ? undefined : { value: v, source: "inverter" };
  }

  // effectiveValue is the override if entered (and numeric), else the default.
  private effectiveValue(code: string): number | undefined {
    const raw = (this.values[code] ?? "").trim();
    if (raw !== "" && !Number.isNaN(Number(raw))) return Number(raw);
    return this.targetDefault(code)?.value;
  }

  // isOverride is true when the entered value differs from the default — only
  // these are saved (a value equal to the default is a no-op).
  private isOverride(code: string): boolean {
    const raw = (this.values[code] ?? "").trim();
    if (raw === "" || Number.isNaN(Number(raw))) return false;
    const td = this.targetDefault(code);
    return !td || Number(raw) !== td.value;
  }

  // prefill seeds an empty, editable field with its default so the operator
  // adjusts from the real starting value rather than from blank.
  private prefill(code: string) {
    if ((this.values[code] ?? "").trim() !== "") return;
    const td = this.targetDefault(code);
    if (td) this.setValue(code, fmtNum(td.value));
  }

  private outOfRange(code: string): boolean {
    const raw = (this.values[code] ?? "").trim();
    if (raw === "" || Number.isNaN(Number(raw))) return false;
    const d = this.defaults[code];
    if (!d) return false;
    const v = Number(raw);
    return (d.min !== undefined && v < d.min) || (d.max !== undefined && v > d.max);
  }

  private label(inv: ProfileInverter): string {
    return this.names[inv.uid] || inv.model || inv.uid;
  }

  private toggleTarget(uid: string, on: boolean) {
    this.selectedUids = on ? [...this.selectedUids, uid] : this.selectedUids.filter((u) => u !== uid);
  }

  private setValue(code: string, raw: string) {
    this.values = { ...this.values, [code]: raw };
  }

  // groups returns [groupName, params[]] in preferred order, only for groups present.
  private groups(): [string, ParamInfo[]][] {
    const by: Record<string, ParamInfo[]> = {};
    for (const p of this.params) (by[p.group] ??= []).push(p);
    const order = [...GROUP_ORDER, ...Object.keys(by).filter((g) => !GROUP_ORDER.includes(g))];
    return order.filter((g) => by[g]?.length).map((g) => [g, by[g]]);
  }

  private save = () => {
    const writable = this.effectiveWritable();
    const points: OverlayPoint[] = this.params
      .filter((p) => writable.has(p.aps_code) && this.isOverride(p.aps_code))
      .map((p) => ({ aps_code: p.aps_code, value: Number(this.values[p.aps_code]) }));

    if (!this.name.trim()) return void (this.localError = "Profile name is required.");
    if (!this.selectedUids.length) return void (this.localError = "Select at least one target inverter.");
    if (!points.length) return void (this.localError = "Change at least one parameter from its default.");
    if (conflicts(this.rules, (c) => this.effectiveValue(c)).length) return void (this.localError = "Resolve the conflicts before saving.");

    this.localError = "";
    const detail: OverlayDraft = { id: this.name.trim(), uids: this.selectedUids, points };
    this.dispatchEvent(new CustomEvent<OverlayDraft>("save", { detail, bubbles: true, composed: true }));
  };

  private cancel = () => this.dispatchEvent(new CustomEvent("cancel", { bubbles: true, composed: true }));

  // markers builds trip-line markers from the catalog: every param in `group`
  // with the given unit and an under/over polarity that has an effective value.
  // Data-driven — a new trip code charts automatically, no hardcoded list.
  private markers(group: string, unit: string): TripMarker[] {
    const out: TripMarker[] = [];
    for (const p of this.params) {
      if (p.group !== group || p.unit !== unit) continue;
      if (p.polarity !== "under" && p.polarity !== "over") continue;
      const v = this.effectiveValue(p.aps_code);
      if (v !== undefined) out.push({ value: v, label: p.aps_code, kind: p.polarity });
    }
    return out;
  }

  // vizFor renders the curve/number-line visualization for a group, driven by
  // the effective values (override if set, else default).
  private vizFor(group: string) {
    if (group === "DERFreqDroop") {
      return html`<freq-watt-chart
        .deadband=${this.effectiveValue("CA")}
        .slope=${this.effectiveValue("DD")}
        .trip=${this.effectiveValue("AF")}
        .nominal=${50}
      ></freq-watt-chart>`;
    }
    if (group === "CrvSet") {
      const ms = this.markers(group, "Hz");
      return ms.length ? html`<trip-line unit="Hz" .nominal=${50} .markers=${ms}></trip-line>` : nothing;
    }
    if (group === "MustTrip") {
      const v = this.markers(group, "V");
      const f = this.markers(group, "Hz");
      return html`
        ${v.length ? html`<trip-line unit="V" .nominal=${230} .markers=${v}></trip-line>` : nothing}
        ${f.length ? html`<trip-line unit="Hz" .nominal=${50} .markers=${f}></trip-line>` : nothing}
      `;
    }
    return nothing;
  }

  private renderRow(p: ParamInfo, writable: Set<string>) {
    const on = writable.has(p.aps_code);
    const td = this.targetDefault(p.aps_code);
    const def = this.defaults[p.aps_code]; // base profile entry (carries range)
    const raw = (this.values[p.aps_code] ?? "").trim();
    const overridden = this.isOverride(p.aps_code);
    const oor = on && this.outOfRange(p.aps_code);
    const inputVal = on ? (this.values[p.aps_code] ?? "") : td ? fmtNum(td.value) : "";
    return html`<tr class="${on ? "" : "off"} ${overridden ? "over" : ""}">
      <td>
        <div class="plabel">
          ${paramLabel(p.aps_code, p.long_name)}
          ${overridden ? html`<span class="otag">overridden</span>` : nothing}
          ${!on && td ? html`<span class="rotag">read-only</span>` : nothing}
        </div>
        <div class="pdesc">${paramDesc(p.aps_code)}</div>
      </td>
      <td class="pcode">${p.aps_code}</td>
      <td class="def">
        ${td ? html`${fmtNum(td.value)} ${p.unit}${td.source === "inverter" ? html` <span class="src" title="from the inverter's current value">inv</span>` : nothing}` : "—"}
      </td>
      <td class="val">
        <input
          type="number" step="any" ?disabled=${!on}
          .value=${inputVal}
          placeholder=${td ? fmtNum(td.value) : on ? "—" : "n/a"}
          @focus=${() => this.prefill(p.aps_code)}
          @input=${(e: Event) => this.setValue(p.aps_code, (e.target as HTMLInputElement).value)}
        />
        <span class="unit">${p.unit}</span>
        ${on && raw !== "" ? html`<button class="clear" title="Clear override" @click=${() => this.setValue(p.aps_code, "")}>↺</button>` : nothing}
        ${oor
          ? html`<span class="warn">⚠ outside base range${def?.min !== undefined ? ` (${fmtNum(def.min)}–${fmtNum(def.max!)} ${p.unit})` : ""}</span>`
          : nothing}
      </td>
    </tr>`;
  }

  render() {
    const writable = this.effectiveWritable();
    const haveTargets = this.selectedUids.length > 0;
    const conf = haveTargets ? conflicts(this.rules, (c) => this.effectiveValue(c)) : [];

    return html`
      <div class="grid">
        <label class="field">
          Profile name
          <input type="text" .value=${this.name} ?disabled=${this.editing} placeholder="e.g. victron-shift"
            @input=${(e: Event) => (this.name = (e.target as HTMLInputElement).value)} />
        </label>

        <fieldset>
          <legend>Target inverters</legend>
          <div class="targets">
            ${this.inverters.length === 0
              ? html`<span class="hint">No inverters seen yet.</span>`
              : this.inverters.map(
                  (inv) => html`<label class="target">
                    <input type="checkbox" .checked=${this.selectedUids.includes(inv.uid)}
                      @change=${(e: Event) => this.toggleTarget(inv.uid, (e.target as HTMLInputElement).checked)} />
                    ${this.label(inv)} <span class="pcode">${inv.model}</span>
                  </label>`,
                )}
          </div>
        </fieldset>

        ${!haveTargets
          ? html`<span class="hint">Select a target to choose editable parameters.</span>`
          : html`
              ${conf.length
                ? html`<div class="conflicts">⚠ Conflicting settings — resolve to save:
                    <ul>${conf.map((m) => html`<li>${m}</li>`)}</ul>
                  </div>`
                : nothing}

              ${this.groups().map(([g, ps]) => {
                const d = GROUP_DOCS[g];
                return html`<details class="group" ?open=${!GROUP_COLLAPSED_BY_DEFAULT.has(g)}>
                  <summary>
                    <span class="gname">${d?.label ?? g}</span>
                    <span class="gcount">${ps.length} setting${ps.length === 1 ? "" : "s"}</span>
                  </summary>
                  ${d?.tip ? html`<div class="gdesc">${d.tip}</div>` : nothing}
                  <div class="viz">${this.vizFor(g)}</div>
                  <table>
                    <thead><tr><th>Setting</th><th>Code</th><th>Default</th><th>Override</th></tr></thead>
                    <tbody>${ps.map((p) => this.renderRow(p, writable))}</tbody>
                  </table>
                </details>`;
              })}

              ${this.selectedUids.length > 1
                ? html`<div class="hint">Greyed rows are not writable on every selected target.</div>`
                : nothing}
            `}

        ${this.localError ? html`<div class="err">⚠ ${this.localError}</div>` : nothing}

        <div class="actions">
          <button class="save" @click=${this.save} ?disabled=${this.busy || conf.length > 0}>
            ${this.busy ? "Applying…" : "Save & apply"}
          </button>
          <button class="cancel" @click=${this.cancel} ?disabled=${this.busy}>Cancel</button>
          <span class="hint">${conf.length ? "resolve conflicts to save" : "applies to the selected inverters"}</span>
        </div>
      </div>
    `;
  }
}

customElements.define("local-site-profile-form", LocalSiteProfileForm);

import { LitElement, html, css, nothing } from "lit";
import type { PairingStatus } from "../api.ts";
import { pairingActive } from "../api.ts";

const STAGES = ["scan", "bind", "migrate", "configure", "rekey"] as const;

const STAGE_LABEL: Record<string, string> = {
  scan: "Scan",
  bind: "Bind",
  migrate: "Migrate",
  configure: "Configure",
  rekey: "Re-key",
  done: "Done",
  aborted: "Aborted",
  error: "Error",
};

/**
 * <pairing-progress-drawer> — a slide-over panel showing the live state of a
 * pairing op. It is presentational: it renders the PairingStatus passed in
 * (the owner polls GET /api/pairing/status ~1s while active) and emits
 *   "abort" → operator requested a safe abort
 *   "close" → dismiss the drawer (only enabled once the op is terminal)
 */
export class PairingProgressDrawer extends LitElement {
  static properties = {
    open: { attribute: false },
    status: { attribute: false },
    aborting: { attribute: false },
  };

  declare open: boolean;
  declare status: PairingStatus | null;
  declare aborting: boolean;

  constructor() {
    super();
    this.open = false;
    this.status = null;
    this.aborting = false;
  }

  static styles = css`
    :host { display: block; }
    .scrim {
      position: fixed; inset: 0; background: rgba(0, 0, 0, 0.45);
      z-index: 40; display: none;
    }
    .scrim.open { display: block; }
    .drawer {
      position: fixed; top: 0; right: 0; bottom: 0; width: 420px; max-width: 92vw;
      background: var(--bg, #0c1116); border-left: 1px solid var(--border);
      box-shadow: -8px 0 30px rgba(0, 0, 0, 0.4);
      transform: translateX(100%); transition: transform 0.18s ease;
      z-index: 41; display: flex; flex-direction: column;
      box-sizing: border-box;
    }
    .scrim.open .drawer { transform: translateX(0); }
    header {
      display: flex; align-items: center; justify-content: space-between;
      padding: 16px 18px; border-bottom: 1px solid var(--border);
    }
    header h2 { margin: 0; font-size: 16px; color: var(--text); }
    button.x {
      background: transparent; border: 1px solid var(--border); color: var(--muted);
      border-radius: 8px; padding: 4px 10px; font-size: 15px; line-height: 1; cursor: pointer;
    }
    button.x:disabled { opacity: 0.4; cursor: not-allowed; }
    .body { padding: 16px 18px; overflow-y: auto; display: grid; gap: 16px; }
    .stages { display: flex; gap: 6px; flex-wrap: wrap; }
    .stage {
      font-size: 11px; padding: 4px 9px; border-radius: 999px;
      border: 1px solid var(--border); color: var(--muted);
    }
    .stage.active { background: var(--accent); color: #04121a; border-color: var(--accent); font-weight: 600; }
    .stage.done { color: var(--ok); border-color: color-mix(in srgb, var(--ok) 50%, transparent); }
    .bar { height: 8px; border-radius: 999px; background: var(--bar-bg); overflow: hidden; }
    .bar > i { display: block; height: 100%; background: var(--accent); transition: width 0.2s ease; }
    .meta { font-size: 13px; color: var(--text); display: grid; gap: 4px; }
    .meta .muted { color: var(--muted); }
    .sweep { font-size: 12px; color: var(--muted); }
    .err { color: var(--err); font-size: 13px; }
    .ok { color: var(--ok); font-size: 13px; }
    table { width: 100%; border-collapse: collapse; font-size: 12px; }
    th, td { text-align: left; padding: 6px 8px; border-bottom: 1px solid var(--border); }
    th { color: var(--muted); text-transform: uppercase; font-size: 10px; letter-spacing: 0.04em; }
    td.mono { font-family: var(--mono); }
    .actions { padding: 14px 18px; border-top: 1px solid var(--border); display: flex; gap: 12px; }
    button.abort {
      background: transparent; border: 1px solid var(--err); color: var(--err);
      border-radius: 8px; padding: 8px 16px; font-size: 13px; font-weight: 600; cursor: pointer;
    }
    button.abort:hover:not(:disabled) { background: color-mix(in srgb, var(--err) 14%, transparent); }
    button.abort:disabled { opacity: 0.4; cursor: not-allowed; }
    .empty { color: var(--muted); font-size: 13px; }
    @media (max-width: 480px) { .drawer { width: 100vw; } }
  `;

  private abort = () => {
    this.dispatchEvent(new CustomEvent("abort", { bubbles: true, composed: true }));
  };

  private close = () => {
    this.dispatchEvent(new CustomEvent("close", { bubbles: true, composed: true }));
  };

  private stageClass(stage: string, current: string): string {
    if (stage === current) return "stage active";
    const si = STAGES.indexOf(stage as (typeof STAGES)[number]);
    const ci = STAGES.indexOf(current as (typeof STAGES)[number]);
    if (ci > si && si >= 0) return "stage done";
    return "stage";
  }

  render() {
    const st = this.status;
    const active = pairingActive(st);
    const stage = st?.stage ?? "";
    const total = st?.total ?? 0;
    const done = st?.done ?? 0;
    const pct = total > 0 ? Math.min(100, Math.round((done / total) * 100)) : active ? 0 : 0;

    return html`
      <div class="scrim ${this.open ? "open" : ""}" @click=${(e: Event) => { if (e.target === e.currentTarget && !active) this.close(); }}>
        <aside class="drawer" role="dialog" aria-label="Pairing progress" aria-modal="true">
          <header>
            <h2>${st?.op ? `Pairing: ${st.op}` : "Pairing"}</h2>
            <button class="x" aria-label="Close" ?disabled=${active} @click=${this.close}>✕</button>
          </header>
          <div class="body">
            ${!st || !st.op
              ? html`<p class="empty">No pairing operation running.</p>`
              : html`
                  <div class="stages">
                    ${STAGES.map((sName) => html`<span class=${this.stageClass(sName, stage)}>${STAGE_LABEL[sName]}</span>`)}
                  </div>

                  ${total > 0
                    ? html`<div class="bar"><i style="width:${pct}%"></i></div>
                        <div class="meta"><span class="muted">${done} / ${total} inverters</span></div>`
                    : nothing}

                  <div class="meta">
                    <div><span class="muted">Stage:</span> ${STAGE_LABEL[stage] ?? stage ?? "—"}</div>
                    ${st.current_serial ? html`<div><span class="muted">Current:</span> ${st.current_serial}</div>` : nothing}
                    ${st.substep ? html`<div><span class="muted">Step:</span> ${st.substep}</div>` : nothing}
                    ${st.message ? html`<div class="muted">${st.message}</div>` : nothing}
                  </div>

                  ${st.sweep
                    ? html`<div class="sweep">Channel ${st.sweep.chan} (sweep ${st.sweep.chan_lo}–${st.sweep.chan_hi}) — telemetry paused</div>`
                    : nothing}

                  ${st.error ? html`<div class="err">Error: ${st.error}</div>` : nothing}
                  ${stage === "done" ? html`<div class="ok">Completed.</div>` : nothing}
                  ${stage === "aborted" ? html`<div class="muted">Aborted.</div>` : nothing}

                  ${st.per_inverter && st.per_inverter.length > 0
                    ? html`<table>
                        <thead><tr><th>Serial</th><th>Addr</th><th>State</th><th>Link</th></tr></thead>
                        <tbody>
                          ${st.per_inverter.map(
                            (p) => html`<tr>
                              <td class="mono">${p.serial}</td>
                              <td class="mono">${p.short_addr ? p.short_addr.toString(16) : "—"}</td>
                              <td>${p.state}</td>
                              <td>${p.encrypted === true ? "🔒" : p.encrypted === false ? "⚠" : "—"}</td>
                            </tr>`,
                          )}
                        </tbody>
                      </table>`
                    : nothing}
                `}
          </div>
          <div class="actions">
            <button class="abort" ?disabled=${!active || this.aborting} @click=${this.abort}>
              ${this.aborting ? "Aborting…" : "Safe abort"}
            </button>
          </div>
        </aside>
      </div>
    `;
  }
}

customElements.define("pairing-progress-drawer", PairingProgressDrawer);

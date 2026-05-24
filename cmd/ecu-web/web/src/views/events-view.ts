import { LitElement, html, css, nothing } from "lit";
import { api, type Event } from "../api.ts";
import "../components/events-table.ts";

/**
 * <events-view> fetches the inv-driver event log and renders it, with a
 * manual refresh and a periodic auto-refresh (events are not streamed).
 */
export class EventsView extends LitElement {
  static properties = {
    events: { state: true },
    error: { state: true },
    loading: { state: true },
  };

  declare events: Event[];
  declare error: string;
  declare loading: boolean;

  private timer: ReturnType<typeof setInterval> | null = null;

  constructor() {
    super();
    this.events = [];
    this.error = "";
    this.loading = false;
  }

  static styles = css`
    :host { display: block; }
    .bar { display: flex; align-items: center; justify-content: space-between; margin-bottom: 14px; }
    .count { color: var(--muted); font-size: 13px; }
    button {
      background: transparent;
      border: 1px solid var(--border);
      color: var(--muted);
      border-radius: 8px;
      padding: 6px 12px;
      font-size: 13px;
      cursor: pointer;
    }
    button:hover { color: var(--text); border-color: var(--muted); }
    .err { color: var(--err); font-size: 13px; margin-bottom: 12px; }
    .panel { background: var(--surface); border: 1px solid var(--border); border-radius: 10px; overflow: hidden; }
  `;

  connectedCallback(): void {
    super.connectedCallback();
    void this.load();
    this.timer = setInterval(() => void this.load(), 15000);
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    if (this.timer) clearInterval(this.timer);
    this.timer = null;
  }

  private async load() {
    this.loading = true;
    try {
      const res = await api.events({ limit: 200 });
      this.events = res.events ?? [];
      this.error = res.error ?? "";
    } catch (e) {
      this.error = (e as Error).message;
    } finally {
      this.loading = false;
    }
  }

  render() {
    return html`
      <div class="bar">
        <span class="count">${this.events.length} event(s)${this.loading ? " · refreshing…" : ""}</span>
        <button @click=${() => void this.load()}>Refresh</button>
      </div>
      ${this.error ? html`<div class="err">⚠ ${this.error}</div>` : nothing}
      <div class="panel"><events-table .events=${this.events}></events-table></div>
    `;
  }
}

customElements.define("events-view", EventsView);

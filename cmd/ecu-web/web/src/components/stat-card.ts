import { LitElement, html, css } from "lit";

/** <stat-card label="Today" value="4.2 kWh"> — a labelled metric tile. */
export class StatCard extends LitElement {
  static properties = {
    label: { type: String },
    value: { type: String },
    sub: { type: String },
  };

  declare label: string;
  declare value: string;
  declare sub: string;

  constructor() {
    super();
    this.label = "";
    this.value = "";
    this.sub = "";
  }

  static styles = css`
    :host {
      display: block;
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: 10px;
      padding: 14px 16px;
    }
    .label {
      color: var(--muted);
      font-size: 12px;
      text-transform: uppercase;
      letter-spacing: 0.04em;
    }
    .value {
      font-size: 22px;
      font-weight: 700;
      color: var(--text);
      margin-top: 4px;
    }
    .sub {
      font-size: 12px;
      color: var(--muted);
      margin-top: 2px;
    }
  `;

  render() {
    return html`
      <div class="label">${this.label}</div>
      <div class="value">${this.value}</div>
      ${this.sub ? html`<div class="sub">${this.sub}</div>` : ""}
    `;
  }
}

customElements.define("stat-card", StatCard);

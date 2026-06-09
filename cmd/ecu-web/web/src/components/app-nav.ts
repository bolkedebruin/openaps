import { LitElement, html, css, nothing } from "lit";

export interface NavItem {
  id: string;
  label: string;
  icon: string;
}

/**
 * <app-nav .items .route .open> renders the sidebar navigation. On desktop it
 * is the static left column; below 720px it is a slide-in drawer with a scrim.
 * Items are hash links (#/<id>). Clicking an item or the scrim dispatches a
 * composed "close" event so the shell can close the mobile drawer.
 */
export class AppNav extends LitElement {
  static properties = {
    items: { attribute: false },
    route: { type: String },
    open: { type: Boolean },
    version: { type: String },
    commit: { type: String },
  };

  declare items: NavItem[];
  declare route: string;
  declare open: boolean;
  declare version: string;
  declare commit: string;

  constructor() {
    super();
    this.items = [];
    this.route = "dashboard";
    this.open = false;
    this.version = "";
    this.commit = "";
  }

  private close = () => {
    this.dispatchEvent(new CustomEvent("close", { bubbles: true, composed: true }));
  };

  static styles = css`
    :host { display: block; height: 100%; }
    nav {
      height: 100%;
      box-sizing: border-box;
      background: var(--surface);
      border-right: 1px solid var(--border);
      padding: 20px 12px;
      display: flex;
      flex-direction: column;
    }
    .foot {
      margin-top: auto;
      padding: 14px 12px 2px;
      font-size: 11px;
      line-height: 1.4;
      color: var(--muted);
      font-family: var(--mono);
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }
    .brand {
      font-weight: 800;
      letter-spacing: 0.06em;
      color: var(--accent);
      padding: 0 12px 20px;
      font-size: 16px;
    }
    a.item {
      display: flex;
      align-items: center;
      gap: 10px;
      padding: 10px 12px;
      border-radius: 8px;
      color: var(--muted);
      text-decoration: none;
      font-size: 14px;
      margin-bottom: 2px;
    }
    a.item:hover { background: var(--bar-bg); color: var(--text); }
    a.item.active { background: color-mix(in srgb, var(--accent) 18%, transparent); color: var(--accent); }
    .ic { width: 18px; text-align: center; opacity: 0.8; }
    .scrim { display: none; }
    @media (max-width: 720px) {
      :host { height: auto; }
      nav {
        position: fixed;
        top: 0;
        left: 0;
        bottom: 0;
        width: 240px;
        z-index: 30;
        transform: translateX(-100%);
        transition: transform 0.2s ease;
        overflow-y: auto;
      }
      nav.open { transform: translateX(0); box-shadow: 4px 0 32px rgba(0, 0, 0, 0.5); }
      .scrim { display: block; position: fixed; inset: 0; background: rgba(0, 0, 0, 0.5); z-index: 20; }
    }
  `;

  render() {
    return html`
      <nav class=${this.open ? "open" : ""}>
        <div class="brand">ECU CONSOLE</div>
        ${this.items.map(
          (n) => html`<a
            class="item ${this.route === n.id ? "active" : ""}"
            href="#/${n.id}"
            @click=${this.close}
          ><span class="ic">${n.icon}</span>${n.label}</a>`,
        )}
        ${this.version || this.commit
          ? html`<div class="foot" title="OpenAPS ecu-web ${this.version}${this.commit ? ` (${this.commit})` : ""}">
              ${this.version || "—"}${this.commit ? html` · ${this.commit}` : nothing}
            </div>`
          : nothing}
      </nav>
      ${this.open ? html`<div class="scrim" @click=${this.close}></div>` : nothing}
    `;
  }
}

customElements.define("app-nav", AppNav);

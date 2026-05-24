import { LitElement, html, css } from "lit";
import { api } from "../api.ts";

/**
 * <login-view .configured=${bool}> shows the password prompt — first-run
 * "set a password" when unconfigured, otherwise normal login. Dispatches
 * a bubbling "authed" event on success.
 */
export class LoginView extends LitElement {
  static properties = {
    configured: { type: Boolean },
    error: { state: true },
    busy: { state: true },
  };

  declare configured: boolean;
  declare error: string;
  declare busy: boolean;

  constructor() {
    super();
    this.configured = true;
    this.error = "";
    this.busy = false;
  }

  static styles = css`
    :host {
      display: grid;
      place-items: center;
      min-height: 100vh;
    }
    .box {
      width: 320px;
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: 12px;
      padding: 28px;
    }
    h1 { font-size: 20px; margin: 0 0 4px; color: var(--text); }
    p { color: var(--muted); font-size: 13px; margin: 0 0 18px; }
    label { display: block; font-size: 12px; color: var(--muted); margin-bottom: 6px; }
    input {
      width: 100%;
      box-sizing: border-box;
      padding: 10px 12px;
      background: var(--bar-bg);
      border: 1px solid var(--border);
      border-radius: 8px;
      color: var(--text);
      font-size: 14px;
    }
    button {
      width: 100%;
      margin-top: 16px;
      padding: 10px;
      background: var(--accent);
      color: #04222b;
      border: none;
      border-radius: 8px;
      font-weight: 700;
      cursor: pointer;
    }
    button:disabled { opacity: 0.6; cursor: default; }
    .err { color: var(--err); font-size: 13px; margin-top: 12px; min-height: 16px; }
    .brand { color: var(--accent); font-weight: 700; letter-spacing: 0.04em; }
  `;

  private async submit(e: Event) {
    e.preventDefault();
    const input = this.renderRoot.querySelector("input") as HTMLInputElement;
    const pw = input?.value ?? "";
    this.busy = true;
    this.error = "";
    try {
      if (this.configured) await api.login(pw);
      else await api.setup(pw);
      this.dispatchEvent(new CustomEvent("authed", { bubbles: true, composed: true }));
    } catch (err) {
      this.error = (err as Error).message || "failed";
    } finally {
      this.busy = false;
    }
  }

  render() {
    return html`
      <form class="box" @submit=${this.submit}>
        <h1><span class="brand">ECU</span> Console</h1>
        <p>
          ${this.configured
            ? "Enter the operator password."
            : "First run — choose an operator password (min 8 characters)."}
        </p>
        <label for="pw">Password</label>
        <input
          id="pw"
          type="password"
          autocomplete=${this.configured ? "current-password" : "new-password"}
          ?disabled=${this.busy}
        />
        <button type="submit" ?disabled=${this.busy}>
          ${this.busy ? "…" : this.configured ? "Sign in" : "Set password"}
        </button>
        <div class="err">${this.error}</div>
      </form>
    `;
  }
}

customElements.define("login-view", LoginView);

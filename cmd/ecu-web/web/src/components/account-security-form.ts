import { LitElement, html, css, nothing } from "lit";
import { api } from "../api.ts";

/**
 * <account-security-form> lets a logged-in operator change their password
 * (current password required) and regenerate the one-time recovery code. The
 * new recovery code is shown once, inline, after regeneration.
 */
export class AccountSecurityForm extends LitElement {
  static properties = {
    pwError: { state: true },
    pwNotice: { state: true },
    pwBusy: { state: true },
    recError: { state: true },
    recBusy: { state: true },
    newCode: { state: true },
  };

  declare pwError: string;
  declare pwNotice: string;
  declare pwBusy: boolean;
  declare recError: string;
  declare recBusy: boolean;
  declare newCode: string;

  constructor() {
    super();
    this.pwError = "";
    this.pwNotice = "";
    this.pwBusy = false;
    this.recError = "";
    this.recBusy = false;
    this.newCode = "";
  }

  static styles = css`
    :host { display: block; }
    h3 { font-size: 13px; margin: 0 0 12px; color: var(--text); }
    .section + .section { margin-top: 24px; padding-top: 20px; border-top: 1px solid var(--border); }
    label { display: block; font-size: 12px; color: var(--muted); margin: 12px 0 6px; }
    label:first-of-type { margin-top: 0; }
    input {
      width: 100%;
      box-sizing: border-box;
      padding: 9px 12px;
      background: var(--bar-bg);
      border: 1px solid var(--border);
      border-radius: 8px;
      color: var(--text);
      font-size: 14px;
    }
    button {
      margin-top: 14px;
      padding: 9px 16px;
      border-radius: 8px;
      font-size: 13px;
      font-weight: 600;
      cursor: pointer;
    }
    button.primary { background: var(--accent); color: #04222b; border: none; }
    button.secondary { background: transparent; color: var(--muted); border: 1px solid var(--border); }
    button.secondary:hover { color: var(--text); border-color: var(--muted); }
    button:disabled { opacity: 0.6; cursor: default; }
    .muted { color: var(--muted); font-size: 13px; margin: 0 0 6px; }
    .banner { border-radius: 8px; padding: 8px 12px; font-size: 13px; margin-top: 12px; }
    .banner.ok { color: var(--ok); border: 1px solid var(--ok); background: color-mix(in srgb, var(--ok) 12%, transparent); }
    .banner.err { color: var(--err); border: 1px solid var(--err); background: color-mix(in srgb, var(--err) 12%, transparent); }
    .code {
      font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
      font-size: 16px;
      letter-spacing: 0.06em;
      text-align: center;
      background: var(--bar-bg);
      border: 1px solid var(--accent);
      border-radius: 8px;
      padding: 12px;
      color: var(--text);
      user-select: all;
      margin-top: 12px;
    }
  `;

  private val(id: string): string {
    const el = this.renderRoot.querySelector(`#${id}`) as HTMLInputElement | null;
    return el?.value ?? "";
  }

  private clear(id: string) {
    const el = this.renderRoot.querySelector(`#${id}`) as HTMLInputElement | null;
    if (el) el.value = "";
  }

  private changePassword = async (e: Event) => {
    e.preventDefault();
    if (this.pwBusy) return;
    this.pwError = "";
    this.pwNotice = "";
    if (this.val("new") !== this.val("new2")) {
      this.pwError = "New passwords do not match.";
      return;
    }
    this.pwBusy = true;
    try {
      await api.changePassword(this.val("cur"), this.val("new"));
      this.pwNotice = "Password changed.";
      this.clear("cur");
      this.clear("new");
      this.clear("new2");
    } catch (err) {
      this.pwError = (err as Error).message || "failed";
    } finally {
      this.pwBusy = false;
    }
  };

  private regenerate = async () => {
    if (this.recBusy) return;
    this.recError = "";
    this.newCode = "";
    this.recBusy = true;
    try {
      const r = await api.regenerateRecovery();
      this.newCode = r.recovery_code;
    } catch (err) {
      this.recError = (err as Error).message || "failed";
    } finally {
      this.recBusy = false;
    }
  };

  render() {
    return html`
      <div class="section">
        <h3>Change password</h3>
        <form @submit=${this.changePassword}>
          <label for="cur">Current password</label>
          <input id="cur" type="password" autocomplete="current-password" ?disabled=${this.pwBusy} />
          <label for="new">New password</label>
          <input id="new" type="password" autocomplete="new-password" ?disabled=${this.pwBusy} />
          <label for="new2">Confirm new password</label>
          <input id="new2" type="password" autocomplete="new-password" ?disabled=${this.pwBusy} />
          <button class="primary" type="submit" ?disabled=${this.pwBusy}>
            ${this.pwBusy ? "…" : "Change password"}
          </button>
          ${this.pwNotice ? html`<div class="banner ok">${this.pwNotice}</div>` : nothing}
          ${this.pwError ? html`<div class="banner err">⚠ ${this.pwError}</div>` : nothing}
        </form>
      </div>

      <div class="section">
        <h3>Recovery code</h3>
        <p class="muted">
          The recovery code resets your password without console access. Generating a
          new one invalidates the previous code. It's shown only once.
        </p>
        <button class="secondary" type="button" @click=${this.regenerate} ?disabled=${this.recBusy}>
          ${this.recBusy ? "…" : "Generate new recovery code"}
        </button>
        ${this.newCode
          ? html`<div class="code">${this.newCode}</div>
              <p class="muted" style="margin-top:8px">Write this down now — it won't be shown again.</p>`
          : nothing}
        ${this.recError ? html`<div class="banner err">⚠ ${this.recError}</div>` : nothing}
      </div>
    `;
  }
}

customElements.define("account-security-form", AccountSecurityForm);

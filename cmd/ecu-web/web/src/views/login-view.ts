import { LitElement, html, css, nothing } from "lit";
import { api } from "../api.ts";

/**
 * <login-view .configured=${bool}> is the unauthenticated gate. It shows
 * first-run setup when unconfigured, normal login when configured, and a
 * recovery-code reset when the operator forgot their password. Setup and
 * recovery hand back a one-time recovery code, shown on a "save this" screen
 * before entering the app. Dispatches a bubbling "authed" event on success.
 */
export class LoginView extends LitElement {
  static properties = {
    configured: { type: Boolean },
    error: { state: true },
    busy: { state: true },
    recoverMode: { state: true },
    savedCode: { state: true },
    copied: { state: true },
  };

  declare configured: boolean;
  declare error: string;
  declare busy: boolean;
  declare recoverMode: boolean;
  declare savedCode: string; // non-empty => show the "save this code" screen
  declare copied: boolean;

  constructor() {
    super();
    this.configured = true;
    this.error = "";
    this.busy = false;
    this.recoverMode = false;
    this.savedCode = "";
    this.copied = false;
  }

  static styles = css`
    :host {
      display: grid;
      place-items: center;
      min-height: 100vh;
    }
    .box {
      width: 340px;
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: 12px;
      padding: 28px;
    }
    h1 { font-size: 20px; margin: 0 0 4px; color: var(--text); }
    p { color: var(--muted); font-size: 13px; margin: 0 0 18px; }
    label { display: block; font-size: 12px; color: var(--muted); margin: 14px 0 6px; }
    label:first-of-type { margin-top: 0; }
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
    button.primary {
      width: 100%;
      margin-top: 18px;
      padding: 10px;
      background: var(--accent);
      color: #04222b;
      border: none;
      border-radius: 8px;
      font-weight: 700;
      cursor: pointer;
    }
    button.primary:disabled { opacity: 0.6; cursor: default; }
    .err { color: var(--err); font-size: 13px; margin-top: 12px; min-height: 16px; }
    .brand { color: var(--accent); font-weight: 700; letter-spacing: 0.04em; }
    .link {
      display: inline-block;
      margin-top: 16px;
      background: none;
      border: none;
      padding: 0;
      color: var(--muted);
      font-size: 13px;
      text-decoration: underline;
      cursor: pointer;
    }
    .link:hover { color: var(--text); }
    .code {
      font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
      font-size: 18px;
      letter-spacing: 0.06em;
      text-align: center;
      background: var(--bar-bg);
      border: 1px solid var(--border);
      border-radius: 8px;
      padding: 14px;
      color: var(--text);
      user-select: all;
      word-break: break-all;
    }
    .warn { color: var(--text); font-size: 13px; margin: 0 0 14px; }
    .copy {
      width: 100%;
      margin-top: 10px;
      padding: 8px;
      background: transparent;
      border: 1px solid var(--border);
      color: var(--muted);
      border-radius: 8px;
      font-size: 13px;
      cursor: pointer;
    }
    .copy:hover { color: var(--text); border-color: var(--muted); }
  `;

  firstUpdated() {
    this.focusFirst();
  }

  updated(changed: Map<string, unknown>) {
    // Re-focus the first field when switching between login/recover so the
    // operator can type immediately without clicking.
    if (changed.has("recoverMode") || changed.has("savedCode")) this.focusFirst();
  }

  private focusFirst() {
    const el = this.renderRoot.querySelector("input") as HTMLInputElement | null;
    el?.focus();
  }

  private val(id: string): string {
    const el = this.renderRoot.querySelector(`#${id}`) as HTMLInputElement | null;
    return el?.value ?? "";
  }

  private async submit(e: Event) {
    e.preventDefault();
    if (this.busy) return; // guard double-submit (Enter + click)
    this.error = "";

    const setup = !this.configured;
    const recover = this.configured && this.recoverMode;

    if (setup || recover) {
      if (this.val("pw") !== this.val("pw2")) {
        this.error = "Passwords do not match.";
        return;
      }
    }

    this.busy = true;
    try {
      if (setup) {
        const r = await api.setup(this.val("pw"));
        this.savedCode = r.recovery_code;
      } else if (recover) {
        const r = await api.recover(this.val("code"), this.val("pw"));
        this.savedCode = r.recovery_code;
      } else {
        await api.login(this.val("pw"));
        this.done();
      }
    } catch (err) {
      this.error = (err as Error).message || "failed";
    } finally {
      this.busy = false;
    }
  }

  private done() {
    this.dispatchEvent(new CustomEvent("authed", { bubbles: true, composed: true }));
  }

  private async copyCode() {
    try {
      await navigator.clipboard?.writeText(this.savedCode);
      this.copied = true;
    } catch {
      /* clipboard unavailable — the code is selectable */
    }
  }

  render() {
    if (this.savedCode) return this.renderSaved();

    const setup = !this.configured;
    const recover = this.configured && this.recoverMode;
    const title = recover ? "Reset password" : "ECU Console";
    const intro = setup
      ? "First run — choose an operator password (min 8 characters)."
      : recover
        ? "Enter your recovery code and a new password."
        : "Enter the operator password.";

    return html`
      <form class="box" @submit=${this.submit}>
        <h1>${recover ? title : html`<span class="brand">ECU</span> Console`}</h1>
        <p>${intro}</p>

        ${recover
          ? html`
              <label for="code">Recovery code</label>
              <input id="code" type="text" autocomplete="off" spellcheck="false"
                placeholder="XXXX-XXXX-XXXX-XXXX" ?disabled=${this.busy} />
            `
          : nothing}

        <label for="pw">${recover || setup ? "New password" : "Password"}</label>
        <input id="pw" type="password"
          autocomplete=${setup || recover ? "new-password" : "current-password"}
          ?disabled=${this.busy} />

        ${setup || recover
          ? html`
              <label for="pw2">Confirm password</label>
              <input id="pw2" type="password" autocomplete="new-password" ?disabled=${this.busy} />
            `
          : nothing}

        <button class="primary" type="submit" ?disabled=${this.busy}>
          ${this.busy ? "…" : setup ? "Set password" : recover ? "Reset password" : "Sign in"}
        </button>
        <div class="err">${this.error}</div>

        ${this.configured
          ? html`<button class="link" type="button" @click=${this.toggleRecover}>
              ${recover ? "Back to sign in" : "Forgot password?"}
            </button>`
          : nothing}
      </form>
    `;
  }

  private toggleRecover = () => {
    this.recoverMode = !this.recoverMode;
    this.error = "";
  };

  private renderSaved() {
    return html`
      <div class="box">
        <h1>Save your recovery code</h1>
        <p class="warn">
          Write this down and keep it safe. It's the only way to reset your password
          without console access, and it's shown only once. Using it later replaces it
          with a new code.
        </p>
        <div class="code">${this.savedCode}</div>
        <button class="copy" type="button" @click=${this.copyCode}>
          ${this.copied ? "Copied ✓" : "Copy to clipboard"}
        </button>
        <button class="primary" type="button" @click=${this.done}>
          I've saved it — continue
        </button>
      </div>
    `;
  }
}

customElements.define("login-view", LoginView);

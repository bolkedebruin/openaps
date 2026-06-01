import { LitElement, html, css, nothing } from "lit";
import { api } from "../api.ts";

/**
 * <password-confirm-dialog kind="rekey|channel" @confirm @cancel>
 *
 * A modal that collects a step-up-gated value (a new PAN for "rekey", a new
 * channel number for "channel") together with the operator's password, then
 * verifies the password inline and emits "confirm" with the validated value.
 * The owner runs the actual privileged action; this dialog is purely about
 * collecting value+password and gating on verifyPassword. Errors from the
 * subsequent action can be surfaced back via the `actionError` property to
 * render inside the still-open dialog.
 *
 * Dispatches:
 *   "confirm" { value: string }  — operator confirmed; password verified ok.
 *                                  For kind="rekey", value is a 1-4 hex PAN.
 *                                  For kind="channel", value is "11".."26".
 *   "cancel"                      — operator cancelled or backdrop-clicked.
 *
 * The dialog is single-shot: the owner closes it (by clearing whatever state
 * caused it to mount) on a successful action. While the action is in flight
 * the owner can set `busy=true` to lock the controls.
 */
export class PasswordConfirmDialog extends LitElement {
  static properties = {
    kind: { attribute: true },
    busy: { attribute: false },
    actionError: { attribute: false },
    value: { state: true },
    password: { state: true },
    pwdError: { state: true },
    valueError: { state: true },
    pwdBusy: { state: true },
  };

  declare kind: "rekey" | "channel";
  declare busy: boolean;
  declare actionError: string;
  declare value: string;
  declare password: string;
  declare pwdError: string;
  declare valueError: string;
  declare pwdBusy: boolean;

  constructor() {
    super();
    this.kind = "rekey";
    this.busy = false;
    this.actionError = "";
    this.value = "";
    this.password = "";
    this.pwdError = "";
    this.valueError = "";
    this.pwdBusy = false;
  }

  static styles = css`
    :host { display: contents; }
    .backdrop {
      position: fixed; inset: 0;
      background: rgba(0, 0, 0, 0.55);
      display: flex; align-items: center; justify-content: center;
      z-index: 1000;
    }
    .dialog {
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: 10px;
      padding: 20px 22px;
      max-width: 460px;
      width: 92%;
      color: var(--text);
      box-shadow: 0 12px 40px rgba(0, 0, 0, 0.5);
      box-sizing: border-box;
    }
    .dialog h3 { margin: 0 0 10px; font-size: 15px; }
    .dialog p {
      margin: 0 0 14px; font-size: 13px; color: var(--muted); line-height: 1.45;
    }
    .dialog p.warn {
      color: var(--text);
      border: 1px solid var(--accent);
      background: color-mix(in srgb, var(--accent) 10%, transparent);
      border-radius: 8px;
      padding: 8px 10px;
      font-size: 12px;
      margin: 0 0 12px;
    }
    .dialog label {
      display: block; font-size: 12px; color: var(--muted);
      margin: 8px 0 6px;
    }
    .dialog input {
      width: 100%;
      box-sizing: border-box;
      padding: 9px 11px;
      background: var(--bar-bg);
      border: 1px solid var(--border);
      border-radius: 8px;
      color: var(--text);
      font: inherit;
    }
    .dialog .err {
      color: var(--err);
      border: 1px solid var(--err);
      background: color-mix(in srgb, var(--err) 12%, transparent);
      border-radius: 8px;
      padding: 8px 10px;
      font-size: 12px;
      margin-top: 10px;
    }
    .dialog .err-inline {
      color: var(--err); font-size: 12px; margin-top: 6px;
    }
    .dialog .row {
      display: flex; gap: 10px; justify-content: flex-end; margin-top: 16px;
    }
    .dialog button {
      padding: 8px 14px;
      border-radius: 8px;
      font-size: 13px;
      font-weight: 600;
      cursor: pointer;
    }
    .dialog button.primary {
      background: var(--accent); color: #04121a; border: none;
    }
    .dialog button.secondary {
      background: transparent; color: var(--muted); border: 1px solid var(--border);
    }
    .dialog button:disabled { opacity: 0.6; cursor: default; }
    @media (max-width: 480px) {
      .dialog { width: 96%; padding: 18px 16px; }
    }
  `;

  // autofocus the value input as soon as the dialog mounts.
  firstUpdated(): void {
    queueMicrotask(() => {
      const root = this.shadowRoot;
      root?.querySelector<HTMLInputElement>("#pcd_value")?.focus();
    });
  }

  private validateValue(v: string): string {
    const t = v.trim();
    if (this.kind === "rekey") {
      if (!/^[0-9a-fA-F]{1,4}$/.test(t)) {
        return "PAN must be 1–4 hexadecimal digits.";
      }
      return "";
    }
    // channel
    const n = Number(t);
    if (!Number.isInteger(n) || n < 11 || n > 26) {
      return "Channel must be an integer 11–26.";
    }
    return "";
  }

  private onValueInput = (e: Event) => {
    this.value = (e.target as HTMLInputElement).value;
    // clear the inline value error as the user types; keep pwd error.
    if (this.valueError) this.valueError = "";
  };

  private onPasswordInput = (e: Event) => {
    this.password = (e.target as HTMLInputElement).value;
    if (this.pwdError) this.pwdError = "";
  };

  private onKey = (e: KeyboardEvent) => {
    if (e.key === "Enter") {
      e.preventDefault();
      void this.submit();
    } else if (e.key === "Escape") {
      e.preventDefault();
      this.cancel();
    }
  };

  private async submit(): Promise<void> {
    if (this.pwdBusy || this.busy) return;
    const valueErr = this.validateValue(this.value);
    if (valueErr) {
      this.valueError = valueErr;
      // focus the value input
      this.shadowRoot?.querySelector<HTMLInputElement>("#pcd_value")?.focus();
      return;
    }
    if (!this.password) {
      this.pwdError = "Password required.";
      this.shadowRoot?.querySelector<HTMLInputElement>("#pcd_pwd")?.focus();
      return;
    }
    this.pwdBusy = true;
    this.pwdError = "";
    try {
      const ok = await api.verifyPassword(this.password);
      if (!ok) {
        this.pwdError = "Password is wrong.";
        this.shadowRoot?.querySelector<HTMLInputElement>("#pcd_pwd")?.focus();
        return;
      }
      // password verified — hand the value back to the owner.
      const detail = { value: this.value.trim() };
      this.dispatchEvent(
        new CustomEvent("confirm", { detail, bubbles: true, composed: true }),
      );
    } catch (e) {
      this.pwdError = (e as Error).message || "Verification failed.";
    } finally {
      this.pwdBusy = false;
    }
  }

  private cancel = () => {
    if (this.pwdBusy || this.busy) return;
    this.dispatchEvent(new CustomEvent("cancel", { bubbles: true, composed: true }));
  };

  private onBackdrop = (e: Event) => {
    // only close when the click is on the backdrop itself, not the dialog
    if (e.target === e.currentTarget) this.cancel();
  };

  private stop = (e: Event) => e.stopPropagation();

  private renderRekey() {
    return html`
      <h3>Fleet re-key</h3>
      <p>
        Broadcasts a new PAN to every inverter (opcode 0x22) and moves the
        radio onto it. Telemetry pauses while the broadcast runs; on failure
        the old PAN is restored.
      </p>
      <p class="warn">
        Privileged action — your password is required to confirm.
      </p>
      <label for="pcd_value">New PAN (1–4 hex digits, e.g. 0DCE)</label>
      <input
        id="pcd_value"
        type="text"
        autocomplete="off"
        spellcheck="false"
        maxlength="4"
        placeholder="0DCE"
        .value=${this.value}
        @input=${this.onValueInput}
        @keydown=${this.onKey}
        ?disabled=${this.pwdBusy || this.busy}
      />
      ${this.valueError
        ? html`<div class="err-inline">${this.valueError}</div>`
        : nothing}
    `;
  }

  private renderChannel() {
    return html`
      <h3>Change ZigBee channel</h3>
      <p>
        Migrates the whole fleet to a new RF channel: each inverter is hopped
        to the new channel, then the radio follows. Telemetry pauses while the
        radio moves.
      </p>
      <p class="warn">
        Not atomic — an inverter hops the instant it gets the command, so a
        partway failure can split the fleet across the old and new channels
        (the module rolls back, but already-hopped units stay on the new one).
        Re-running this same change-channel toward the new channel converges
        them. Privileged action — your password is required to confirm.
      </p>
      <label for="pcd_value">New channel (11–26)</label>
      <input
        id="pcd_value"
        type="number"
        min="11"
        max="26"
        step="1"
        inputmode="numeric"
        placeholder="20"
        .value=${this.value}
        @input=${this.onValueInput}
        @keydown=${this.onKey}
        ?disabled=${this.pwdBusy || this.busy}
      />
      ${this.valueError
        ? html`<div class="err-inline">${this.valueError}</div>`
        : nothing}
    `;
  }

  render() {
    const confirmLabel = this.kind === "rekey" ? "Re-key fleet" : "Change channel";
    return html`
      <div class="backdrop" @click=${this.onBackdrop}>
        <div class="dialog" role="dialog" aria-modal="true" @click=${this.stop}>
          ${this.kind === "rekey" ? this.renderRekey() : this.renderChannel()}
          <label for="pcd_pwd">Password</label>
          <input
            id="pcd_pwd"
            type="password"
            autocomplete="current-password"
            .value=${this.password}
            @input=${this.onPasswordInput}
            @keydown=${this.onKey}
            ?disabled=${this.pwdBusy || this.busy}
          />
          ${this.pwdError ? html`<div class="err">${this.pwdError}</div>` : nothing}
          ${this.actionError && !this.pwdError
            ? html`<div class="err">${this.actionError}</div>`
            : nothing}
          <div class="row">
            <button
              class="secondary"
              type="button"
              @click=${this.cancel}
              ?disabled=${this.pwdBusy || this.busy}
            >
              Cancel
            </button>
            <button
              class="primary"
              type="button"
              @click=${() => void this.submit()}
              ?disabled=${this.pwdBusy || this.busy}
            >
              ${this.pwdBusy ? "Verifying…" : this.busy ? "Working…" : confirmLabel}
            </button>
          </div>
        </div>
      </div>
    `;
  }
}

customElements.define("password-confirm-dialog", PasswordConfirmDialog);

import { LitElement, html, css, nothing } from "lit";
import { api, type AccessState, type SshKey } from "../api.ts";
import { fmtTime } from "../format.ts";

/**
 * <security-view> manages the box's SSH access plane via recoveryd, the
 * single owner of authorized_keys.
 *
 * It lists the authorized keys (fingerprint, comment, added
 * date), lets the operator paste a new public key (validated server-side by
 * recoveryd), and remove a key. Removal is high-impact — it can lock an
 * operator out of shell access — so it is gated by a password step-up: the
 * operator confirms their password, which the server requires within a short
 * window before it accepts the DELETE.
 *
 * Provider semantics: "openaps" renders /root/.ssh/authorized_keys (the ECU
 * console manages dropbear access); "host" renders a host user's keys; "off"
 * is a no-op (recoveryd is not managing any keys).
 */
export class SecurityView extends LitElement {
  static properties = {
    state: { state: true },
    loading: { state: true },
    error: { state: true },
    notice: { state: true },
    adding: { state: true },
    addError: { state: true },
    // delete step-up dialog
    pendingFp: { state: true },
    pwError: { state: true },
    deleting: { state: true },
  };

  declare state: AccessState | null;
  declare loading: boolean;
  declare error: string;
  declare notice: string;
  declare adding: boolean;
  declare addError: string;
  declare pendingFp: string; // fingerprint awaiting password confirm, "" when no dialog
  declare pwError: string;
  declare deleting: boolean;

  constructor() {
    super();
    this.state = null;
    this.loading = false;
    this.error = "";
    this.notice = "";
    this.adding = false;
    this.addError = "";
    this.pendingFp = "";
    this.pwError = "";
    this.deleting = false;
  }

  static styles = css`
    :host { display: block; }
    .panel {
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: 10px;
      padding: 24px;
      max-width: 680px;
    }
    .panel + .panel { margin-top: 22px; }
    h2 { font-size: 15px; margin: 0 0 4px; color: var(--text); }
    .sub { color: var(--muted); font-size: 12px; margin: 0 0 16px; }
    .banner { border-radius: 8px; padding: 10px 12px; font-size: 13px; margin-bottom: 16px; }
    .banner.ok { color: var(--ok); border: 1px solid var(--ok); background: color-mix(in srgb, var(--ok) 12%, transparent); }
    .banner.err { color: var(--err); border: 1px solid var(--err); background: color-mix(in srgb, var(--err) 12%, transparent); }
    .loading { color: var(--muted); font-size: 13px; }
    .nudge {
      border: 1px dashed var(--border);
      border-radius: 8px;
      padding: 16px;
      color: var(--muted);
      font-size: 13px;
      text-align: center;
    }
    ul.keys { list-style: none; margin: 0; padding: 0; }
    li.key {
      display: flex;
      align-items: flex-start;
      justify-content: space-between;
      gap: 12px;
      padding: 12px 0;
      border-top: 1px solid var(--border);
    }
    li.key:first-child { border-top: none; }
    .keymeta { min-width: 0; }
    .comment { color: var(--text); font-size: 14px; font-weight: 600; }
    .comment.none { color: var(--muted); font-weight: 400; font-style: italic; }
    .fp {
      font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
      font-size: 12px;
      color: var(--muted);
      word-break: break-all;
      margin-top: 3px;
    }
    .added { color: var(--muted); font-size: 12px; margin-top: 3px; }
    label { display: block; font-size: 12px; color: var(--muted); margin: 14px 0 6px; }
    textarea, input {
      width: 100%;
      box-sizing: border-box;
      padding: 9px 12px;
      background: var(--bar-bg);
      border: 1px solid var(--border);
      border-radius: 8px;
      color: var(--text);
      font-size: 14px;
    }
    textarea {
      font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
      font-size: 12px;
      resize: vertical;
      min-height: 64px;
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
    button.danger {
      margin-top: 0;
      background: transparent;
      color: var(--err);
      border: 1px solid var(--err);
      padding: 6px 12px;
      flex: none;
    }
    button.danger:hover { background: color-mix(in srgb, var(--err) 12%, transparent); }
    button:disabled { opacity: 0.6; cursor: default; }
    .addrow { border-top: 1px solid var(--border); margin-top: 18px; padding-top: 6px; }
    /* step-up dialog */
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
      max-width: 440px;
      width: 92%;
      box-sizing: border-box;
      box-shadow: 0 12px 40px rgba(0, 0, 0, 0.5);
    }
    .dialog h3 { margin: 0 0 10px; font-size: 15px; color: var(--text); }
    .dialog p { margin: 0 0 12px; font-size: 13px; color: var(--muted); line-height: 1.45; }
    .dialog .row { display: flex; gap: 10px; justify-content: flex-end; margin-top: 16px; }
    .dialog button { margin-top: 0; }
    .dialog button.secondary { background: transparent; color: var(--muted); border: 1px solid var(--border); }
    .dialog .err { color: var(--err); font-size: 12px; margin-top: 8px; }
  `;

  connectedCallback(): void {
    super.connectedCallback();
    void this.load();
  }

  private async load() {
    this.loading = true;
    try {
      this.state = await api.sshKeys();
      this.error = this.state.error ?? "";
    } catch (e) {
      this.error = (e as Error).message;
    } finally {
      this.loading = false;
    }
  }

  private val(id: string): string {
    const el = this.renderRoot.querySelector(`#${id}`) as HTMLInputElement | HTMLTextAreaElement | null;
    return el?.value ?? "";
  }

  private clear(id: string) {
    const el = this.renderRoot.querySelector(`#${id}`) as HTMLInputElement | HTMLTextAreaElement | null;
    if (el) el.value = "";
  }

  private addKey = async (e: Event) => {
    e.preventDefault();
    if (this.adding) return;
    this.addError = "";
    this.notice = "";
    const pubkey = this.val("pubkey").trim();
    if (!pubkey) {
      this.addError = "Paste a public key.";
      return;
    }
    this.adding = true;
    try {
      this.state = await api.addSshKey(pubkey, this.val("comment").trim());
      this.error = this.state.error ?? "";
      this.notice = "Key added.";
      this.clear("pubkey");
      this.clear("comment");
    } catch (err) {
      this.addError = (err as Error).message || "failed";
    } finally {
      this.adding = false;
    }
  };

  // askDelete opens the step-up dialog for one fingerprint.
  private askDelete(fp: string) {
    this.pendingFp = fp;
    this.pwError = "";
    queueMicrotask(() => {
      this.renderRoot.querySelector<HTMLInputElement>("#delpw")?.focus();
    });
  }

  private cancelDelete = () => {
    if (this.deleting) return;
    this.pendingFp = "";
    this.pwError = "";
  };

  // confirmDelete verifies the operator password (which marks the session as
  // stepped-up server-side) and then removes the key. The two calls must be
  // close together — the server only honours the DELETE within the step-up
  // window opened by the verify.
  private confirmDelete = async () => {
    if (this.deleting) return;
    const pw = this.val("delpw");
    if (!pw) {
      this.pwError = "Password required.";
      return;
    }
    this.deleting = true;
    this.pwError = "";
    this.notice = "";
    try {
      const ok = await api.verifyPassword(pw);
      if (!ok) {
        this.pwError = "Password is wrong.";
        return;
      }
      this.state = await api.removeSshKey(this.pendingFp);
      this.error = this.state.error ?? "";
      this.notice = "Key removed.";
      this.pendingFp = "";
    } catch (err) {
      this.pwError = (err as Error).message || "failed";
    } finally {
      this.deleting = false;
    }
  };

  private onDialogKey = (e: KeyboardEvent) => {
    if (e.key === "Enter") {
      e.preventDefault();
      void this.confirmDelete();
    } else if (e.key === "Escape") {
      e.preventDefault();
      this.cancelDelete();
    }
  };

  private renderKey(k: SshKey) {
    return html`
      <li class="key">
        <div class="keymeta">
          ${k.comment
            ? html`<div class="comment">${k.comment}</div>`
            : html`<div class="comment none">(no comment)</div>`}
          <div class="fp">${k.fingerprint}</div>
          <div class="added">Added ${fmtTime(k.added_ms)}</div>
        </div>
        <button
          class="danger"
          type="button"
          @click=${() => this.askDelete(k.fingerprint)}
          ?disabled=${this.deleting}
        >
          Remove
        </button>
      </li>
    `;
  }

  private renderKeysPanel() {
    const keys = this.state?.keys ?? [];
    const provider = this.state?.provider ?? "";
    return html`
      <div class="panel">
        <h2>SSH keys</h2>
        <p class="sub">
          Authorized keys for shell access${provider ? html` · provider: ${provider}` : nothing}${this.state?.host_user
            ? html` (${this.state.host_user})`
            : nothing}.
        </p>
        ${this.notice ? html`<div class="banner ok">${this.notice}</div>` : nothing}
        ${this.error ? html`<div class="banner err">⚠ ${this.error}</div>` : nothing}
        ${this.loading && !this.state
          ? html`<div class="loading">Loading…</div>`
          : keys.length === 0
            ? html`<div class="nudge">
                No SSH keys — add one below for shell access.
              </div>`
            : html`<ul class="keys">
                ${keys.map((k) => this.renderKey(k))}
              </ul>`}

        <form class="addrow" @submit=${this.addKey}>
          <label for="pubkey">Public key</label>
          <textarea
            id="pubkey"
            placeholder="ssh-ed25519 AAAA… user@host"
            spellcheck="false"
            ?disabled=${this.adding}
          ></textarea>
          <label for="comment">Comment (optional)</label>
          <input id="comment" type="text" placeholder="laptop" ?disabled=${this.adding} />
          <button class="primary" type="submit" ?disabled=${this.adding}>
            ${this.adding ? "…" : "Add key"}
          </button>
          ${this.addError ? html`<div class="banner err" style="margin-top:12px">⚠ ${this.addError}</div>` : nothing}
        </form>
      </div>
    `;
  }

  private renderDeleteDialog() {
    if (!this.pendingFp) return nothing;
    return html`
      <div class="backdrop" @click=${(e: Event) => { if (e.target === e.currentTarget) this.cancelDelete(); }}>
        <div class="dialog" role="dialog" aria-modal="true">
          <h3>Remove SSH key</h3>
          <p>
            Removing this key revokes its shell access. If it's the only key,
            you may lose console-less access to the box. Confirm with your
            password.
          </p>
          <input
            id="delpw"
            type="password"
            autocomplete="current-password"
            placeholder="Password"
            @keydown=${this.onDialogKey}
            ?disabled=${this.deleting}
          />
          ${this.pwError ? html`<div class="err">${this.pwError}</div>` : nothing}
          <div class="row">
            <button class="secondary" type="button" @click=${this.cancelDelete} ?disabled=${this.deleting}>
              Cancel
            </button>
            <button class="danger" type="button" @click=${() => void this.confirmDelete()} ?disabled=${this.deleting}>
              ${this.deleting ? "Removing…" : "Remove key"}
            </button>
          </div>
        </div>
      </div>
    `;
  }

  render() {
    return html`
      ${this.renderKeysPanel()}
      ${this.renderDeleteDialog()}
    `;
  }
}

customElements.define("security-view", SecurityView);

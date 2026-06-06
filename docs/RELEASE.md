# OpenAPS v1.0.11

`recoveryd` now treats `authorized_keys` as the **single source of truth**.
The prior release kept a separate `/etc/recoveryd/access.json` key list that
the daemon *rendered* into `authorized_keys` — letting the two diverge (the UI
could show an empty list while `authorized_keys` still held a key) and forcing a
"refuse to render empty over non-empty" anti-brick hack plus an installer
adoption/seed step. This release removes that indirection entirely: `recoveryd`
reads and rewrites the real `authorized_keys` file directly.

## Changed

- **`authorized_keys` is the source of truth.** `recoveryd` parses it on every
  `ListKeys`, and every `AddKey`/`RemoveKey` is an atomic
  (temp+fsync+rename, mode `0600`) rewrite of that same file. The `.ssh` parent
  dir is ensured `0700` (and chowned to the target user under the host
  provider).
- **Removed the `access.json` store and the render-from-list model**, including
  the "refuse to render empty over non-empty" guard, the boot-render step, the
  installer `seed` subcommand, and the `-access` flag. This supersedes the
  `access.json` render model shipped in the prior release.
- **One lockout guard remains:** `RemoveKey` refuses to remove the *last*
  remaining key ("refusing to remove the only key — you would lose access").
- **Provider is selected by flags, not a config file** (provider switching over
  the API was deferred): `-authorized-keys` (default `/root/.ssh/authorized_keys`),
  `-chown-user` (default empty = root; set for the host/Pi provider),
  `-manage-dropbear` (default `true`; host/Pi sets `false` to defer to the host
  sshd). `-socket` and `-dropbear-host-key` are unchanged. `S97-recoveryd` passes
  `-authorized-keys /root/.ssh/authorized_keys -manage-dropbear=true`.
- **Boot is render-free:** `recoveryd` ensures the `.ssh` dir and (when managing
  dropbear) the host key, then serves. If `authorized_keys` is absent,
  `ListKeys` returns empty and the operator adds keys via the web UI.
- **Installer** appends the bundled operator key to `/root/.ssh/authorized_keys`
  directly (atomic, deduped) — the original pre-`recoveryd` behavior — and still
  installs the `recoveryd` binary + `S97-recoveryd`. No `access.json` is written.
- The protobuf UDS API (`ListKeys`/`AddKey`/`RemoveKey`/`Status`) and the
  `ecu-web` Security page are unchanged. `Status` now reports the managed
  `authorized_keys` path and the key count parsed from the file. The
  dropbear-managed flag is intentionally **not** surfaced over the API — the
  proto surface is frozen, so adding a field is out of scope; it is a
  start-up flag only.
- **Restricted keys are preserved.** `authorized_keys` options
  (`command=`, `from=`, `no-pty`, …) are parsed and re-emitted verbatim, so a
  forced-command/source-locked key keeps its restrictions across the rewrite
  every mutation performs (a control byte in an option is rejected, like the
  comment).
- **Atomic writes are centralised** in `internal/atomicfile`: the host
  provider's chown-before-rename now goes through `atomicfile.WriteOwned`
  rather than a hand-rolled second copy of the fsync+rename choreography, and
  `writeSync` chmods explicitly so a stale temp left by a crash can't publish
  looser-than-`0600` permissions.

# OpenAPS v1.0.10

Bug-fix for `recoveryd` (v1.0.9). `recoveryd` defaulted its UDS to
`/run/recoveryd.sock`, but the ECU's old BusyBox userspace (Linux 3.2) has **no
`/run`** (only `/var/run` → a volatile tmpfs) — so the daemon boot-rendered
`authorized_keys` correctly but then failed to bind its socket and exited.
Hardware-validated on a real ECU-R-Pro.

## Fixed

- **`recoveryd` socket now defaults to `/var/run/recoveryd.sock`** (exists on the
  ECU, the correct volatile-runtime home, and resolves to `/run` on modern
  systems / Raspberry Pi). `cmd/recoveryd`, the `ecu-web` client default, and
  `S97-recoveryd` all updated to match.
- **`recoveryd` now `mkdir -p`s the socket's parent dir before listening** — a
  defensive belt for any custom `-socket` path.

(The v1.0.9 daemon's `authorized_keys` render, boot-render anti-brick guard, and
dropbear host-key ensure were already confirmed working on the ECU; only the
socket bind was broken.)

## Carried forward — v1.0.9 `recoveryd`

Dedicated, hardened root daemon that owns the SSH access plane (single writer of
`authorized_keys`, source of truth `/etc/recoveryd/access.json`, full-rewrite
render on change + at boot, refuses to empty a non-empty `authorized_keys`,
durable temp+fsync+rename writes, protobuf UDS w/ `SO_PEERCRED` uid-0 gate,
provider openaps/host/off), plus the `ecu-web` Security page to manage keys.

## Compatibility matrix unchanged from v1.0.4

| Family | Telemetry | Grid profile | Set-power | Pairing |
|---|:--:|:--:|:--:|:--:|
| DS3 | ✅ | ✅ | ✅ | ✅ |
| QS1A | ✅ | ✅ | ✅ | ✅ |
| QS1 | ✅ | ✅ | ✅ | ✅ |
| DSP4 | ✅ | ✅ | ✅ | ⚠️ |
| YC600 | ✅ | ✅ | ✅ | ⚠️ |
| YC1000, QT2 | ⚠️ | ⚠️ | ⚠️ | ⚠️ |

## Install / upgrade

```sh
curl -H "Expect:" -F "file=@openaps-v1.0.10-ecu.tar.bz2" \
     http://<ECU-IP>/index.php/management/exec_upgrade_ecu_app
```

`{"res":0}` = received + launched; watch `/home/openaps-install.log` until the
reboot, then open `https://<ECU-IP>/` after ~1-2 minutes. Roll back with
`ssh root@<ECU-IP> /usr/local/bin/openaps-rollback`.

## Still deferred

- **Out-of-band recovery** (button/GPIO into `recoveryd`); `idwriter` remains the
  interim path.
- **Provider switching over the UDS API** — set via `access.json` for now.
- **Signed-tarball OTA**, **passkeys**, **AES encrypt wire-up**, **v2 Pi/.deb**.

## Artifacts

- `openaps-v1.0.10-ecu.tar.bz2` — brownfield installer.
- `SHA256SUMS`.

## Install caveats

Install only on LANs you control.

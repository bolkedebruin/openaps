# OpenAPS v1.0.13

`recoveryd` now manages root's **real** home dir, fixing a wrong-file bug that
silently desynced the Security page from the file dropbear actually reads — and
could lock an operator out of SSH.

## Fixed

- **recoveryd resolves the managed user's real `~/.ssh/authorized_keys`.** On the
  ECU root's home per `/etc/passwd` is `/home/root`, so dropbear authenticates
  against `/home/root/.ssh/authorized_keys`. recoveryd previously hardcoded
  `/root/.ssh/authorized_keys` (a different file), so the web console listed a
  stale key set, "added a key, success but nothing written" reflected a write to
  the wrong file, and removing/moving the real file left recoveryd unable to
  restore access — an SSH lockout. `Provider.path()` now uses an explicit
  `-authorized-keys` verbatim when given, otherwise resolves the managed user's
  home (`-chown-user` when set, else `root`) via the injectable user lookup and
  returns `<home>/.ssh/authorized_keys`, resolved once under a `sync.Once` so a
  tight read/write loop does not re-parse `/etc/passwd` and the UDS `Status` call
  (which reads the path without the Manager mutex) cannot race a concurrent
  `AddKey`. `/root/.ssh/authorized_keys` remains only as a
  last-ditch fallback when the lookup fails or the home is empty. `readKeys`,
  `writeKeys`, `ensureSSHDir` and `resolveOwner` all agree on the resolved path.
- **`recoveryd` `-authorized-keys` flag now defaults to empty.** Empty resolves
  the managed user's `~/.ssh/authorized_keys` (root by default); the `S97-recoveryd`
  init script drops the hardcoded `-authorized-keys /root/.ssh/authorized_keys`
  and starts recoveryd with just `-manage-dropbear=true -socket …`, so it targets
  root's real home. The host/Pi init passes `-chown-user` instead.
- **Installer writes the bundled operator key to root's real home.**
  `openaps-install` derives `ROOT_HOME` from `/etc/passwd`
  (`awk -F: '$1=="root"{print $6}'`, fallback `/root`) and appends to
  `$ROOT_HOME/.ssh/authorized_keys` (0600, `.ssh` 0700), BusyBox-safe. No path is
  hardcoded, so the seeded recovery key lands where dropbear reads it.
- **Security view re-fetches on every visit.** `<security-view>` reloads the key
  list on each connect and whenever the tab becomes visible again, so navigating
  away and back (or returning to a backgrounded tab) always reflects the current
  file on disk rather than a stale snapshot.
- **Install-time socket readiness check matches the init script.** `openaps-install`
  now probes `/var/run/recoveryd.sock`, the same path `S97-recoveryd` publishes, so
  a healthy start on a box where `/var/run` is not a symlink to `/run` no longer
  prints a spurious "socket absent" warning.

---

# OpenAPS v1.0.12

Applying a grid profile from the web console no longer times out. A base-profile
select (and an overlay clear) now reconciles the fleet **asynchronously**: the
HTTP request returns immediately once the reconcile is queued, and progress
flows to the Events log — the same pattern overlay saves already used.

## Changed

- **Base-profile select and overlay clear are now async (inv-driver).**
  `selectBase` persists the active base and enqueues a fleet-wide reconcile onto
  the existing per-uid background applier, then returns `{"status":"reconciling"}`
  at once; `clearOverlay` clears the row and queues a per-uid reconcile-to-base
  the same way. Neither blocks on the per-point `ReadSettle` waits anymore.
  Previously both ran the reconcile synchronously inside the IPC handler — a
  single drifted point already exceeded ecu-web's 8 s UDS read deadline, and a
  fleet blocked for tens of seconds, so the web console reported
  `read unix @->/var/run/inv-driver.sock: i/o timeout`.
- **Fleet reconcile reuses the per-uid serialization.** Each inverter is
  reconciled through the same keyed applier the overlay path uses, so a
  base-select reconcile never races an overlapping overlay apply for the same
  inverter (newer job supersedes older, per uid). Two new fleet-level audit
  events bracket the run: `profile_apply_started` / `profile_apply_complete`,
  alongside the per-uid `overlay_apply_started` / `overlay_param_written` /
  `overlay_param_failed` / `overlay_apply_complete` rows and the existing
  `profile_select` row.
- **The broadcast base-select path (`-gridprofile-broadcast`) is async too.**
  The opt-in broadcast push and its unicast follow-up reconcile now run as a
  fleet job on the same background applier: the goroutine acquires the bus lock
  for its lifetime, broadcasts the active base, then reconciles each inverter
  through the per-uid applier. The IPC roundtrip returns `{"status":"reconciling"}`
  immediately, so a broadcast-enabled ECU no longer hits the read timeout either.
  A "bus busy" rejection (pairing holds the lock) is surfaced as a
  `profile_apply_complete` warn event rather than a synchronous error.
- **ecu-web returns 202 Accepted.** `POST /api/profiles/base` and
  `DELETE /api/profiles/overlay` now answer `202` with
  `{"status":"reconciling"}` instead of waiting for a synchronous pass/fail. The
  ineffective per-call timeout wrapper on the overlay-clear loop is gone; the 8 s
  controller read deadline is unchanged (responses are immediate now).
- **Web console UI is event-driven for profile applies.** Selecting a base or
  clearing a Local Site profile shows "reconciling — see Events" and the operator
  watches the Events stream for per-inverter outcomes, matching how overlay saves
  already reported. Any uid whose persist/queue step failed up front is still
  surfaced inline.

---

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

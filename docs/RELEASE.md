# OpenAPS v1.0.6

Patch release in the `v1.0.x` stream. **Makes the lighttpd removal survivable.**
On a real ECU-R-Pro the installer ran cleanly through the supervisor-disable step
and then died at the `killall lighttpd` — taking the rest of the install (start
services, smoke verify, audit, rollback CLI) with it and leaving a half-migrated
box: stock UI gone, OpenAPS not started, no rollback CLI. assist is a child of
php-cgi under lighttpd, so killing lighttpd tears down assist's process group; the
v1.0.4 `trap '' HUP` didn't help because the fatal signal isn't SIGHUP, and
`setsid` isn't on this BusyBox to re-detach. v1.0.6 reorders and hardens the
install so a death at the kill can no longer lose the install.

## Fixed — lighttpd removal no longer kills the install

- **Reordered: lighttpd removal is now the LAST step.** The old step 7 (kill
  lighttpd) ran before services were started. The new order is: disable stock
  supervisors (6) → start OpenAPS services (7) → smoke verify (8) → audit +
  install `openaps-rollback` + write `installed.json` (9) → remove lighttpd (10).
  By the time the dangerous kill happens, the install is functionally complete
  and persistent, so an assist death at the kill loses nothing. ecu-web (`:443`),
  ecu-sunspec (`:502`) and stock lighttpd (`:80`) don't collide, so OpenAPS starts
  fine while lighttpd is still up.
- **lighttpd is neutralized on disk BEFORE the killall.** The binary is moved to
  `$STATE_DIR/disabled-bin/lighttpd` and any `rcS.d`/`init.d` lighttpd entry is
  disabled first; only then is the running process killed. If assist still dies at
  the `killall`, lighttpd is already off the boot path and won't restart.
- **`trap '' HUP TERM`** now also ignores SIGTERM (KILL stays untrapped — it's
  uncatchable, which is precisely why the removal is ordered last). This
  supersedes the v1.0.4 `trap '' HUP`, which was insufficient.
- The step-3 `DROPBEAR_INSTALLED` recovery-path guard still gates the lighttpd
  removal; the relabeled step headers/comments read coherently in the new order.

## Hardened — non-destructive disable + single-instance lock

- **lighttpd is copied aside, not `mv`'d-or-deleted.** `/usr/local/lighttpd` is
  not in the backup tarball and `openaps-rollback` restores lighttpd only from
  `$STATE_DIR/disabled-bin/lighttpd`, so the old `mv … || rm -f` fallback could
  delete the sole binary (e.g. on a failed cross-device move) and leave the stock
  UI permanently unrecoverable. The step now does `cp -f` then `rm -f`, removing
  the original only after the copy lands, with no `rm -f` fallback — under
  `set -e` a copy failure aborts instead of destroying the binary. The step-6
  supervisor-disable moves use the same copy-then-remove pattern.
- **Single-instance lock.** The unauthenticated `exec_upgrade_ecu_app` endpoint
  stays callable for the whole install (~40s, mostly the step-8 `sleep 30`), and
  the reorder widens that window — a second POST would spawn a concurrent assist
  racing the first on service start, `killall`, the `disabled-apps.list` rewrite
  and the `installed.json` write. An atomic `mkdir "$STATE_DIR/install.lock"`
  guard now rejects a second run; it is cleaned up on a normal `EXIT` while the
  `HUP`/`TERM` ignores are preserved.

## Carried over from v1.0.5 — first-run provisioning

- **`inv-driver import-stock`** seeds the inverter inventory from the stock
  APsystems DB. OpenAPS's ecu-zb is a transparent splice (it doesn't generate
  polls); the installer disables `main.exe`, so inv-driver becomes the sole
  poller — but it only polls inverters already in its inventory, and a running
  fleet doesn't spontaneously re-announce mid-day. The new subcommand imports the
  paired list from `/home/database.db` (`id` table: serial → uid, short_address,
  model → family) so the 0xBB poll targets the real fleet right away. It's
  idempotent, validates serials, range-checks short_address/model code, and never
  ages back a row already carrying live telemetry. The installer runs it
  best-effort in step 5 (`import-stock -stock-db /home/database.db`); failure
  never aborts the install, and the passive dawn-announce path remains the
  fallback for newly paired inverters.

- **Base grid-profile library is now bundled.** The installer drops the
  `gridprofiles-seed/profiles/` set (66 grid codes — EN50438, LFSM-O, Denmark-1,
  C10-26, default-50Hz, the AS/NZS and UL families, …) into
  `/var/lib/inv-driver/gridprofiles/profiles/`, so `gp_base_profiles` is
  populated and the console has selectable profiles on first boot. Operator
  overlays (a separate dir) are untouched.

## Fixed — rollback

- `openaps-rollback` called bare `reboot`, which isn't on `PATH` in the rollback
  context (`reboot: command not found`) — the restore ran but the box never
  rebooted. Now tries `/sbin/reboot`, then the BusyBox applet, then the sysrq
  trigger.
- The step-3 abort hint in the installer still printed the old
  `tar -xjf … -P -C /` recovery command; corrected to `tar -xzf … -C /` to match
  the gzip backup.

## Carried over (v1.0.3 / v1.0.4 installer fixes)

ELF preflight uses `od -c` (BusyBox `od` has no `-A`); gzip backup (no `-P`, no
bzip2 compressor; fails on empty); `S98-dropbear` starts with `-r` + a
`dropbearkey`-generated host key (no `-R`); ecu-sunspec installs as a bare binary;
`slave485` is disabled so ecu-sunspec can bind `:502`; install `curl` examples
carry `-H "Expect:"`; the installer stamps its real version. (The v1.0.4
`assist`-survives-SIGHUP measure is superseded by the v1.0.6 reorder above.)

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
curl -H "Expect:" -F "file=@openaps-v1.0.6-ecu.tar.bz2" \
     http://<ECU-IP>/index.php/management/exec_upgrade_ecu_app
```

Watch `/home/openaps-install.log`. Roll back with
`ssh root@<ECU-IP> /usr/local/bin/openaps-rollback`.

## Still deferred

- **Signed-tarball OTA** — `release.pub` ships; `/api/upgrade` not yet plumbed.
- **Passkeys** — blocked on WebAuthn rpId / PSL constraints; needs a real DNS name.
- **AES encrypt wire-up in ecu-zb** — inbound decrypt only.
- **v2:** Pi / .deb / systemd packaging; `bus-mgr ti-znp-zb` USB-radio backend.

## Artifacts

- `openaps-v1.0.6-ecu.tar.bz2` — brownfield installer.
- `SHA256SUMS`.

## Install caveats

Install only on LANs you control.

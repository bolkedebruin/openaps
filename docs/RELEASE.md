# OpenAPS v1.0.5

Patch release in the `v1.0.x` stream. **Completes the brownfield first-run
experience.** v1.0.4 made the installer run to completion; a full migration then
showed that a fresh install came up with an empty inverter inventory and no grid
profiles until the next dawn. v1.0.5 provisions both at install time, so the
console is populated immediately after a mid-day migration.

## New — first-run provisioning

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
`dropbearkey`-generated host key (no `-R`); `assist` ignores SIGHUP so it
survives killing lighttpd at step 7; ecu-sunspec installs as a bare binary;
`slave485` is disabled so ecu-sunspec can bind `:502`; install `curl` examples
carry `-H "Expect:"`; the installer stamps its real version.

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
curl -H "Expect:" -F "file=@openaps-v1.0.5-ecu.tar.bz2" \
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

- `openaps-v1.0.5-ecu.tar.bz2` — brownfield installer.
- `SHA256SUMS`.

## Install caveats

Install only on LANs you control.

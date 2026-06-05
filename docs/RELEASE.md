# OpenAPS v1.0.7

Patch release in the `v1.0.x` stream. **The brownfield install now finishes
unattended and lands on a working OpenAPS with zero manual steps.** v1.0.6 made
the orchestrator survive killing lighttpd (it reaches `=== done ===`), but the
OpenAPS services it had started in-process were still torn down with lighttpd's
process group, so the box came up empty until a manual reboot. v1.0.7 reboots at
the end of the install, so init starts OpenAPS cleanly outside any web process
group.

## Fixed — install reboots into a working OpenAPS

- **The installer reboots as its last step.** `assist` is launched by PHP under
  lighttpd, so the services it starts in step 7 share lighttpd's process group
  and die when lighttpd is killed in step 10 (the same teardown the v1.0.6
  reorder protects `assist` itself from via `trap '' HUP TERM`; the Go services
  don't trap TERM). Rather than fight the process group without `setsid` (absent
  on this BusyBox), the install now does all its on-disk work — including the
  step-7 start + step-8 smoke that validate the services *can* run — and then
  `reboot`s. Init brings up dropbear, inv-driver, ecu-zb, ecu-web and ecu-sunspec
  cleanly, with the stock stack disabled. Validated on a real ECU-R-Pro: a full
  `exec_upgrade_ecu_app` install now completes and comes back as working OpenAPS
  (`:443` console + the 4 services) with no manual intervention.
- The operator's `curl` already returned `res:0` before the reboot, so it's
  transparent; the console returns in ~1-2 minutes. README / `INSTALL-ECU` note
  the reboot. The reboot uses the same `/sbin/reboot` → BusyBox applet → sysrq
  fallback as `openaps-rollback`.

## The v1.0.x installer arc (all carried forward)

Found and fixed by running the installer end-to-end on real hardware:

- **v1.0.3** — BusyBox blockers: ELF preflight `od -An -c` → `od -c`; gzip backup
  (no `-P`, no bzip2 compressor); `S98-dropbear` `-r` + `dropbearkey` (no `-R`);
  version stamped at package time; `curl -H "Expect:"` for lighttpd 1.4.35's 417.
- **v1.0.4** — ecu-sunspec installs as a bare binary; `slave485` disabled so
  ecu-sunspec can bind `:502`.
- **v1.0.5** — `inv-driver import-stock` seeds the inventory from the stock
  `/home/database.db` so a mid-day migration polls the fleet immediately; the
  base grid-profile library is bundled and installed; `openaps-rollback` uses
  `/sbin/reboot`.
- **v1.0.6** — lighttpd removal reordered to last (start services → smoke →
  audit + rollback CLI → remove lighttpd); non-destructive `cp`-then-`rm`;
  `trap '' HUP TERM`; single-instance install lock.
- **v1.0.7** — reboot at the end so OpenAPS starts clean via init (this release).

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
curl -H "Expect:" -F "file=@openaps-v1.0.7-ecu.tar.bz2" \
     http://<ECU-IP>/index.php/management/exec_upgrade_ecu_app
```

`{"res":0}` means received + launched; watch `/home/openaps-install.log` until
the reboot, then open `https://<ECU-IP>/` after ~1-2 minutes. Roll back with
`ssh root@<ECU-IP> /usr/local/bin/openaps-rollback`.

## Still deferred

- **Signed-tarball OTA** — `release.pub` ships; `/api/upgrade` not yet plumbed.
- **Passkeys** — blocked on WebAuthn rpId / PSL constraints; needs a real DNS name.
- **AES encrypt wire-up in ecu-zb** — inbound decrypt only.
- **v2:** Pi / .deb / systemd packaging; `bus-mgr ti-znp-zb` USB-radio backend.

## Artifacts

- `openaps-v1.0.7-ecu.tar.bz2` — brownfield installer.
- `SHA256SUMS`.

## Install caveats

Install only on LANs you control.

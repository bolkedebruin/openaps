# OpenAPS v1.0.4

Patch release in the `v1.0.x` stream. **Completes the installer fix begun in
`v1.0.3`.** v1.0.3 cleared the three BusyBox blockers that aborted the install
early; a full end-to-end install on a real ECU then surfaced three more issues
that only became reachable once the install got past those points. v1.0.4 fixes
them, so the brownfield installer now runs to completion unattended. No
operator-facing behaviour changes once installed.

## Fixed — the installer now completes end-to-end

Found by running v1.0.3 all the way through on a live ECU-R-Pro:

- **The orchestrator killed itself at step 7.** `assist` is launched by PHP
  under lighttpd, so it lives in lighttpd's session. Step 7 does
  `killall lighttpd` (the session leader) → the orchestrator received SIGHUP and
  died *before* starting OpenAPS — install left half-done (stock UI gone, no
  services). It now `trap '' HUP` (the same SIGHUP-immunity the init scripts
  already use for the FPM context).

- **ecu-sunspec was installed to the wrong path.** Step 5 `mkdir`'d
  `/home/applications/ecu-sunspec` as a directory and then installed the binary
  to that same path, so `put_atomic`'s `mv` dropped it *inside* as
  `ecu-sunspec/ecu-sunspec.new` — S99-sunspec then couldn't find its binary.
  ecu-sunspec is a bare binary (like S99's `BIN=`); the stray `mkdir` is gone.

- **`slave485` blocked ecu-sunspec's Modbus port.** The stock `slave485` daemon
  holds `:502`, so ecu-sunspec couldn't bind its SunSpec Modbus server. It's now
  in the installer's disable set alongside the other stock daemons.

## Carried over from v1.0.3 (the BusyBox blockers)

- ELF pre-flight used `od -An -c`; BusyBox `od` has no `-A` → fixed to `od -c`.
- Backup used `tar -cjf -P`; BusyBox tar has no `-P` and no bzip2 compressor
  (wrote a 0-byte archive) → fixed to gzip, only existing paths, fail-on-empty.
- `S98-dropbear` used `-R` (unsupported by the bundled dropbear 2012.55) → fixed
  to generate the host key with `dropbearkey` and start with `-r`.
- Installer `VERSION` is stamped at package time; install `curl` examples carry
  `-H "Expect:"` (lighttpd 1.4.35 returns 417 otherwise).

## Compatibility matrix unchanged from v1.0.3

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
curl -H "Expect:" -F "file=@openaps-v1.0.4-ecu.tar.bz2" \
     http://<ECU-IP>/index.php/management/exec_upgrade_ecu_app
```

Watch `/home/openaps-install.log` on the ECU for progress (the install runs in
the background after the HTTP reply returns; `{"res":0}` only means "received +
launched"). Roll back with `ssh root@<ECU-IP> /usr/local/bin/openaps-rollback`.

## Still deferred

- **Signed-tarball OTA** — `release.pub` ships at `/etc/openaps/release.pub`;
  `/api/upgrade` is not yet plumbed.
- **Passkeys** — blocked on WebAuthn rpId / PSL constraints; needs a real DNS name.
- **AES encrypt wire-up in ecu-zb** — inbound decrypt only.
- **v2:** Pi / .deb / systemd packaging; `bus-mgr ti-znp-zb` backend for
  off-the-shelf USB ZigBee radios.

## Artifacts

- `openaps-v1.0.4-ecu.tar.bz2` — brownfield installer.
- `SHA256SUMS`.

## Install caveats

Install only on LANs you control.

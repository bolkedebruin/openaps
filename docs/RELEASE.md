# OpenAPS v1.0.3

Patch release in the `v1.0.x` stream. **Installer-critical bug-fix release** —
the brownfield installer in `v1.0.0`–`v1.0.2` could not complete on a real
APsystems ECU because of three BusyBox incompatibilities, each of which aborted
the install before it did anything. No operator-facing behaviour changes once
installed; the four services and wire formats are unchanged from `v1.0.2`.

## Fixed — the installer now runs on a real ECU

All three were reproduced and the fixes validated on a live ECU-R-Pro
(BusyBox v1.20.2, dropbear 2012.55, am335x / ARMv7, Linux 3.2):

- **ELF pre-flight aborted on the first binary.** The binary check used
  `od -An -c`, but the ECU's BusyBox `od` has no `-A` option (`od: invalid
  option -- 'A'`) — so the magic read came back empty and the install FATAL'd
  with `does not look like ELF (magic=)` at step 1, before backup or anything
  else. Now uses plain `od -c` and globs for the `177ELF` magic.

- **Backup wrote a 0-byte archive.** Step 2 used `tar -cjf … -P`. BusyBox tar
  has **no `-P` flag** (`invalid option -- 'P'`) and **no bzip2 compressor**
  (it shells out to a missing `bzip2` and still exits 0 with an empty file).
  Backup now uses **gzip** (`openaps-backup-<ts>.tar.gz`), backs up only paths
  that exist, and hard-fails if the archive is empty. `openaps-rollback`
  autodetects compression, so both new `.tar.gz` and any legacy `.tar.bz2`
  backups restore.

- **Dropbear recovery path never bound :22.** `S98-dropbear` started dropbear
  with `-R` (auto-generate host keys), which the bundled dropbear 2012.55 does
  not support (`Unknown argument -R`). It now generates the host key with
  `dropbearkey` and starts with an explicit `-r <hostkey>`.

## Also in this release

- The installer now **reports its real version** — `VERSION` is stamped by
  `make package-openaps` at build time (it was hard-coded `v1.0.0`, so the
  `v1.0.1`/`v1.0.2` tarballs both identified themselves as `v1.0.0` in
  `/etc/openaps/installed.json` and the install log).
- **Docs:** the install `curl` examples (README, `INSTALL-ECU.md`) now include
  `-H "Expect:"`. Without it, lighttpd 1.4.35 rejects the upload with
  `417 Expectation Failed`. Also documents that the `{"res":0}` reply only means
  "received + launched" — the real outcome is in `/home/openaps-install.log`.

## Compatibility matrix unchanged from v1.0.2

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
curl -H "Expect:" -F "file=@openaps-v1.0.3-ecu.tar.bz2" \
     http://<ECU-IP>/index.php/management/exec_upgrade_ecu_app
```

Watch `/home/openaps-install.log` on the ECU for progress. Roll back with
`ssh root@<ECU-IP> /usr/local/bin/openaps-rollback`.

Or, if you already have `v1.0.x` installed with SSH access, `make deploy-all
ECU_HOST=root@<ECU-IP>` from a checkout at the `v1.0.3` tag and restart the four
services.

## Still deferred

- **Signed-tarball OTA** — `release.pub` ships at `/etc/openaps/release.pub`;
  `/api/upgrade` is not yet plumbed.
- **Passkeys** — blocked on WebAuthn rpId / PSL constraints; needs a real DNS name.
- **AES encrypt wire-up in ecu-zb** — inbound decrypt only; outbound primitive
  ready, blocked on an on-wire test vector.
- **v2:** Pi / .deb / systemd packaging; `bus-mgr ti-znp-zb` backend for
  off-the-shelf USB ZigBee radios.

## Artifacts

- `openaps-v1.0.3-ecu.tar.bz2` — brownfield installer.
- `SHA256SUMS`.

## Install caveats

Install only on LANs you control.

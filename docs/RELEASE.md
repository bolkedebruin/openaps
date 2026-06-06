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

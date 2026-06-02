# OpenAPS v1.0.0

OpenAPS is a clean-room replacement firmware for the APsystems ECU-R /
ECU-R-Pro. v1.0.0 is the first cut: it lands on the brownfield ECU via
the device's own firmware-update endpoint, disables the stock
supervisors, removes the CodeIgniter web UI, and ships a Go stack with a
modern web console.

## What's in v1.0.0

- **`inv-driver`** — bus owner. Codec, ZigBee envelope/L2, gridprofile
  manager, settings store, IPC server over UDS. Single writer for all
  ZigBee writes.
- **`ecu-zb`** — ZigBee modem proxy. UART/pty splitter, PAN
  autodetect, pcap tap on :19999.
- **`ecu-web`** — operator console. HTTP/2 + SSE on :443, embedded Lit
  SPA, single-user passkey-deferred auth (password + rotating recovery
  code).
- **`ecu-sunspec`** — Modbus/TCP + RTU adapter on :502, SunSpec
  Model 1/103/123/711 frontend. Subscriber to inv-driver over UDS.
- **Brownfield installer** — `openaps-v1.0.0-ecu.tar.bz2`. Single
  POST to the stock `exec_upgrade_ecu_app` endpoint. Backup + dropbear
  recovery + atomic install + rollback CLI.
- **Rollback** — `/usr/local/bin/openaps-rollback` restores stock
  firmware from the auto-generated backup tarball.

## What's confirmed working on real hardware

- DS3, QS1A, YC600 family read-path: telemetry ingest, signal
  strength, per-inverter limit, energy aggregates.
- Set-power: percent across all families, validated on-wire.
- On/off: family-independent 13-byte L2 frame, validated.
- Grid-protection writes: DS3 fully tunable (CA/CB/CC/CV/DC/DD); QS1A
  band+trips writable, slope fixed.
- Channel migration (FleetChangeChannel 0x0F), pairing (0x0E/0x11),
  repair via 0x22 / PAN 0xFFFF rendezvous. Live-validated on 3-inverter
  fleet.
- ZigBee modem cold-start (0x0D ping + 0x05 PAN/channel set, AES-128
  data codec).

## Installer policy

- **Brownfield ECU only.** Pi/.deb deferred to v2.
- **Single release keypair** — `release.pub` is shipped in
  `/etc/openaps/` for v1.0.1's signed OTA path. v1.0.0 itself is
  UNSIGNED — install only on LANs you control.
- **Disable, not stub.** Stock supervisors (`monitor.exe`,
  `clientmonitor`, the `autoupdate` family, `idwriter`,
  `mqtt_monitor.exe`, ...) are removed from `/home/applications/` so
  the next-boot `manager` `system()` calls no-op. No sleeper scripts,
  no `.real` swap files.
- **CodeIgniter removal is gated** on dropbear being verified up on
  :22 first. If dropbear fails to bind, the installer aborts with the
  stock web UI intact.

## What's deferred to v1.0.x

- **v1.0.1 — signed-tarball OTA.** Production release key + ecu-web
  `/api/upgrade` endpoint with RSA-PKCS1-v1.5 SHA-256 verification
  against `/etc/openaps/release.pub`. The placeholder key in v1.0.0 is
  non-functional; v1.0.1 will overwrite it.
- **v1.0.x — passkeys.** Single-user passkey auth (deferred from
  v1.0.0 because of WebAuthn rpId / public-suffix-list constraints —
  needs a real DNS name).

## What's deferred to v2

- **Pi / .deb / systemd packaging.** Blocked on a `bus-mgr ti-znp-zb`
  backend in ecu-zb so we can talk to off-the-shelf TI ZNP dongles
  instead of the APsystems-proprietary CC2530 firmware.
- **Generic ZigBee radios.** Same blocker.

## Artifacts

- `openaps-v1.0.0-ecu.tar.bz2` — brownfield installer (this release).

## Install caveats

Install only on LANs you control.

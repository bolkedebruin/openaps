<p align="center">
  <img src="logo.png" alt="OpenAPS" width="320">
</p>

<p align="center">
  <strong>Generic, vendor-independent firmware for APsystems microinverter fleets.</strong>
</p>

<p align="center">
  <a href="https://github.com/bolkedebruin/openaps/releases/latest"><img alt="latest release" src="https://img.shields.io/github/v/release/bolkedebruin/openaps"></a>
  <a href="https://github.com/bolkedebruin/openaps/actions/workflows/ci.yml"><img alt="CI" src="https://github.com/bolkedebruin/openaps/actions/workflows/ci.yml/badge.svg"></a>
  <a href="LICENSE"><img alt="license" src="https://img.shields.io/github/license/bolkedebruin/openaps"></a>
</p>

---

<p align="center">
  <img src="docs/screenshots/dashboard.png" alt="ECU Console — Dashboard" width="32%">
  <img src="docs/screenshots/inverters.png" alt="ECU Console — Inverters" width="32%">
  <img src="docs/screenshots/profiles.png" alt="ECU Console — Grid profiles" width="32%">
</p>
<p align="center">
  <em>Built-in ECU Console — dashboard (live fleet, totals, per-inverter cards), inverters (caps, encryption badges, scan/replace), profiles (base + overlays). See <a href="docs/screenshots/">docs/screenshots/</a> for events, settings and the login screen.</em>
</p>

---

OpenAPS replaces the stock firmware on APsystems ECU gateways with a clean Go stack that keeps the fleet running entirely on your LAN: no cloud uplink, no unauthenticated web admin, no firmware OTAs you don't control. You read live telemetry, push grid-protection profiles, cap output power, expose data over SunSpec/Modbus, and audit every change — locally.

**Supported hardware:** APsystems **ECU-R-Pro** (serial prefix `2162…`) and **ECU-C** (`215…`) — both run a BusyBox Linux userspace on ARMv7. **Not supported:** the original RTOS-based ECU-R (`2160…`, also sold as ECU-R-M3) and ECU-B (`2163…`) — they have no Linux userspace, so OpenAPS cannot be installed. A Raspberry Pi target is on the roadmap.

## Compatibility

| Family  | Model codes               | Telemetry | Grid profile | Set-power | Pairing | Notes                                              |
|---------|---------------------------|:--:|:--:|:--:|:--:|----------------------------------------------------|
| DS3     | `0x20 0x21 0x22 0x36`     | ✅ | ✅ | ✅ | ✅ | Live-validated                                     |
| QS1A    | `0x18`                    | ✅ | ✅ | ✅ | ✅ | Live-validated. `DC` / `CG` / `CF` rejected by inverter firmware |
| QS1     | `0x08`                    | ✅ | ✅ | ✅ | ✅ | Shares encoders with QS1A                          |
| DSP4    | `0x05 0x06`               | ✅ | ✅ | ✅ | ⚠️ | Codec implemented; pairing not exercised on hardware |
| YC600   | `0x07 0x17`               | ✅ | ✅ | ✅ | ⚠️ | Codec implemented; not exercised on hardware       |
| YC1000  | (multi-byte)              | ⚠️ | ⚠️ | ⚠️ | ⚠️ | Decoder present; needs on-hardware validation       |
| QT2     | `0x29 0x30 0x31 0x32`     | ⚠️ | ⚠️ | ⚠️ | ⚠️ | Decoder present; needs on-hardware validation       |

✅ live-validated on real hardware · ⚠️ implemented but not yet exercised on a real device

## Features

**In v1.0:**

- Live per-inverter telemetry (panels, AC/DC, RSSI, lifetime energy) — no polling delay, no cloud round-trip
- Grid-protection profile management: select base profile (e.g. `EN 50549-1`), apply per inverter, verify on read-back, audit
- Per-inverter and array-wide output-power capping; works with Victron frequency-shift curtailment
- OTA pairing: fast/slow scan, add-by-ID, replace-me (inherits the dead inverter's grid profile and operator label), full fleet PAN re-key, channel migration
- SunSpec / Modbus TCP (port `502`, the IANA-standard Modbus port) — models 101/103/111/113/123/711 etc., consumed cleanly by Home Assistant and other EMS
- HTTPS operator console on `:443` with operator-password auth + single-use recovery code + change-password
- Encryption badge per inverter (AES vs plaintext frame detection)
- L1 OTA AES-128 codec ⚠️ experimental, opt-in via `-enable-aes-l1` (decrypt + encrypt primitive implemented per [`docs/AES-DESIGN.md`](docs/AES-DESIGN.md); no on-wire test vectors available on the maintainer's fleet — see the design doc's validation-gap note, and the AES threat model below)
- Audit event log with `by` attribution for every settings/profile/power-cap change
- Rollback CLI restores the original stock firmware from a backup snapshot

### AES L1 Encryption (EXPERIMENTAL)

`inv-driver` supports AES-128-ECB L1 over-the-air decryption for encrypted
inverter telemetry frames (opt-in via `-enable-aes-l1`). This cipher is
reverse-engineered from APsystems firmware and **is not a new security
boundary**:

- The cipher **mode and key derivation are wire-mandated** by the inverter
  firmware (`AES_flag_ALL=1` at runtime). We do not control the algorithm
  choice.
- The per-frame **random nonce is sourced from `crypto/rand`** for each
  transmitted frame; no static key is involved on the L1 OTA path.
- **No integrity protection** (MAC/HMAC) exists on encrypted frames; an
  attacker on the LAN can inject or modify ciphertext. The firmware itself
  relies only on the L2 frame structure (`FB FB ... FE FE`) as a sanity
  check, which this codec mirrors.
- **Gate-byte collision:** approximately 6.25% of random nonces occupy the
  plaintext indicator band (`[0xF0, 0xFF]`) and would be mis-classified as
  plaintext on reception. These frames fail L2 parse downstream and are
  dropped safely (not a security issue, graceful degradation).
- **Operator trust assumption:** the LAN is assumed trusted. AES-ECB
  without a MAC is susceptible to known-plaintext and chosen-ciphertext
  attacks if an attacker can inject frames or observe responses.

**Do NOT enable `-enable-aes-l1` on untrusted networks.** It is suitable
only for closed local networks (home solar arrays, building networks)
where RF eavesdropping is the only realistic threat.

**Deliberately not included (and not going to be):**

- Cloud uplink to `apsystemsema.com` / `.cn` — local-only by design
- The stock CodeIgniter web UI on `:80` (stock) — removed during install, replaced by OpenAPS on `:443`

**Open / on the roadmap:**

- Signed OTA upgrades via `POST /api/upgrade` (placeholder key ships in `v1.0.0`; production key + verify code land in `v1.0.x`)
- Raspberry Pi greenfield target — arm64 `.deb`, `systemd` units, mDNS `openaps.local`, captive-portal first-boot (v2)
- Generic CC2652P / Sonoff / ConBee USB radio support via a `ti-znp-zb` bus-manager backend (v2 — required before generic Pi is meaningfully useful)
- Per-device signing key option (v1.0 uses a single release keypair)
- WebAuthn / passkeys (deferred until a real DNS hostname exists)

## Install

> v1.0 supports brownfield migration of an existing APsystems **ECU-R-Pro** or **ECU-C**. The RTOS-based ECU-R / ECU-B are NOT compatible (no Linux userspace). Pi support is on the roadmap.

**1. Download the installer tarball** from the [latest release](https://github.com/bolkedebruin/openaps/releases/latest):

```
openaps-v1.0.0-ecu.tar.bz2
```

**2. Push it to your ECU** over the stock firmware's existing local-upgrade endpoint:

```sh
curl -F "file=@openaps-v1.0.0-ecu.tar.bz2" \
     http://<ECU-IP>/index.php/management/exec_upgrade_ecu_app
```

The orchestrator script inside the tarball:

- Takes a full backup of `/home/applications`, `/etc/rcS.d`, `/etc/init.d`, `/etc/yuneng`
- Installs SSH (`dropbear`) **first** as a recovery path and verifies it's listening
- Installs the four OpenAPS binaries + `S48..S99` init scripts
- Disables every stock APsystems supervisor (cloud uplink, unauthenticated stock endpoints, the broken `mqtt.exe` CPU-spinner, the auto-update path) by moving their binaries out of the manager's launch path
- Removes the stock `lighttpd` / CodeIgniter web UI **only after** SSH is confirmed reachable
- Starts OpenAPS in dependency order and smoke-checks the result

**3. Open `https://<ECU-IP>/`** in a browser, accept the self-signed cert, set an operator password, and start using the console.

**Roll back at any time** with:

```sh
ssh root@<ECU-IP> /usr/local/bin/openaps-rollback
```

> The bundled `dropbear` predates the algorithms modern OpenSSH ships enabled by default — drop the snippet from [`docs/SSH-CONFIG.md`](docs/SSH-CONFIG.md) into your `~/.ssh/config` on first connection.

### What's preserved when you migrate from stock firmware

The install is a near-zero-config drop-in. Inverters, grid profile, and power caps come back automatically within a few minutes of first boot. The matrix:

| Thing | Inherited? | How |
|---|:--:|---|
| ZigBee PAN, channel, ECU ID, ECU MAC | ✅ | Read from `/etc/yuneng/*.conf` during install. Same PAN your fleet is paired against. |
| Paired inverter inventory | ✅ | Auto-discovered from the first telemetry frame each inverter sends after the radio comes up — usually within 1-3 minutes. |
| Active grid profile per inverter | ✅ | Reverse-identified from the inverter's own protection-register read-back (e.g. matches `EN 50549-1` against shipped base profiles). |
| Output power caps | ✅ | Persisted in inverter NVRAM (DA code, both DS3 and QS1A), read back on first contact. |
| Encryption state (AES vs plaintext badge) | ✅ | Detected from frame gate byte on first sight. |
| Inverter friendly names | ❌ | Stock keeps these in `/home/database.db`; you re-label via the inverters table on first browse. |
| Historical pre-install energy timeseries | ❌ | Stock per-day/per-month/per-year history isn't imported. The backup tarball preserves `/home/database.db` so a future importer is feasible. Per-inverter lifetime counters on the inverter itself are unaffected — totals stay correct. |
| Operator account (web UI password) | ❌ | Fresh install prompts you to set a new password + generates a one-time recovery code. |

Full install reference: [`docs/INSTALL-ECU.md`](docs/INSTALL-ECU.md). Release notes: [`docs/RELEASE.md`](docs/RELEASE.md).

## Build from source

Requires Go 1.26+ and [Bun](https://bun.sh) for the web UI bundle.

```sh
git clone https://github.com/bolkedebruin/openaps
cd openaps
make web                   # build the SPA bundle (Bun)
make build-all-arm         # cross-compile the four ARMv7 binaries
make package-openaps       # produce the brownfield ECU installer tarball
```

Output lands in `build/`. `make test` runs the full suite (`go test -race ./...` + `bun test`).

## License

[MIT](LICENSE).

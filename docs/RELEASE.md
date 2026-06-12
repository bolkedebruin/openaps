# OpenAPS v1.1.11

Fixes the built-in grid profiles being absent on a fresh install.

## Fixed

- **The base grid profiles now ship in `openaps-inv-driver`.** inv-driver
  loads its base profiles from `/var/lib/inv-driver/gridprofiles/profiles` at
  startup, but the package never populated that directory — so a fresh install
  showed an empty Profiles screen. The package now ships the 64 built-in
  profiles (EN 50549-1, AS/NZS 4777.2, ABNT, …) into that directory. They are
  package data (refreshed on upgrade), not conffiles; operator overlays are
  stored separately and are never touched.

## Upgrading

Install the `.ipk` packages from this release over the opkg feed as usual.
Upgrading `openaps-inv-driver` drops the base profiles into place and reloads
them on restart. No configuration or schema changes.

---

# OpenAPS v1.1.10

Adds an NTP time-sync package for the ECU.

## Added

- **`ntpdate` opkg — keeps the ECU clock synced.** The device has no cron and
  no battery-backed RTC, so its clock drifts and resets to the last-shutdown
  time across reboots — which skews logs and TLS certificate validity. This
  packages the Debian-wheezy `ntpdate` binary (SHA256-pinned from
  snapshot.debian.org, no cross-compilation) as a bare `ntpdate` package. An
  rcS init steps the clock with `ntpdate -b` against the Debian NTP pool at
  boot and hourly; the server list is an operator-editable conffile at
  `/etc/ntpdate/servers.conf`. It runs as a transient root one-shot with no
  privilege separation, so it needs no extra users or libraries.

## Upgrading

Install the `.ipk` packages from this release over the opkg feed as usual; see
`UPGRADING.md`. To enable time sync, install the new package:
`opkg install ntpdate`. It is feed-installable only (not part of the
bootstrap). No configuration or schema changes.

---

# OpenAPS v1.1.9

Reliability release: a runtime radio watchdog so the ECU recovers from a
wedged ZigBee module on its own, and an ecu-web fix so an expired session no
longer looks like an inv-driver outage.

## Added

- **ecu-zb radio liveness watchdog.** The radio bring-up (hardware reset +
  Set-PANID) previously ran only at startup, and the splice's modem read has no
  deadline — so a CC2530/UART that wedged after the fleet slept overnight stayed
  dead until a manual reboot. The watchdog now monitors inbound activity; when
  the bus goes silent past a threshold it pings the local module (0x0D) through
  the existing pairing path, and re-arms the radio (hardware reset + Set-PANID)
  only if the module fails to ack. A healthy module acks even when every
  inverter is asleep, so night silence is never mistaken for a fault; a cooldown
  prevents reset storms, and on a healthy site it is a complete no-op. Active
  only with an inv-driver-backed radio and a known operating PAN.

## Fixed

- **ecu-web: an expired session drops to the login view instead of showing
  "inv-driver offline".** Operator sessions are held in memory, so restarting
  ecu-web invalidates a browser's cookie. The UI kept polling auth-gated
  endpoints, got 401s, and swallowed them — leaving a stale dashboard whose
  clients card read inv-driver as offline even though it was healthy. A 401 from
  any request now prompts re-authentication. Password step-up confirms still
  treat 401 as "wrong password" and do not log the operator out.

## Upgrading

Install the `.ipk` packages from this release over the opkg feed as usual; see
`UPGRADING.md`. No configuration or schema changes. The watchdog needs no
configuration — it activates automatically on ecu-zb where inv-driver drives
the radio.

---

# OpenAPS v1.1.8

Dependency security release. `govulncheck` reported three vulnerabilities
reachable from deployed code; all are fixed by this release and the scan is
now clean. No functional changes.

## Security

- **Go 1.26.4.** Fixes `net/textproto` unescaped error inputs (GO-2026-5039)
  and inefficient `crypto/x509` hostname parsing (GO-2026-5037), both
  reachable through `openaps-tls-proxy`'s HTTP server.
- **golang.org/x/crypto v0.52.0.** Fixes a DoS in `x/crypto/ssh` on
  pathological RSA/DSA parameters (GO-2026-5018), reachable through
  recoveryd's authorized-keys parsing.

## Changed

- `golang.org/x/sys` v0.46.0, `modernc.org/sqlite` v1.51.0.
- ecu-web frontend: `lit` 3.3.3, `@happy-dom/global-registrator` 20.10.1
  (transitive `happy-dom` pinned in `overrides` to clear the 7-day
  dependency-cooldown gate). The shipped bundle is byte-identical to
  v1.1.7's.

## Upgrading

Install the `.ipk` packages from this release over the opkg feed as usual;
see `UPGRADING.md`. No configuration or schema changes.

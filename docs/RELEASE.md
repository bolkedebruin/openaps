# OpenAPS v1.1.18

Power chart survives a wrong ECU clock.

## Fixed

- **The power chart no longer goes blank after the ECU clock was wrong.**
  The chart spans its x-axis from the first to the last sample, so a
  single point recorded while the clock was unset (e.g. `2000-01-01`
  after an RTC loss) stretched the axis across decades and collapsed the
  real data into an invisible sliver. The power-history ring now drops
  samples with an implausible timestamp (before 2020) both when loading
  its persisted file and when recording new samples, so a clock glitch
  self-heals once the clock is corrected — no manual cleanup of
  `power-history.json` needed.

## Upgrading

`opkg upgrade openaps-inv-driver`. No configuration or schema changes.
An ECU whose chart is already poisoned heals on the next restart after
upgrading (the bad points are filtered on load); the lifetime energy
totals in the database were never affected.

---

# OpenAPS v1.1.17

Decodes QS2 (4-channel) inverter telemetry.

## Added

- **QS2 telemetry decoding.** QS2 (model `0x36`) reports four DC
  channels and shares the DS3-class reply command (`0xBB`) with DS3, so
  a telemetry reply is now dispatched by inverter model code rather than
  the command byte alone. QS2 decodes all four channels (A–D) and their
  lifetime accumulators; previously it fell through the DS3 decoder and
  surfaced only two. The model code is sourced from each inverter's info
  reply, and QS2 telemetry now reports its model as `QS2`. DS3 and QS1A
  decoding are unchanged.

## Upgrading

`opkg upgrade openaps-inv-driver`. No configuration or schema changes.

---

# OpenAPS v1.1.16

Recognizes the QS2 inverter and corrects nameplate ratings.

## Added

- **Model code `0x36` is now identified as QS2** (single-phase
  4-channel) and classified into the DS3 family, so it uses the
  DS3-class wire protocol for encoding, broadcast, and grid-profile
  pushes. Model strings prefixed `QS2` map to the DS3 family as well.

## Changed

- **Nameplate ratings now follow the APsystems datasheet "maximum
  continuous output power"** (EMEA 230 V / 50 Hz), not short-term peak.
  QS2 is 2200 VA; QS1A 1600→1500, DS3 750→880, DS3H 880→960, DS3L
  600→730, QT2 1800→2000; the YC600 family is 550 and YC1000 is 900.
  These feed SunSpec Model 120 (Nameplate Ratings) fleet sums and cap
  the per-inverter power slider.

## Upgrading

`opkg upgrade openaps-inv-driver`. No configuration or schema changes.

---

# OpenAPS v1.1.15

Lets operators add inverters while telemetry is running.

## Fixed

- **Adding inverters no longer fails with "ZigBee bus is busy with
  telemetry-poll".** Pairing ops (add-by-serial, scan, replace, rekey,
  change-channel) already paused telemetry via the shared bus lock, but with a
  non-blocking acquire — and the steady-state telemetry poller re-takes that
  lock every round. On larger fleets a poll round outgrows the 1 s interval, so
  the poller held the lock almost continuously and the operator op's acquire
  always lost the race, getting "busy with telemetry-poll." The bus lock now
  gives operator ops priority: they preempt the poller (which yields within one
  telemetry send), so commissioning a full fleet no longer means retrying every
  add. The poller's preemption rides a single revocation signal, so the
  coordination is robust as the radio layer grows.

## Upgrading

`opkg upgrade openaps-inv-driver`. No configuration or schema changes.

---

# OpenAPS v1.1.14

Documentation fix for the SunSpec adapter, plus a Home Assistant curtailment guide.

## Fixed

- **ecu-sunspec README rewritten to the current write path.** The Modbus
  write-controls, latency, and verify-a-write sections still described the
  removed pre-monorepo SQLite path (`UPDATE power SET limitedpower/flag`, "expect
  30-300 s for the dispatch poll"). They now document the real path: a SunSpec
  Model 123 write is encoded per inverter and dispatched through inv-driver to
  the radio in a few seconds. Data-source, grid-protection sourcing, and
  per-panel-cap notes corrected to inv-driver's live state.
- **`ecu-sunspec -config` help and config loader comments corrected.** Both
  claimed "missing file = writes disabled"; the tristate default makes a missing
  `/home/sunspec.json` enable writes for loopback and the local LAN.

## Added

- **"Curtailing from Home Assistant" guide.** Documents setting the output cap
  from HA via the built-in `modbus` integration: discovering the `WMaxLimPct`
  register, an `input_number` slider, and a `modbus.write_register` automation
  (slave ID 1 curtails the whole fleet, 2..N+1 a single inverter).

## Upgrading

Documentation only — no binary or package changes.

---

# OpenAPS v1.1.13

Fixes the ECU clock not being set after a reboot.

## Fixed

- **ntpdate now reliably sets the clock at boot.** Two issues on this
  battery-RTC-less ECU: (1) a cold boot starts at year 2000, and `ntpdate -b`
  could fail to step a multi-year gap (`step-systime: Invalid argument`) — the
  init now nudges an implausibly-old clock to the last-known-good
  `/etc/timestamp` first, so ntpdate only steps a small offset; (2) the init ran
  *before* `S55bootmisc.sh`, which calls `hwclock --hctosys` and reset the clock
  back to the dead RTC right after ntpdate had corrected it, leaving the box at
  year 2000 for up to an hour. The init is renumbered `S56-ntpdate` so it runs
  after bootmisc as the final boot-time clock setter.

## Upgrading

`opkg upgrade ntpdate` (or reinstall it). The upgrade replaces the old
`S46-ntpdate` init with `S56-ntpdate` and restarts the sync loop. No other
changes.

---

# OpenAPS v1.1.12

Adds a helper to migrate ECUs from the 1.0.X release to 1.1.X.

## Added

- **`openaps-unstub-stock.sh` — 1.0.X → 1.1.X migration helper (release asset).**
  1.0.X suppressed the stock firmware with in-place sleeper stubs; 1.1.X disables
  it cleanly at the manager level (the `apsystems-stock` package). This script
  converts a 1.0.X box to the 1.1.X layout without opkg: it restores every
  stubbed `<app>.real` binary and comments the stock manager launch in
  `S50ecu_init` (the exact state `opkg remove apsystems-stock` produces),
  starting nothing. It refuses unless persistent SSH (`openaps-dropbear`) is
  installed, so it can't cause a lockout. Idempotent; `--dry-run` supported.
  Shipped as a standalone asset (with a SHA256SUMS entry), not in the bootstrap.

## Migrating from 1.0.X

See **`docs/MIGRATION-1.0-to-1.1.md`** for the full procedure. In short, on the
ECU as root: (1) `opkg install openaps-dropbear` first — its postinst starts
persistent, manager-independent SSH immediately (this release includes the
`openaps-dropbear` ipk); (2) run `openaps-unstub-stock.sh`; (3) install the rest
of the 1.1.X packages (not `apsystems-stock`); (4) reboot.

## Upgrading

Install the `.ipk` packages from this release over the opkg feed as usual. No
configuration or schema changes.

---

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

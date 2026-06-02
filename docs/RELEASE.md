# OpenAPS v1.0.2

Patch release in the `v1.0.x` stream. Same brownfield-ECU installer mechanism as `v1.0.1`; **no breaking changes for operators upgrading from `v1.0.1`**. The big-ticket additions are an experimental L1 OTA AES codec (opt-in) and a codec-API cleanup. CI is meaningfully more robust.

## What's new

### L1 OTA AES codec — EXPERIMENTAL, opt-in

OpenAPS now ships the per-frame-keyed AES-128-ECB scheme reverse-engineered from APsystems stock firmware (see [`docs/AES-DESIGN.md`](AES-DESIGN.md)). The decoder is wired and gated behind a daemon flag:

```sh
inv-driver serve -enable-aes-l1 ...
```

When enabled, encrypted L1 frames (gate byte `< 0xF0`) are transparently decrypted before the family resolvers see them. The `Encrypted` badge per inverter keeps its meaning ("this frame was AES on the wire").

**Caveats:**
- The encrypt primitive (`codec.EncryptTX`) is shipped and unit-tested but is NOT wired into ecu-zb's outbound send path yet. Inbound decrypt only.
- No on-wire test vectors exist on the maintainer's fleet (all-plaintext). Correctness is by-construction against the Ghidra-derived spec + algorithmic round-trip tests. First operator with a `'2'`-prefix fleet is the proof-test.
- No integrity protection. Don't enable on networks you don't control — see the AES threat-model block in the README.

### Codec API cleanup — `Family` enum + `ExtendedStatus` interface

`codec.Reply.Model string` is gone. Replaced by `Reply.Family codec.Family` (an enum), with `Family.String()` at presentation boundaries. The two per-family status fields (`Reply.DS3Status` + `Reply.QS1AStatus`) collapse into a single `Reply.ExtendedStatus ExtendedStatus` interface that DS3/QS1A and future YC600/YC1000/QT2 implementations satisfy. Adding a new family no longer grows `Reply`.

Wire format unchanged. `wire.Telemetry.Model` (proto string) is preserved byte-for-byte via `Reply.ModelLabel()`.

### CI / test hardening

- `internal/ingest`: `online` tracker is now lazy-initialised via `sync.Once` (matches the existing `faultsOnce` pattern). Fixed a data race two concurrent telemetry connections could trigger.
- gridprofile tests: dropped a supersession-race in `TestReconcileAllOverlays` (overlays now target disjoint UIDs), drained the async applier in `TestSetOverlay_AcceptsAllUidsInFleet` before `t.TempDir` cleanup. Race + 20× count clean locally.
- gridprofile `stubRunner.calls` migrated to `atomic.Int64` (was `sync.Mutex`-guarded for the write but read unlocked).
- Bumped `actions/checkout@v4 → v5` (Node.js 24 readiness).

### Docs

- README hero strip — three Playwright-captured screenshots (synthetic-fleet, no real serials/MAC/PAN leak): dashboard, inverters, profiles. Plus `docs/screenshots/` with events, settings, alarms, login.
- New `docs/AES-DESIGN.md` — frame layout, key derivation, padding scheme, Ghidra references.
- New `docs/SSH-CONFIG.md` — minimum legacy-algorithm opt-in for OpenSSH 9.x against the bundled dropbear 2012.55.
- README + INSTALL-ECU now spell out the supported hardware (ECU-R-Pro + ECU-C) and unambiguously rule out the RTOS-based ECU-R / ECU-B.
- License link fixed to MIT (it's always been MIT).

## Compatibility matrix unchanged from v1.0.1

| Family | Telemetry | Grid profile | Set-power | Pairing |
|---|:--:|:--:|:--:|:--:|
| DS3 | ✅ | ✅ | ✅ | ✅ |
| QS1A | ✅ | ✅ | ✅ | ✅ |
| QS1 | ✅ | ✅ | ✅ | ✅ |
| DSP4 | ✅ | ✅ | ✅ | ⚠️ |
| YC600 | ✅ | ✅ | ✅ | ⚠️ |
| YC1000, QT2 | ⚠️ | ⚠️ | ⚠️ | ⚠️ |

## Upgrade

```sh
curl -F "file=@openaps-v1.0.2-ecu.tar.bz2" \
     http://<ECU-IP>/index.php/management/exec_upgrade_ecu_app
```

Or, if you already have `v1.0.x` installed and SSH access, just `make deploy-all ECU_HOST=root@<ECU-IP>` from a checkout at the `v1.0.2` tag and restart the four services.

## What's deferred to v1.0.x

- **Signed-tarball OTA** — `release.pub` ships at `/etc/openaps/release.pub` but `/api/upgrade` is not yet plumbed. The placeholder key generated in v1.0.0 needs replacing with a production keypair (kept offline) before signing matters.
- **Passkeys** — still blocked on WebAuthn rpId / PSL constraints; needs a real DNS name.
- **AES encrypt wire-up in ecu-zb** — primitive ready; production wiring blocked on an on-wire test vector.

## What's deferred to v2

- Pi / .deb / systemd packaging.
- `bus-mgr ti-znp-zb` backend for off-the-shelf USB ZigBee radios.

## Artifacts

- `openaps-v1.0.2-ecu.tar.bz2` — brownfield installer.
- `SHA256SUMS`.

## Install caveats

Install only on LANs you control.

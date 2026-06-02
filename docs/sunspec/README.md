# apsystems-sunspec

A SunSpec / Modbus TCP adapter for APsystems ECU gateways (ECU-R, ECU-R-Pro, ECU-C, etc.). Exposes the data the ECU already collects from your microinverters as a standards-compliant SunSpec register bank, so Victron GX, Home Assistant, Grafana, Node-RED, and any other SunSpec consumer can read it natively.

The adapter reads the ECU's own SQLite databases and `/tmp/parameters_app.conf`. No effect on the ECU's radio cycle and no cloud round-trip. Runs on the ECU itself or as a sidecar.

## Compatibility

The adapter requires a Linux userspace on the ECU (BusyBox + writable `/home/applications/`) and reads tables from APsystems' `/home/database.db`. ECU models that run a bare-metal RTOS instead of Linux can't host the adapter — point your SunSpec client at APsystems' built-in port 502 in that case.

| ECU model | Serial prefix | Platform | This adapter | APsystems :502 (per their Rev 3.3 spec) |
|---|---|---|---|---|
| ECU-R-M3 | `2160xxxxxxxx` | Cortex-M3 + RT-Thread / lwIP 1.4.1 (RTOS, single-image firmware) | no — no userspace | yes: Common, 101, 111, 123 (firmware ≥ 1.3.7) |
| **ECU-R-Pro** | `2162xxxxxxxx` | **Linux ARMv7** (BusyBox) | **yes — tested on fw 2.1.29D** | yes, same minimal PICS (firmware ≥ 2.0.2) |
| ECU-B | `2163xxxxxxxx` | (RTOS, same family as ECU-R-M3) | no — no userspace | no — APsystems explicitly disables Modbus on this model |
| ECU-C | `215xxxxxxxxx` | Linux | yes — untested but expected to work, same Yuneng userspace | yes: Common, 101, 111, 123 (firmware ≥ C1.1.3) |
| ECU-3 / ECU-3Z | various | Linux | yes — untested | unknown |

Notes:
- "Tested" means I run this build daily on the listed firmware. Other Linux-based ECUs share the same userspace lineage (Yuneng platform, sqlite-backed `/home/database.db`, BusyBox `/etc/init.d/S99-*` script convention) so the adapter should work without code changes — open an issue if you find otherwise.
- The Cortex-M3 / RT-Thread ECUs (2160 / 2163) ship a single firmware blob that boots from MCU flash at `0x08000000` — there's nowhere to install a Go binary. APsystems' own port-502 server on those models implements only the minimal SunSpec PICS (4 models); the richer 700-series DER models, Multi-MPPT, nameplate, and float variants this adapter exposes simply aren't reachable on those ECUs.
- A sidecar deployment that proxies a 2160's port 502 is possible but limited — a sidecar machine can only republish what :502 already serves; it can't read the per-inverter tables that aren't exposed over Modbus.

## What it exposes

| Modbus unit ID | Bank | Why |
|---|---|---|
| **1** | Aggregate: `Common + Inverter (101) + Nameplate (120) + Basic Settings (121) + Controls (123) + DER Trip LV/HV/LF/HF (707/708/709/710) + Enter Service (703) + Multi-MPPT (160 with every panel) + Vendor (64202) + End` | System-level totals; what Victron's GX polls |
| **2..N+1** | Per-microinverter: `Common (SN = inverter UID) + Inverter (101) + Controls (123) + DER Trip + Enter Service + Multi-MPPT (160 with that inverter's panels) + End` | Per-inverter dashboards in HA / Grafana |

Each microinverter shows up in HA's SunSpec integration as an independent device. Per-panel data lives in Multi-MPPT (Model 160) — module count derives from the type code in `parameters_app.conf` (DS3: 2, QS1: 4, DS3-H: 2, YC1000 / QT2: 4).

Standard SunSpec event flags are populated from the ECU's own alarm bitstring: ground fault, over-temperature, AC over/under voltage, over/under frequency, manual shutdown, AC disconnect, grid disconnect (anti-island trip), HW test failure. Raw APsystems bits remain in `EvtVnd1..3` for full fidelity. See [`docs/EVENTS.md`](docs/EVENTS.md) for the complete bit table.

### IEEE 1547-2018 grid protection (read-only)

Models **707/708/709/710** expose the active per-inverter trip thresholds — the LV/HV/LF/HF curves the firmware will actually disconnect on. **Model 703** (Enter Service) exposes the reconnect window: V/Hz band the grid must hold for `ESDlyTms` seconds before the inverter rejoins. Sourced from `protection_parameters60code` in `database.db`, refreshed each ZigBee cycle.

This is the SunSpec model set adopted by SMA ennexOS, Fronius GEN24, Enphase, and the IEEE 1547-2018 conformance profile — current-generation DER tooling reads it natively. Older ride-through curve models (129/130/135/136) are not emitted.

Useful for confirming whether your fleet supports Victron AC-coupled frequency-shift: read `Model 710 → Crv[0].MustTrip.Pt[1]` per inverter; the Hz threshold has to sit above the Multi's max FS frequency (typically 52 Hz) with at least 0.5 Hz margin, and the clearance time has to exceed the longest dwell at high frequency. On a mixed fleet the inverter with the lowest OF threshold gates the whole system.

Currently read-only. The active settings can already be changed via the existing PHP `management/set_protection60_parameters` endpoint or via a future Modbus write path gated by `writes.allow_grid_protection` (not implemented).

## Verify it's working

A quick sanity check against any SunSpec scanner. Using `pysunspec2`:

```sh
pip install pysunspec2 pyserial
python3 -c "
import sunspec2.modbus.client as c
d = c.SunSpecModbusClientDeviceTCP(slave_id=1, ipaddr='<ECU-IP>', ipport=1502, timeout=3)
d.scan()
for m in d.model_list: print(m.model_id, m.model_len)
"
```

Expected: `1, 101, 120, 121, 123, 707, 708, 709, 710, 703, 160, 64202` for unit 1.

## Building

```sh
make ecu      # ARMv7 binary for the ECU itself (~8 MB, statically linked)
make sidecar  # x86_64 binary for a sidecar host (Synology, generic Linux server)
make mac      # local development build
make test     # unit + integration tests
```

The build is pure Go (`CGO_ENABLED=0`) so the ARMv7 binary is glibc-version-independent and runs on the ECU's old userland.

### Pre-push hooks (optional)

`.pre-commit-config.yaml` ships [prek](https://github.com/j178/prek)-compatible hooks that mirror the CI workflow — gofmt, go vet, go test — so a failed push is caught locally instead of in GitHub Actions:

```sh
brew install prek          # or `pip install pre-commit` if you prefer
prek install --hook-type pre-push
```

Hooks then run automatically on every `git push`. Same config works with stock `pre-commit`.

## Installing on the ECU

The ECU exposes a local web endpoint that accepts a `tar.bz2` package and runs an embedded `assist` script after extraction. The Makefile builds the right shape of package directly.

### 1. Build the package

```sh
make package
# produces dist/apsystems-sunspec-<version>.tar.bz2
```

### 2. POST it to the ECU

```sh
curl -X POST -H "Expect:" -F file=@dist/apsystems-sunspec-<version>.tar.bz2 \
     http://<ECU-IP>/index.php/management/exec_upgrade_ecu_app
# `-H "Expect:"` disables curl's automatic 100-continue header, which the
# ECU's lighttpd rejects with 417.
```

The PHP handler extracts the tarball into `/home/update_from_app/` and runs `update_localweb/assist`. The script:

- copies `ecu-sunspec` to `/home/applications/`
- installs `/etc/init.d/S99-sunspec` (auto-start on boot)
- restarts the adapter

A log of the install lands in `/home/sunspec-install.log` on the ECU.

### 3. Confirm

```sh
nc -zv <ECU-IP> 1502   # should connect
```

…and re-run the pysunspec2 verify above.

### Including dropbear (SSH on the ECU)

To bundle a dropbear SSH server in the same install package:

```sh
# 1. Fetch dropbear binaries that match the ECU's glibc generation.
make fetch-dropbear
# downloads dropbear 2012.55 from Debian wheezy's archive
# and stages dist/dropbear-armv7/{dropbear,dropbearkey,dropbearconvert,dbclient}

# 2. (optional) install your SSH public key for root.
cp ~/.ssh/id_rsa.pub dist/dropbear-armv7/authorized_keys

# 3. Build the package.
make package-with-dropbear DROPBEAR_DIR=dist/dropbear-armv7
```

Why these specific binaries: the ECU's userland is glibc 2.15 / armhf, the same generation as Debian wheezy. dropbear 2012.55 from wheezy's security pocket links cleanly. Newer dropbear builds (Debian bookworm and later) require a glibc that the ECU doesn't have.

If you'd rather use your own pre-built dropbear, skip step 1 and pass `DROPBEAR_DIR=` pointing at any directory with `dropbear` + `dropbearkey` (and optional `authorized_keys`).

The resulting `apsystems-sunspec-<version>-dropbear.tar.bz2` adds an `S98-dropbear` init script so the SSH daemon comes up automatically on each boot.

#### Connecting to the bundled dropbear

dropbear 2012.55 is a vintage build. Modern OpenSSH clients refuse the algorithms it speaks by default — you have to opt in. Three things to know:

1. **Host key algorithm**: dropbear 2012.55 only offers `ssh-rsa`. OpenSSH 8.5+ deprecated it. Re-enable with `HostKeyAlgorithms +ssh-rsa`.
2. **User key algorithm**: same story — `PubkeyAcceptedAlgorithms +ssh-rsa`. ed25519 keys won't work; use an RSA key.
3. **No `sftp` / `scp` subsystem**: dropbear 2012.55's distribution doesn't include the SFTP server. Use `ssh ecu cat /file > local` or the bundled `dbclient` instead of `scp`.

A workable `~/.ssh/config` entry:

```
Host ecu
    HostName <ECU-IP>
    User root
    IdentityFile ~/.ssh/id_rsa
    HostKeyAlgorithms +ssh-rsa
    PubkeyAcceptedAlgorithms +ssh-rsa
    # Some networks need this to avoid the "no matching key exchange method" error:
    KexAlgorithms +diffie-hellman-group14-sha1,diffie-hellman-group1-sha1
```

First-time connection accepts the host key, then `ssh ecu` works. To copy a file *to* the ECU when scp isn't available:

```sh
cat local-file | ssh ecu "cat > /target/path && chmod +x /target/path"
```

To copy *from* the ECU:

```sh
ssh ecu "cat /home/sunspec-install.log" > install.log
```

If your `id_rsa` is passphrase-encrypted and you want to use a dedicated unencrypted key just for the ECU (recommended — limit scope), generate a throwaway RSA key:

```sh
ssh-keygen -t rsa -b 4096 -N "" -f ~/.ssh/ecu_id_rsa
# then in your ~/.ssh/config:
#   IdentityFile ~/.ssh/ecu_id_rsa
# and ship ecu_id_rsa.pub as authorized_keys in the dropbear bundle.
```

## Running as a sidecar

If you don't want to deploy on the ECU, run the binary anywhere with read access to the ECU's `/home` (NFS, rsync mirror, SSHFS) and `/tmp/parameters_app.conf`:

```sh
./ecu-sunspec \
    --bind tcp://0.0.0.0:1502 \
    --db-dir /mnt/ecu/home \
    --params-file /mnt/ecu/tmp/parameters_app.conf \
    --yuneng-dir /mnt/ecu/etc/yuneng
```

All configuration is via flags — see `--help`.

## Adding to Home Assistant

Use the [CJNE/ha-sunspec](https://github.com/CJNE/ha-sunspec) custom component (HACS-installable). Add the integration up to four times:

```
Host: <ECU-IP>   Port: 1502   Slave ID: 1   →  System aggregate
Host: <ECU-IP>   Port: 1502   Slave ID: 2   →  Microinverter A
Host: <ECU-IP>   Port: 1502   Slave ID: 3   →  Microinverter B
Host: <ECU-IP>   Port: 1502   Slave ID: 4   →  Microinverter C
```

For the vendor model (model 64202 — daily/month/year energy aggregates, per-inverter RSSI, etc.) to be decoded natively, copy [`sunspec-models/model_64202.json`](sunspec-models/model_64202.json) into pysunspec2's `models/json/` directory inside the HA container. See [`sunspec-models/README.md`](sunspec-models/README.md) for the path.

## Adding to Victron Venus

```
Settings → PV inverters → Find PV inverters
```

Pick the entry that auto-discovers at `<ECU-IP>:1502` (the aggregate, slave ID 1). When prompted, choose **Position = AC out** (if your microinverters are downstream of the Multi for AC-coupled freq-shift control) and **Phase = L1** for single-phase setups.

If Venus' driver gets stuck after a binary upgrade — the standard fix is to toggle the inverter's "Show in overview" off and on again from the GX UI, which forces a driver reconnect.

## Modbus write controls (SunSpec Model 123)

Out-of-the-box the package ships with **writes enabled for any host on the same LAN as the ECU**. That's the most common case (HA, Cerbo, your laptop all on `10.25.1.0/24`). To turn writes off or restrict them further, edit `/home/sunspec.json` on the ECU.

### Default config (shipped at `/home/sunspec.json`)

```json
{
    "writes": {
        "enabled": true,
        "allow_local_network": true,
        "allow_list": []
    }
}
```

| Field | Default | Effect |
|---|---|---|
| `enabled` | `true` | Master switch. `false` = all writes rejected with `ErrIllegalFunction`. |
| `allow_local_network` | `true` | Auto-allow any IP in the same subnet as one of the ECU's interfaces. |
| `allow_list` | `[]` | Extra IPs or CIDRs allowed beyond loopback + local network. |

To restrict to two specific hosts and disable the broad LAN allow:

```json
{
    "writes": {
        "enabled": true,
        "allow_local_network": false,
        "allow_list": ["10.25.1.29", "10.25.1.21"]
    }
}
```

To disable writes entirely (read-only deployment):

```json
{ "writes": { "enabled": false } }
```

`assist` only installs the default config when `/home/sunspec.json` doesn't already exist — re-installing the package preserves your custom settings.

### What can be written

| SunSpec field (Model 123) | Effect | Mapped to |
|---|---|---|
| `WMaxLim_Pct` (0..100) | Per-panel cap = `500 W × pct/100` (clamped to `[20, 500]`). Per-inverter banks affect one inverter; aggregate bank affects all. | `UPDATE power SET limitedpower=…, flag=1` (per-panel watts) |
| `WMaxLim_Ena=0` | Restore full output (`limitedpower = 500`, the ECU's uncapped sentinel) | same table, `limitedpower = 500` |
| `Conn=0` | Turn inverter(s) off | `INSERT turn_on_off VALUES(uid, 0)` |
| `Conn=1` | Turn inverter(s) on | `INSERT turn_on_off VALUES(uid, 1)` |

The per-panel cap is clamped to the same `[20, 500]` W range the ECU's own PHP `set_maxpower` endpoint enforces. `WMaxLim_Pct` values above 100 are clamped to 100; values that would resolve to under 20 W (including `WMaxLim_Pct=0`) are raised to 20 W. **`WMaxLim_Pct=0` is not "off"** — it's "minimum cap." Use `Conn=0` to actually turn an inverter off.

> "% of nameplate" is the SunSpec semantic, but on this hardware the underlying knob is the per-panel watt cap, not AC nameplate. `WMaxLim_Pct=100` resolves to 500 W/panel — the ECU firmware's hard ceiling, also the value the upstream `homeassistant-apsystems_ecu_reader` integration uses as "uncapped." On a DS3 (2 panels) that's 1000 W of headroom, which is well above any practical panel and effectively means "no curtailment." On a QS1 (4 panels, lower per-panel rating) the 500 W/panel boundary won't bind in practice either.

### Latency

Writes go to the ECU's SQLite tables. The ECU's dispatch loop polls those tables once per ZigBee cycle (default 300 s, fast-poll mode 30 s) and pushes the queued commands over the radio. **Expect 30–300 s** between a Modbus write and the actual inverter responding.

For real-time control (e.g. fast zero-feed-in), use AC-coupled frequency-shift on a Victron Multi instead — that's sub-second because each microinverter's local P-f curve responds without any radio round-trip.

### Verify a write end-to-end

```sh
# Read the current cap.
python3 -c "
import sunspec2.modbus.client as c
d = c.SunSpecModbusClientDeviceTCP(slave_id=2, ipaddr='<ECU-IP>', ipport=1502, timeout=3)
d.scan()
m = next(m for m in d.model_list if m.model_id == 123); m.read()
print('current pct:', m.WMaxLimPct.value)
"

# Cap to 50%.
python3 -c "
import sunspec2.modbus.client as c
d = c.SunSpecModbusClientDeviceTCP(slave_id=2, ipaddr='<ECU-IP>', ipport=1502, timeout=3)
d.scan()
m = next(m for m in d.model_list if m.model_id == 123); m.read()
m.WMaxLimPct.value = 50
m.write()
"

# Confirm the SQL row updated; flag=1 means the ECU dispatcher will
# pick it up on the next poll.
ssh ecu sqlite3 /home/database.db \
    "'SELECT id, limitedpower, flag FROM power'"
```

## Architecture / details

- [`docs/EVENTS.md`](docs/EVENTS.md) — full APsystems event bitstring decode, with mapping to standard SunSpec Evt1 flags.
- [`sunspec-models/`](sunspec-models/) — vendor model 64202 JSON descriptor and instructions for plugging into pysunspec2 / generic SunSpec libraries.

The encoder is data-driven: the inverter list, panel count, AC topology, and curtailment caps all come from the ECU's runtime state — nothing is hardcoded to a specific site or fleet.

## License

MIT.

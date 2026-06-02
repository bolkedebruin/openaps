# Installing OpenAPS v1.0.0 on an APsystems ECU

OpenAPS v1.0.0 ships as a single brownfield installer tarball that lands on
a stock APsystems **ECU-R-Pro** or **ECU-C** via the device's own firmware-update
endpoint. The installer DISABLES the stock supervisor processes (no
sleeper stubs), removes the CodeIgniter web UI, and starts the Go-based
OpenAPS stack on :443.

Pi / .deb support is **v2**. v1.0.0 supports only brownfield ECU install.

## Hardware compatibility

| Model | Serial prefix | Userspace | OpenAPS |
|---|---|---|:--:|
| **ECU-R-Pro** | `2162…` | Linux ARMv7 (BusyBox) | ✅ supported |
| **ECU-C** | `215…` | Linux ARMv7 (BusyBox) | ✅ supported |
| ECU-R / ECU-R-M3 | `2160…` | RT-Thread RTOS (no Linux userspace) | ❌ — cannot install |
| ECU-B | `2163…` | RT-Thread RTOS (no Linux userspace) | ❌ — cannot install |

Check your serial prefix on the device label or via the stock web UI before downloading the tarball. If you're on an RTOS-based ECU-R / ECU-B there is no install path — the binaries OpenAPS ships are ARMv7 Linux ELFs and won't run.

## What you need

- An APsystems **ECU-R-Pro** or **ECU-C** on your LAN, reachable on port 80.
- The ECU's IP address. (`arp -a | grep -i apsystems` from a host on the
  same VLAN usually finds it.)
- `openaps-v1.0.0-ecu.tar.bz2` from the GitHub release.
- ~50 MB of free space on the ECU's `/home` partition (the installer
  checks this and refuses to start if you're short).

## What the install does, in order

1. **Pre-flight** — verifies free disk and that `/etc/yuneng/ecu_eth0_mac.conf`
   is present (used to derive the ZigBee PAN). Checks ELF magic on the
   bundled binaries.
2. **Backup** — `tar -cjPf /home/openaps-backup-<ts>.tar.bz2 /home/applications
   /etc/rcS.d /etc/init.d /etc/yuneng`. Path recorded at
   `/etc/openaps/last-backup`.
3. **Dropbear FIRST** — installs `dropbear`, generates a host key,
   appends any bundled `authorized_keys`, starts dropbear, and **verifies
   :22 is listening** before continuing. If verification fails, the
   installer aborts BEFORE touching lighttpd — your stock web UI is still
   available for retry.
4. **Provision settings** — writes `/etc/inv-driver/settings.json` from
   `/etc/yuneng/ecu_eth0_mac.conf` + `/etc/yuneng/ecuid.conf`. Idempotent:
   if you've already edited that file, it's left alone.
5. **Install binaries + init scripts** — `inv-driver`, `ecu-zb`,
   `ecu-web`, `ecu-sunspec` under `/home/applications/`, plus
   `S48 / S53 / S54 / S99` in `/etc/rcS.d/`.
6. **Disable stock supervisors** — kills the running processes and moves
   their binaries aside into `/etc/openaps/disabled-bin/` so the next
   boot's `manager` `system()` calls no-op. Disabled set:
   - `monitor.exe` (kills main.exe path)
   - `clientmonitor`, `control_client_monitor`, `gprsmonitor` (cloud uplinks)
   - `idwriter` (unauthenticated)
   - `mqtt_monitor.exe` (CPU bug)
   - `phone_server_monitor`, `phone_server_app_monitor`, `lancommClientMonitor`
   - `single_update_monitor`, `updatemanager`, `autoupdate2`,
     `autoupdate_restart`, `AutoUpInver2`, `AutoUpInver`, `autoupdate`,
     `autoupdate_main_restart`
   - `resmonitor`, `usbTest`, `quectel_monitor.exe`, `4gmonitor.exe`,
     `diagnosis_network`

   **Kept:** `manager`, `network.exe`, `buttonreset`, `ledMonitor.exe`,
   `rtc_app`, `ntpapp.exe`, `wifi_init`.
7. **Remove lighttpd / CodeIgniter** — only after dropbear is verified
   in step 3. The stock unauthenticated endpoints (`exec_upgrade_ecu_app`,
   `set_ip`, `set_protection_parameters_inverter`) are gone after this
   step.
8. **Start OpenAPS** — `S48-inv-driver` → `S53-ecu-zb` → `S54-ecu-web`
   → `S99-sunspec`.
9. **Smoke verify** — confirms `/var/run/inv-driver.sock`, hits
   `https://localhost/api/auth/status`, and waits 30s for ecu-zb's first
   telemetry frame.
10. **Audit** — writes `/etc/openaps/installed.json` and
    `/home/openaps-install.log`.

## The install command

```sh
curl -X POST -F "file=@openaps-v1.0.0-ecu.tar.bz2" \
     http://<ECU-IP>/index.php/management/exec_upgrade_ecu_app
```

That endpoint returns `{"value":0,"res":0,"result":""}` immediately — the
install runs asynchronously under PHP-FPM. **Watch the install log:**

```sh
# Before the install removes lighttpd you can still use the stock web UI
# logs. After the install you'll have dropbear:
ssh root@<ECU-IP> 'tail -f /home/openaps-install.log'
```

The installer typically takes ~60-90s end-to-end.

## Verifying the install

After ~90s:

- Open `https://<ECU-IP>/` in a browser (self-signed cert; click through).
- Log in (first run prompts you to set a password).
- The fleet view should show your inverters within a couple of minutes
  once ecu-zb has rejoined the radio mesh.

From the command line:

```sh
ssh root@<ECU-IP> cat /etc/openaps/installed.json
ssh root@<ECU-IP> netstat -ltn   # :22 dropbear, :443 ecu-web, :502 sunspec, :19999 ecu-zb tap
```

> **First-time SSH note:** the bundled dropbear is 2012.55 (latest build that links against the ECU's glibc 2.15 runtime) and predates the algorithms modern OpenSSH ships enabled by default. If `ssh root@<ECU-IP>` fails to negotiate, drop the legacy-host block from [`SSH-CONFIG.md`](SSH-CONFIG.md) into your `~/.ssh/config`.

## Rolling back

The installer ships a rollback CLI at `/usr/local/bin/openaps-rollback`:

```sh
ssh root@<ECU-IP> /usr/local/bin/openaps-rollback
```

It stops OpenAPS, restores the stock binaries + init scripts from the
backup tarball, and reboots. The ECU comes back up on stock firmware
(plus dropbear — the installer doesn't remove dropbear because it's your
recovery path).

If you need to roll back from a specific backup:

```sh
ssh root@<ECU-IP> /usr/local/bin/openaps-rollback --backup /home/openaps-backup-<ts>.tar.bz2
```

## Known limitations (v1.0.0)

- **No Pi support.** The radio firmware story (APsystems-proprietary
  CC2530 vs stock TI ZNP on off-the-shelf dongles) is the actual blocker.
  v2 unblocks generic radios via a `bus-mgr ti-znp-zb` backend in
  `ecu-zb`.
- **Unsigned tarball.** v1.0.0 ships an UNSIGNED installer. The
  installer plumbs `release.pub` to `/etc/openaps/release.pub` so v1.0.1
  can verify signed OTAs — but v1.0.0 itself does not check the
  signature. Install on trusted networks only.
- **Trust model.** Install only on LANs you control. The installer's
  last act removes the stock unauthenticated admin endpoints.

## Recovery contingencies

- **Before install:** the stock ECU's telnet root shell is at
  `:2323` (if previously enabled) and the stock CodeIgniter UI on
  `:80` is reachable.
- **After install:** dropbear SSH on `:22`. The installer refuses to
  remove lighttpd until dropbear is verified listening.
- **If the install aborts during steps 1-6:** the stock firmware is
  intact — dropbear is the only addition. Stock web UI on :80 still
  works.
- **If the install aborts during steps 7-10:** lighttpd is gone, but
  dropbear is your remote shell. Run `openaps-rollback` (it lands at
  `/usr/local/bin/` in step 10, so if the install aborted before then,
  extract it from the tarball manually: `tar -xjf
  openaps-v1.0.0-ecu.tar.bz2 update/openaps-rollback -O > /tmp/r && sh
  /tmp/r`).

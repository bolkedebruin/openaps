# OpenAPS v1.0.8

Patch release in the `v1.0.x` stream. **Keeps the stock `idwriter` daemon enabled
after migration as a LAN recovery path.** Behaviour change in the installer only;
the OpenAPS services are unchanged from v1.0.7.

## Changed — `idwriter` is no longer disabled

The brownfield installer's step-6 supervisor-disable list no longer includes
`idwriter`. After migration the box keeps the stock provisioning daemon running
on TCP `:4540`, which exposes an **unauthenticated root command surface** (the
`A108:` shell verb). It is retained deliberately as a **recovery path**: once the
stock web UI is removed, dropbear on `:22` is otherwise the only way back in, and
if dropbear ever fails to start (e.g. a host-key problem) idwriter's `A108:`
backdoor is the fallback that gets a root shell on the LAN.

This is an intentional security trade-off — it leaves an unauthenticated root
surface reachable on the LAN. The installer comment and README call it out; to
opt out, add `idwriter` back to the step-6 disable set (or remove the binary on
the box).

Everything else (the full v1.0.3–v1.0.7 installer arc — BusyBox fixes,
`import-stock` inventory seeding, bundled grid profiles, the reordered/rebooting
install that lands on working OpenAPS) is carried forward unchanged.

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
curl -H "Expect:" -F "file=@openaps-v1.0.8-ecu.tar.bz2" \
     http://<ECU-IP>/index.php/management/exec_upgrade_ecu_app
```

`{"res":0}` means received + launched; watch `/home/openaps-install.log` until
the reboot, then open `https://<ECU-IP>/` after ~1-2 minutes. Roll back with
`ssh root@<ECU-IP> /usr/local/bin/openaps-rollback`.

## Still deferred

- **Signed-tarball OTA** — `release.pub` ships; `/api/upgrade` not yet plumbed.
- **Passkeys** — blocked on WebAuthn rpId / PSL constraints; needs a real DNS name.
- **AES encrypt wire-up in ecu-zb** — inbound decrypt only.
- **v2:** Pi / .deb / systemd packaging; `bus-mgr ti-znp-zb` USB-radio backend.

## Artifacts

- `openaps-v1.0.8-ecu.tar.bz2` — brownfield installer.
- `SHA256SUMS`.

## Install caveats

Install only on LANs you control. v1.0.8 additionally leaves `idwriter`'s
unauthenticated root surface on `:4540` — only run it where that is acceptable.

# ecu-zb

Transparent splice between the host process and the CC2530 ZigBee
modem on `/dev/ttyO2`, with a pcapng tap fanout for live Wireshark
dissection.

v1 is byte-oriented passthrough only — no MITM, no L2 parsing on the
hot path. The hook interface exists for v2; `NoOpHook` is wired.

## Build

```
make armv7         # cross-compile static armv7l binary
make deploy ECU_HOST=root@<ECU-IP>  # ssh+cat to <ECU-IP>:/home/applications/ecu-zb/
make install-init  # install S53-ecu-zb in /etc/rcS.d/ (boot-time start)
```

`scp` and `sftp-server` are absent from the ECU; deployment uses
`ssh + cat` instead.

## Boot-time integration

`packaging/S53-ecu-zb` is a BusyBox init script that goes into
`/etc/rcS.d/` and runs at S53 (after `S50ecu_init`). Boot order on this
firmware:

```
S50ecu_init        →  brings up /dev/reset + radio kernel modules
S53-ecu-zb start   →  /dev/ttyO2 renamed to .real, pty published
```

ecu-zb runs at S53 because nothing needs it earlier and `/dev/reset` +
the radio kernel modules are then up for the hardware-reset bring-up.
After `make install-init`, reboot the ECU and ecu-zb starts at boot.

For development / one-off runs the init script also supports
`{start|stop|restart|status}` invocations.

## Run on the ECU

```sh
# move the real UART aside so the host process will open our pty
mv /dev/ttyO2 /dev/ttyO2.real

/home/applications/ecu-zb/ecu-zb \
    --tty /dev/ttyO2.real \
    --link /dev/ttyO2 \
    --pty /home/applications/ecu-zb/run/zb-pty \
    --tap-sock /home/applications/ecu-zb/run/pcap.sock \
    --tap-tcp 0.0.0.0:19999 \
    > /home/applications/ecu-zb/log/ecu-zb.log 2>&1 &
```

On clean shutdown (SIGINT/SIGTERM) `ecu-zb` removes the symlink and
restores `/dev/ttyO2.real → /dev/ttyO2`.

## Live capture

```sh
wireshark -k -i TCP@<ECU-IP>:19999
```

The pcapng stream uses link type `DLT_USER0` (147) with the same
per-packet payload format the existing `aps_zigbee.lua` dissector
already understands: byte 0 = direction (0 = host→CC2530,
1 = CC2530→host), bytes 1..N = raw chunk.

Late connectors do NOT receive backlog. The TCP listener fans out the
same stream to N concurrent clients.

## v1 scope

- Transparent splice with `NoOpHook` (passes 100% of bytes through).
- pcapng tap with multi-consumer fanout over Unix STREAM socket and
  TCP listener.
- Clean shutdown: restore `/dev/ttyO2`, remove pty symlink, exit 0.

## Out of scope (for now)

- L0/L1/L2 parsing on the hot path.
- Reassembly, frame modification, synth/divert.
- SQLite, MQTT, SunSpec integration.

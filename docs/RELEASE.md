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

# APsystems event-bit decode

ecu-sunspec exposes inverter alarms via standard SunSpec `Evt1` plus
vendor registers `EvtVnd1` / `EvtVnd2` / `EvtVnd3`. There are **two
source paths** that contribute, and the bit layout in `EvtVnd*`
depends on which one supplied the data for a given inverter:

## Source paths

| Source | When | Evt1 | EvtVnd1 | EvtVnd2/3 |
|---|---|---|---|---|
| **inv-driver (typed)** | inv-driver supplied a telemetry frame this session | `FaultsToSunSpecEvt1` | **New typed layout** (table below) | zero (reserved) |
| **SQLite Event.eve (legacy)** | no inv-driver data available — fallback | `MapAPsystemsToSunSpecEvt1` | Raw PHP-UI bits 0-31 | Raw bits 32-63, 64-95 |

Standard SunSpec `Evt1` is the **OR of both contributions** when both
are available — generic SunSpec consumers (HA, Venus, etc.) keep
seeing complete data during the transition.

`EvtVnd1` semantics depend on which path supplied this inverter.
The dispatch is per-inverter: if **any** inverter has typed faults,
its EvtVnd1 reflects the new layout. Inverters that have only legacy
SQLite data continue to emit the legacy raw bits in EvtVnd1/2/3 until
inv-driver covers them too.

---

## New typed-faults layout (`EvtVnd1` from inv-driver)

Defined and owned by ecu-sunspec. 26 named bits, family-agnostic
where bits overlap. DS3's `stage1/stage2/aux` collapses onto
`Primary/Secondary/Tertiary`; QS1A's `fast/slow` collapses onto
`Primary/Secondary`. Family-specific `Extra`/`RMS` buckets get their
own bits. Bits 26-31 reserved.

| Bit | Constant | Meaning (1 = event/fault, 0 = healthy) | DS3 source | QS1A source |
|----:|---|---|---|---|
| 0 | `EvtVnd1GridRelayFault` | Grid-side relay event (DS3 alarm-group: comm/RTC; QS1A: paired with over-temp). Polarity confirmed via `DS3_DS3D_status @ 0x290f8` and `qs1200_60_status @ 0x297d8` alarm-builder gates. | body[0x0c] bit 1 | body[0x17] bit 2 |
| 1 | `EvtVnd1DCContactorFault` | DC contactor / inverter-on-relay event | body[0x0d] bit 7 | body[0x17] bit 1 |
| 2 | `EvtVnd1DCBusFault` | DC-bus event (counter inv+0x3f0 tracks fault-poll count; inv+0x3ec the healthy streak since last fault) | body[0x0d] bit 4 | body[0x18] bit 4 |
| 3 | `EvtVnd1DCGroundFault` | DC ground / GFCI | body[0x0d] bit 3 | body[0x17] bit 3 |
| 4 | `EvtVnd1CommFault` | Comm / RTC fault | — | body[0x17] bit 4 |
| 5 | `EvtVnd1OverTemperature` | Over-temperature | — | body[0x18] bit 2 |
| 6 | `EvtVnd1IsoFaultA` | Isolation sensor A | body[0x0e] bit 4 | body[0x19] bit 2 |
| 7 | `EvtVnd1IsoFaultB` | Isolation sensor B | body[0x0e] bit 6 | body[0x19] bit 4 |
| 8 | `EvtVnd1IsoFaultC` | Isolation sensor C | — | body[0x18] bit 0 |
| 9 | `EvtVnd1IsoFaultD` | Isolation sensor D | — | body[0x19] bit 6 |
| 10 | `EvtVnd1ACOverVoltPrimary` | AC over-voltage (stage1 / fast) | body[0x0e] bit 0 | body[0x1a] bit 2 |
| 11 | `EvtVnd1ACOverVoltSecondary` | AC over-voltage (stage2 / slow) | body[0x0e] bit 2 | body[0x1a] bit 0 |
| 12 | `EvtVnd1ACUnderVoltPrimary` | AC under-voltage (stage1 / fast) | body[0x0e] bit 1 | body[0x1a] bit 3 |
| 13 | `EvtVnd1ACUnderVoltSecondary` | AC under-voltage (stage2 / slow) | body[0x0e] bit 3 | body[0x1a] bit 1 |
| 14 | `EvtVnd1OverFreqPrimary` | Over-freq (stage1 / fast) | body[0x0f] bit 2 | body[0x1a] bit 6 |
| 15 | `EvtVnd1OverFreqSecondary` | Over-freq (stage2 / slow) | body[0x0f] bit 4 | body[0x1a] bit 4 |
| 16 | `EvtVnd1OverFreqTertiary` | Over-freq (aux) | body[0x0f] bit 6 | — |
| 17 | `EvtVnd1OverFreqExtra` | Over-freq (extra) | body[0x0f] bit 0 | body[0x19] bit 0 |
| 18 | `EvtVnd1OverFreqRMS` | Over-freq (10-min RMS) | — | body[0x17] bit 5 |
| 19 | `EvtVnd1UnderFreqPrimary` | Under-freq (stage1 / fast) | body[0x0f] bit 3 | body[0x1a] bit 7 |
| 20 | `EvtVnd1UnderFreqSecondary` | Under-freq (stage2 / slow) | body[0x0f] bit 5 | body[0x1a] bit 5 |
| 21 | `EvtVnd1UnderFreqTertiary` | Under-freq (aux) | body[0x0f] bit 7 | — |
| 22 | `EvtVnd1UnderFreqExtra` | Under-freq (extra) | body[0x0f] bit 1 | body[0x19] bit 1 |
| 23 | `EvtVnd1UnderFreqRMS` | Under-freq (10-min RMS) | — | body[0x17] bit 6 |
| 24 | `EvtVnd1ZBLinkA` | ZigBee link channel A | — | body[0x39] bit 0 |
| 25 | `EvtVnd1ZBLinkB` | ZigBee link channel B | — | body[0x39] bit 1 |

`EvtVnd2`, `EvtVnd3`, `EvtVnd4` are reserved on the typed path — they
will carry RCD / subsystem-fault / ZB-link detail when inv-driver owns
those sources too.

Source for the bit assignments: firmware decompile (resolvedata_DS3,
resolvedata_60_1200) plus the SunSpec mapping verified against
update_modbus_status.

---

## Legacy layout (`EvtVnd1/2/3` raw PHP-UI bits)

This section documents the bit positions of the 86-bit event
bitstring stored in the ECU's `/home/database.db Event` table
(column `eve`), taken from the firmware's UI language file at
`/home/local_web/pages/application/language/english/page_lang.php`
(`display_status_zigbee_<N>` keys). The bitstring is **only** emitted
when no inv-driver-sourced faults are available for an inverter —
treat this layout as deprecated. Verified through firmware 2.1.29D.

## Full bit table (positions 0-83)

| Bit | SunSpec mapping            | APsystems name (English)                   |
|----:|----------------------------|---------------------------------------------|
|   0 | Evt1OverFrequency          | AC Frequency Exceeding Range                |
|   1 | Evt1UnderFrequency         | AC Frequency Under Range                    |
|   2 | Evt1ACOverVolt             | Channel A: AC Voltage Exceeding Range       |
|   3 | Evt1ACUnderVolt            | Channel A: AC Voltage Under Range           |
|   4 | Evt1ACOverVolt             | Channel B: AC Voltage Exceeding Range       |
|   5 | Evt1ACUnderVolt            | Channel B: AC Voltage Under Range           |
|   6 | Evt1ACOverVolt             | Channel C: AC Voltage Exceeding Range       |
|   7 | Evt1ACUnderVolt            | Channel C: AC Voltage Under Range           |
|   8 | Evt1DCOverVolt             | Channel A: DC Voltage Too High              |
|   9 | (none — vendor only)       | Channel A: DC Voltage Too Low               |
|  10 | Evt1DCOverVolt             | Channel B: DC Voltage Too High              |
|  11 | (none — vendor only)       | Channel B: DC Voltage Too Low               |
|  16 | Evt1OverTemp               | Over Critical Temperature                   |
|  17 | Evt1GroundFault            | GFDI Locked                                 |
|  18 | Evt1ManualShutdown         | Remote Shut                                 |
|  19 | Evt1ACDisconnect           | AC Disconnect                               |
|  21 | Evt1GridDisconnect         | Active Anti-island Protection (freq-shift!) |
|  22 | (none — vendor only)       | CP Protection                               |
|  23 | Evt1ACOverVolt             | AC Voltage Exceeding Range (legacy)         |
|  24 | Evt1ACUnderVolt            | AC Voltage Under Range (legacy)             |
|  25 | (none — vendor only)       | 10min Protect                               |
|  26 | (none — vendor only)       | BUS Voltage Too Low                         |
|  27 | (none — vendor only)       | BUS Voltage Too High                        |
|  28 | Evt1HWTestFailure          | Relay Failed                                |
|  29 | Evt1OverFrequency          | AC Frequency stage-1 Exceeding Range        |
|  30 | Evt1UnderFrequency         | AC Frequency stage-1 Under Range            |
|  31 | Evt1OverFrequency          | AC Frequency stage-2 Exceeding Range        |
|  32 | Evt1UnderFrequency         | AC Frequency stage-2 Under Range            |
|  33 | Evt1ACOverVolt             | AC Voltage stage-2 Exceeding Range          |
|  34 | Evt1ACUnderVolt            | AC Voltage stage-2 Under Range              |
|  35 | Evt1ACOverVolt             | AC Voltage stage-3 Exceeding Range          |
|  36 | Evt1ACUnderVolt            | AC Voltage stage-3 Under Range              |
|  37 | Evt1ACOverVolt             | AC Voltage stage-4 Exceeding Range          |
|  38 | Evt1ACUnderVolt            | AC Voltage stage-4 Under Range              |
|  39 | Evt1DCOverVolt             | Channel C: DC Voltage Too High              |
|  40 | (none — vendor only)       | Channel C: DC Voltage Too Low               |
|  41 | Evt1DCOverVolt             | Channel D: DC Voltage Too High              |
|  42 | (none — vendor only)       | Channel D: DC Voltage Too Low               |
|  43 | (none — vendor only)       | Get Data Failed                             |
|  44 | Evt1ACOverVolt             | AC Voltage stage-1 Exceeding Range          |
|  45 | Evt1ACUnderVolt            | AC Voltage stage-1 Under Range              |
|  46 | (none — vendor only)       | AC-Parameter setting errors                 |
|  47 | (none — vendor only)       | Varistors Protection                        |
|  48 | (none — vendor only)       | AB-line stage-1 over-voltage (3-phase)      |
|  49 | (none — vendor only)       | AB-line stage-1 under-voltage               |
|  50 | (none — vendor only)       | AB-line stage-2 over-voltage                |
|  51 | (none — vendor only)       | AB-line stage-2 under-voltage               |
|  52 | (none — vendor only)       | AB-line stage-3 voltage                     |
|  53 | (none — vendor only)       | AB-line stage-3 voltage                     |
|  54 | (none — vendor only)       | AB-line stage-4 voltage                     |
|  55 | (none — vendor only)       | AB-line stage-4 voltage                     |
|  56-63 | (none — vendor only)    | BC-line stage-1..4 voltages                 |
|  64-71 | (none — vendor only)    | CA-line stage-1..4 voltages                 |
|  72 | (none — vendor only)       | CP1 protection                              |
|  73 | (none — vendor only)       | CP2 protection                              |
|  74 | (none — vendor only)       | Over-temperature derating                   |
|  75 | (none — vendor only)       | Over-frequency derating                     |
|  76 | (none — vendor only)       | Over-voltage derating                       |
|  77 | (none — vendor only)       | Channel A arc fault                         |
|  78 | (none — vendor only)       | Channel B arc fault                         |
|  79 | (none — vendor only)       | Channel C arc fault                         |
|  80 | (none — vendor only)       | Channel D arc fault                         |
|  81 | (none — vendor only)       | Channel A overcurrent                       |
|  82 | (none — vendor only)       | Channel B overcurrent                       |
|  83 | (none — vendor only)       | D-signal timeout                            |

(Bits 12-15, 20 are not assigned in this firmware.)

Bits 48-71 use Chinese strings in `page_lang.php`; English equivalents above
are paraphrases. The structure is `{AB,BC,CA}-line × stage-{1,2,3,4} ×
{over,under}-voltage`.

## Notes for the freq-shift use case

If you're driving an AC-coupled installation via Victron freq-shift, the
bits to watch are:

- **Bit 21** (`Active Anti-island Protection`) — fires when the inverter
  hits its anti-island trip; this is the bit that flips when your Multi
  raises Hz to throttle.
- **Bits 0, 29, 31** (over-frequency variants) — the Multi's freq-shift
  causes these as it ramps Hz up.
- **Bit 19** (AC Disconnect) — full disconnect.

In SunSpec these all surface in `Evt1`:
`OverFrequency | GridDisconnect | ACDisconnect`. Build HA automations on
those flags rather than per-bit.

## Latched vs transient

Empirically, several bits stay set indefinitely (e.g., bits 9, 11 — DC
voltage low — were set on every poll on this site even with the panels
producing). They appear to be *latched* — set when first observed, never
cleared until the inverter is power-cycled or restarted. Use for forensics,
not real-time alarms.

The standard-SunSpec `Evt1` mapping deliberately excludes these latched DC
flags so HA's alarm panel doesn't constantly show "DC fault" for healthy
inverters. They remain visible in `EvtVnd1`/`EvtVnd2` for owners who want to
see them.

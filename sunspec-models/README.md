# Custom SunSpec model definition for `ecu-sunspec`

`model_64202.json` is the SunSpec-style descriptor for the vendor model emitted by `ecu-sunspec`. Drop it where pysunspec2 (the library used by HA's SunSpec custom component, by `suns`, and by most generic SunSpec consumers) looks for model definitions, and clients will decode the model natively instead of seeing it as an opaque "model 64202".

## Where to install

### Home Assistant (CJNE/ha-sunspec)

Locate pysunspec2's model directory. Inside HA's container:

```sh
python3 -c "import sunspec2.file.client as f, os; print(os.path.dirname(f.__file__))"
# -> /usr/local/lib/python3.x/site-packages/sunspec2
```

Copy the JSON next to the bundled models:

```sh
cp model_64202.json /usr/local/lib/python3.x/site-packages/sunspec2/models/json/
```

Restart HA. The vendor model now appears as a "APsystems ECU vendor extras" entity group with `today_wh`, `month_wh`, `year_wh`, `polling_s`, `ecu_fw_ver`, plus per-inverter `idx_n_rssi`, `idx_n_limited_w`, etc.

### pysunspec2 CLI (`suns`)

Same idea, or set `SUNSPEC_MODELS_PATH` to a directory containing the JSON:

```sh
export SUNSPEC_MODELS_PATH=/path/to/this/directory
python3 -m sunspec2.cli.suns -i <ECU-IP> -P 1502
```

### Other SunSpec libraries

This JSON follows the SunSpec Alliance "JSON model" format. Most SunSpec libraries that load JSON model definitions will accept it — check your library's docs for the search path.

## Model layout (for reference)

```
ID                  64202
L                   16 + 10·N

Fixed block:
  PollingS          uint16    seconds  /etc/yuneng/polling_interval.conf
  EcuFwVer          string    8 regs   /etc/yuneng/version.conf
  TodayWh           acc32     Wh       historical_data.db.daily_energy
  MonthWh           acc32     Wh       monthly_energy
  YearWh            acc32     Wh       yearly_energy
  N                 uint16             number of inverter rows below

Repeating block (N times, 10 regs each):
  Idx               uint16             1-based index
  RSSI              uint16   0..255    signal_strength.signal_strength
  LimitedW          uint16   W         power.limitedpower (per-panel cap)
  Phase             uint16             id.phase (0..3)
  Model             uint16             id.model
  SoftwareVer       uint16             id.software_version
  PanelCnt          uint16             2 for DS3, 4 for QS1
  NameplateW        uint16   W         rated AC output
  Online            uint16   bool      1 = online
  Pad               uint16
```

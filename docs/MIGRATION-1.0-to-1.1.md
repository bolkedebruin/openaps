# Migrating an ECU from OpenAPS 1.0.X to 1.1.X

1.0.X suppressed the stock APsystems firmware with **in-place sleeper stubs**
(each `/home/applications/<app>` replaced by a shell loop, the real ELF saved
as `<app>.real`). 1.1.X instead disables stock **cleanly at the manager level**:
the `apsystems-stock` package comments the single manager launch in
`/etc/rcS.d/S50ecu_init`, so nothing stock starts at boot and the binaries stay
pristine.

`openaps-unstub-stock.sh` (a release asset) converts the former to the latter.
It restores every stubbed binary and disables the manager — producing the exact
state `opkg remove apsystems-stock` would — and starts nothing. It is
idempotent and supports `--dry-run`.

Everything below runs on the ECU as **root**, over its existing SSH.

## 1. Install persistent SSH first (required)

The migration disables the stock manager, which also stops the stock recovery
paths. You must have **manager-independent SSH** in place first, or the next
reboot could lock you out — `openaps-unstub-stock.sh` refuses to run otherwise.

`openaps-dropbear` provides it: its postinst generates the host key, fixes the
`/home/root` ownership dropbear needs, and **starts dropbear immediately** (no
reboot needed) via `/etc/rcS.d/S98-dropbear`.

Copy the package to the ECU and install it locally:

```sh
# from your machine — the ECU's old TLS can't fetch from GitHub directly:
scp -O openaps-dropbear_<ver>_armv7ahf-vfp-neon.ipk root@<ecu>:/tmp/
# (or, if scp's SFTP is unavailable on the box:
#   ssh root@<ecu> 'cat > /tmp/openaps-dropbear.ipk' < openaps-dropbear_<ver>_*.ipk )

# on the ECU:
opkg install /tmp/openaps-dropbear_<ver>_armv7ahf-vfp-neon.ipk
```

Confirm SSH is up independently before continuing: open a **second** SSH session
and keep it open.

## 2. Un-stub and disable stock

Copy `openaps-unstub-stock.sh` to the ECU, preview, then apply:

```sh
ssh root@<ecu> 'cat > /tmp/openaps-unstub-stock.sh' < openaps-unstub-stock.sh

# on the ECU:
sh /tmp/openaps-unstub-stock.sh --dry-run    # preview
sh /tmp/openaps-unstub-stock.sh              # apply
```

After this, stock is disabled (manager launch commented, `stock-disabled` marker
set), all `<app>.real` binaries are restored, and nothing stock is running. The
OpenAPS services are untouched.

## 3. Install the corrected 1.1.X packages

If the box isn't on the opkg feed yet, copy over and `opkg install ./<file>.ipk`
the rest of the packages (the `apsystems-stock` package is **not** needed for a
stock-disabled box — installing it would re-enable stock):

```sh
opkg install /tmp/openaps-base_<ver>_*.ipk \
             /tmp/openaps-tls-proxy_<ver>_*.ipk \
             /tmp/openaps-inv-driver_<ver>_*.ipk \
             /tmp/openaps-ecu-zb_<ver>_*.ipk \
             /tmp/openaps-ecu-web_<ver>_*.ipk \
             /tmp/openaps-ecu-sunspec_<ver>_*.ipk \
             /tmp/openaps-recoveryd_<ver>_*.ipk \
             /tmp/ntpdate_<ver>_*.ipk
```

Installing `openaps-tls-proxy` + `openaps-base` configures the opkg feed, so
from then on `opkg update && opkg install <pkg>` works over the signed feed.

## 4. Reboot

```sh
/sbin/reboot
```

The box comes up on the 1.1.X Go stack (`inv-driver`, `ecu-zb`, `ecu-web`,
`ecu-sunspec`) with the stock firmware fully disabled.

### Re-enabling stock later

Stock is disabled by absence of `apsystems-stock` plus the commented manager
launch. To get it back: `opkg install apsystems-stock` (its postinst restores
`S50ecu_init` from the backup), then `/sbin/reboot`.

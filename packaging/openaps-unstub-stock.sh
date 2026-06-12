#!/bin/sh
#
# openaps-unstub-stock.sh — bring a 1.0.X-style ECU (stock firmware suppressed
# by in-place sleeper STUBS) to the 1.1.X stock-disabled layout, without opkg
# and without touching the OpenAPS services.
#
# Two idempotent actions:
#   1. Disable stock at the manager level (the 1.1.X way): back up
#      /etc/rcS.d/S50ecu_init, comment out the single `… /home/applications/
#      manager &` launch line, and drop the /etc/openaps/stock-disabled marker.
#      No stock app starts at the next boot. This is exactly the on-disk state
#      that `opkg remove apsystems-stock` produces.
#   2. Un-stub: restore every /home/applications/<name>.real back over its
#      sleeper stub so the stock binaries are pristine again.
#
# It starts NOTHING and does not touch inv-driver/ecu-zb/ecu-web. Afterwards the
# stock binaries are restored but inert (the disabled manager never launches
# them), so you can install the corrected 1.1.X packages and reboot.
#
# SAFETY: refuses unless persistent, manager-independent SSH (openaps-dropbear,
# /etc/rcS.d/S98-dropbear) is installed and running — disabling the manager
# otherwise risks an SSH lockout at the next boot.
#
# Usage:  sh openaps-unstub-stock.sh             # apply
#         sh openaps-unstub-stock.sh --dry-run   # show what it would do
set -eu

INIT=/etc/rcS.d/S50ecu_init
STATE=/etc/openaps
BACKUP=$STATE/S50ecu_init.stock-backup
MARK=$STATE/stock-disabled
APPS=/home/applications
DROPBEAR_INIT=/etc/rcS.d/S98-dropbear

DRY=0
[ "${1:-}" = "--dry-run" ] && DRY=1
run() { if [ "$DRY" = 1 ]; then echo "  [dry-run] $*"; else eval "$*"; fi; }

echo "=== openaps stock un-stub + disable ==="

# --- Safety guard: persistent SSH must survive the manager being disabled ----
if [ ! -x "$DROPBEAR_INIT" ] || ! pidof dropbear >/dev/null 2>&1; then
    echo "REFUSING: persistent SSH (openaps-dropbear, $DROPBEAR_INIT) is not"
    echo "installed and running. Disabling the stock manager would risk an SSH"
    echo "lockout at the next boot. Install openaps-dropbear (and reboot) first."
    exit 1
fi
echo "ok: persistent SSH present (rcS-level dropbear)."

# --- 1. Disable stock at the manager level (idempotent via the marker) -------
if [ -f "$MARK" ]; then
    echo "stock already disabled (marker $MARK present) — skipping manager edit."
elif [ ! -f "$INIT" ]; then
    echo "note: $INIT not found — nothing to disable at the init level."
else
    run "mkdir -p '$STATE'"
    [ -f "$BACKUP" ] || run "cp -f '$INIT' '$BACKUP'"
    run "sed -i '/\\/home\\/applications\\/manager[[:space:]]*&/s/^/# OPENAPS-DISABLED /' '$INIT'"
    run "touch '$MARK'"
    echo "disabled: commented the manager launch in $INIT (backup: $BACKUP)."
fi

# --- 2. Stop the running stock inverter/web supervisors (conservative) -------
# Network/MAC/system apps are left for this session; none restart after a reboot
# because the manager launch is now commented out. Each supervisor is matched
# both as a real binary (killall) and as an sh-wrapped sleeper stub (its comm is
# "sh", so killall misses it — match the script path in /proc/<pid>/cmdline).
stop_stock() {
    sig=$1
    for name in manager monitor.exe main.exe lighttpd thttpd mqtt.exe; do
        run "killall $sig -q '$name' 2>/dev/null || true"
        for pd in /proc/[0-9]*; do
            [ -r "$pd/cmdline" ] || continue
            cl=$(tr '\0' ' ' < "$pd/cmdline" 2>/dev/null)
            case " $cl " in
                *" /home/applications/$name "*) run "kill $sig '${pd#/proc/}' 2>/dev/null || true" ;;
            esac
        done
    done
}
stop_stock ""
run "sleep 1"
stop_stock "-9"

# --- 3. Un-stub: restore every <name>.real over its stub --------------------
n=0
for real in "$APPS"/*.real; do
    [ -f "$real" ] || continue          # glob did not match → no stubs
    name=${real%.real}
    run "mv -f '$real' '$name'"
    run "chmod 0755 '$name'"
    echo "  un-stubbed: $(basename "$name")"
    n=$((n + 1))
done
echo "restored $n stock binary(ies)."

echo
echo "=== done ==="
echo "Stock is DISABLED (the manager won't launch at boot) and its binaries are"
echo "restored but NOT running. Next:"
echo "  - copy over and install the corrected 1.1.X packages (openaps-base,"
echo "    openaps-dropbear, openaps-tls-proxy, openaps-inv-driver, -ecu-zb,"
echo "    -ecu-web, -ecu-sunspec, -recoveryd, ntpdate)."
echo "  - do NOT install apsystems-stock unless you want stock back (install it,"
echo "    then 'opkg remove apsystems-stock', to keep stock off but opkg-tracked)."
echo "  - reboot when ready."

#!/bin/sh
#
# fetch-busybox.sh — download the static musl busybox-armv7l binary that
# provides the OpenAPS time client (ntpd) and cron (crond).
#
# Why busybox and not ntpdate: the Debian wheezy ntpdate we used to ship is
# dynamically linked against libcrypto.so.1.0.0 (OpenSSL 1.0.0). Newer stock ECU
# firmware upgraded to OpenSSL 1.1 and dropped 1.0.0, so that binary can no
# longer load and the clock never gets set (silently, from the S56 loop). This
# busybox is a fully STATIC musl build — no shared libraries at all — so it is
# immune to the box's OpenSSL version, and its ntpd applet does SNTP with no
# crypto dependency. Validated running on the ECU's armv7 hardfloat / Linux 3.2
# userland: steps a large offset then disciplines the clock.
#
# The binary is VENDORED at packaging/vendor/busybox-armv7l and committed to the
# repo; the normal build uses that committed copy and never touches the network
# (busybox.net is a single flaky host). This script is the REFRESH/re-pin tool:
# run it to re-download and SHA-verify the vendored binary, e.g. after bumping
# the pinned version.
#
# Usage:
#   ./packaging/fetch-busybox.sh [output-dir]   # default: packaging/vendor
#
# Produces:
#   <out>/busybox-armv7l   (update SHA256 below if you bump the version)

set -e

OUT="${1:-packaging/vendor}"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

# busybox 1.31.0, armv7l, musl static (defconfig-multiarch build). The ntpd and
# crond applets are compiled in (CONFIG_NTPD=y / CONFIG_CROND=y).
#
# PROVENANCE / TRUST: this is an upstream prebuilt binary from busybox.net (the
# project's "well-known" binaries), NOT reproducible from a pinned source tree.
# Unlike the Debian snapshot.debian.org fetches (content-addressed URLs), this
# URL is path-addressed — the file at that path could be swapped — so the pinned
# SHA-256 below is the SOLE integrity anchor; the download is fail-closed on any
# mismatch. Hash cd04052b... independently verified running on a live ECU
# (Linux 3.2 armv7 hardfloat): steps a large offset then disciplines the clock,
# opens no listening socket. The pin also FREEZES the security posture at 1.31.0
# (no upstream backports) — re-review and re-bump periodically.
URLS="
https://busybox.net/downloads/binaries/1.31.0-defconfig-multiarch-musl/busybox-armv7l
https://www.busybox.net/downloads/binaries/1.31.0-defconfig-multiarch-musl/busybox-armv7l
"
SHA256="cd04052b8b6885f75f50b2a280bfcbf849d8710c8e61d369c533acf307eda064"

echo "+ fetching busybox-armv7l"
BIN="$WORK/busybox"
ok=0
for url in $URLS; do
    [ -z "$url" ] && continue
    echo "  trying $url"
    if curl -fsSL --connect-timeout 10 --max-time 120 -o "$BIN" "$url"; then
        ok=1
        break
    fi
done
if [ "$ok" -ne 1 ]; then
    echo "ERROR: could not download busybox-armv7l from any mirror"
    exit 1
fi

# Verify checksum and fail closed on mismatch.
got_sha=$(shasum -a 256 "$BIN" 2>/dev/null | awk '{print $1}')
if [ -z "$got_sha" ]; then
    got_sha=$(sha256sum "$BIN" 2>/dev/null | awk '{print $1}')
fi
if [ -z "$got_sha" ]; then
    echo "ERROR: no SHA-256 tool available (need shasum or sha256sum)"
    exit 1
fi
if [ "$got_sha" != "$SHA256" ]; then
    echo "ERROR: checksum mismatch"
    echo "  expected $SHA256"
    echo "  got      $got_sha"
    exit 1
fi

mkdir -p "$OUT"
cp "$BIN" "$OUT/busybox-armv7l"
chmod 0755 "$OUT/busybox-armv7l"

echo "+ wrote (vendored) busybox-armv7l to $OUT"
echo
echo "verify ELF arch:"
file "$OUT/busybox-armv7l" 2>/dev/null | head -1 || true

echo
echo "commit the refreshed binary; the build (make ipk-busybox) uses it directly."

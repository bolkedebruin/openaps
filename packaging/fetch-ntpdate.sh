#!/bin/sh
#
# fetch-ntpdate.sh — download an ntpdate .deb from Debian's wheezy archive
# and extract the ntpdate binary needed to package opkg `ntpdate`.
#
# The ECU's userland is glibc 2.15 / armhf, the same generation as Debian
# wheezy (oldoldoldstable). ntpdate 4.2.6.p5 from wheezy links cleanly
# against that runtime (libc.so.6 + libssl1.0.0 / libcrypto.so.1.0.0, all
# present on the box). It runs one-shot as root with no privilege separation,
# which is why it works on this Linux 3.2 kernel where openntpd's privsep
# engine dies at startup.
#
# Usage:
#   ./packaging/fetch-ntpdate.sh [output-dir]
#
# Produces (default output dir = build/ntpdate-armv7):
#   <out>/ntpdate
#
# After running this:
#   make ipk-ntpdate NTPDATE_DIR=<out>

set -e

OUT="${1:-build/ntpdate-armv7}"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

# ntpdate 1:4.2.6.p5+dfsg-2+deb7u6, armhf, Debian wheezy. The
# snapshot.debian.org content-addressed URL is keyed by the blob hash, so it
# won't 404 on archive rotation; archive.debian.org's pool is the fallback.
PKG="ntpdate_4.2.6.p5+dfsg-2+deb7u6_armhf.deb"
URLS="
https://snapshot.debian.org/file/e10748bcd9cfecf2427aa096816f31308197e191
http://archive.debian.org/debian/pool/main/n/ntp/ntpdate_4.2.6.p5+dfsg-2+deb7u6_armhf.deb
"

# The primary URL is content-addressed, so the bytes are guaranteed. A
# mismatch means tampering or the wrong file — fail closed.
SHA256="88daa0d8b9af4984387e9cb317cb894c678678e8a28010c8d5667c978bf59703"

echo "+ fetching $PKG"
DEB="$WORK/ntpdate.deb"
ok=0
for url in $URLS; do
    [ -z "$url" ] && continue
    echo "  trying $url"
    if curl -fsSL --connect-timeout 10 --max-time 120 -o "$DEB" "$url"; then
        ok=1
        break
    fi
done
if [ "$ok" -ne 1 ]; then
    echo "ERROR: could not download $PKG from any mirror"
    exit 1
fi

# Verify checksum and fail closed on mismatch.
got_sha=$(shasum -a 256 "$DEB" 2>/dev/null | awk '{print $1}')
if [ -z "$got_sha" ]; then
    got_sha=$(sha256sum "$DEB" 2>/dev/null | awk '{print $1}')
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

# Extract data.tar.* from the .deb (which is an `ar` archive).
echo "+ extracting"
mkdir -p "$WORK/extract"
( cd "$WORK/extract" && ar x "$DEB" )

# data.tar.gz, data.tar.xz, data.tar.bz2 — try them all.
DATA=""
for f in "$WORK/extract"/data.tar.*; do
    [ -f "$f" ] && DATA="$f" && break
done
if [ -z "$DATA" ]; then
    echo "ERROR: no data.tar.* in $PKG"
    exit 1
fi

mkdir -p "$WORK/root"
case "$DATA" in
    *.gz)  tar -xzf "$DATA" -C "$WORK/root" ;;
    *.xz)  tar -xJf "$DATA" -C "$WORK/root" ;;
    *.bz2) tar -xjf "$DATA" -C "$WORK/root" ;;
    *.zst) tar --zstd -xf "$DATA" -C "$WORK/root" ;;
esac

# Stage the ntpdate ELF.
src="$WORK/root/usr/sbin/ntpdate"
if [ ! -f "$src" ]; then
    echo "ERROR: ./usr/sbin/ntpdate not found in $PKG"
    exit 1
fi

mkdir -p "$OUT"
cp "$src" "$OUT/ntpdate"
chmod 0755 "$OUT/ntpdate"

echo "+ wrote ntpdate to $OUT"
echo
echo "verify ELF arch:"
file "$OUT/ntpdate" 2>/dev/null | head -1 || true

echo
echo "next:"
echo "  make ipk-ntpdate NTPDATE_DIR=$OUT"

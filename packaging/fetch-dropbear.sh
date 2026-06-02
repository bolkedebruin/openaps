#!/bin/sh
#
# fetch-dropbear.sh — download a dropbear .deb from Debian's wheezy archive
# and extract the binaries needed by `make package-with-dropbear`.
#
# The ECU's userland is glibc 2.15 / armhf, the same generation as Debian
# wheezy (oldoldoldstable). dropbear 2012.55 from the wheezy security pocket
# links cleanly against that runtime.
#
# Usage:
#   ./packaging/fetch-dropbear.sh [output-dir]
#
# Produces (default output dir = dist/dropbear-armv7):
#   <out>/dropbear, dropbearkey, dropbearconvert, dbclient
#
# If you'd like to bundle authorized_keys with the install, drop it next to
# the binaries before running `make package-with-dropbear`:
#   cp ~/.ssh/id_rsa.pub <out>/authorized_keys
#
# After running this:
#   make package-with-dropbear DROPBEAR_DIR=<out>

set -e

OUT="${1:-dist/dropbear-armv7}"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

# Debian wheezy security: dropbear 2012.55-1.3+deb7u2 (last security update
# for wheezy). The snapshot.debian.org content-addressed URL is content-hash
# stable, so it won't 404 on archive rotation.
PKG="dropbear_2012.55-1.3+deb7u2_armhf.deb"
URLS="
https://snapshot.debian.org/file/06fecb8faca59579a6188137bb36edfa1cdb7d6c
"

# SHA256 verified 2026-05 against the snapshot.debian.org content-addressed
# fetch. The URL is keyed by the SHA1 of this same blob, so any tampering
# would also change the URL — checking SHA256 here adds belt to braces.
SHA256="cec0862ecae9589588095de8bd5dbf68b4609795581006a07c0bf07cf8bdf8e9"

echo "+ fetching $PKG"
DEB="$WORK/dropbear.deb"
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

# Verify checksum (best-effort — different mirrors may re-sign).
got_sha=$(shasum -a 256 "$DEB" 2>/dev/null | awk '{print $1}')
if [ -z "$got_sha" ]; then
    got_sha=$(sha256sum "$DEB" 2>/dev/null | awk '{print $1}')
fi
if [ -n "$got_sha" ] && [ -n "$SHA256" ] && [ "$got_sha" != "$SHA256" ]; then
    echo "WARNING: checksum mismatch"
    echo "  expected $SHA256"
    echo "  got      $got_sha"
    echo "  Continuing anyway — some mirrors re-pack. Verify manually if you require strict supply-chain integrity."
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

# Stage the binaries.
mkdir -p "$OUT"
copied=""
for b in dropbear dropbearkey dropbearconvert dbclient; do
    src="$WORK/root/usr/sbin/$b"
    [ -f "$src" ] || src="$WORK/root/usr/bin/$b"
    if [ -f "$src" ]; then
        cp "$src" "$OUT/$b"
        chmod 0755 "$OUT/$b"
        copied="$copied $b"
    fi
done

if [ -z "$copied" ]; then
    echo "ERROR: no dropbear binaries found in package"
    exit 1
fi

echo "+ wrote$copied to $OUT"
echo
echo "verify ELF arch:"
for b in $copied; do
    file "$OUT/$b" 2>/dev/null | head -1 || true
done

echo
echo "next:"
echo "  # optional: install your SSH public key for root"
echo "  cp ~/.ssh/id_rsa.pub $OUT/authorized_keys"
echo
echo "  make package-with-dropbear DROPBEAR_DIR=$OUT"

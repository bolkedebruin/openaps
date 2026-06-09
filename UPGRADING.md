# Upgrading OpenAPS v1.0.x → v1.1.x

v1.0.x installed OpenAPS as a one-shot tarball: the binaries were copied into
`/home/applications`, the stock firmware was removed, and nothing was tracked by
a package manager. v1.1.x is **opkg-managed** — packages come from a signed
GitHub feed, updates are `opkg upgrade`, and the release-signing key is the
**production** key (v1.0.0 shipped a placeholder that can't verify the feed).

Upgrading means making the box opkg-managed and swapping in the production key.
It is a one-time manual migration (a v1.0.x box has no opkg feed configured yet).

## Before you start

- You need SSH access to the ECU (the `dropbear` v1.0.x installed) and its IP.
- This does **not** touch your inverters, PAN, grid profiles, or power caps —
  those live in inverter NVRAM / `settings.json` and are read back on first contact.
- **`apsystems-stock` does not apply here.** A v1.0.x box already removed the
  stock firmware, so there is no stock to wrap/disable.

## 1. Copy the two "feed bootstrap" packages onto the box

The box can't reach the feed yet (no proxy, and its `release.pub` is the v1.0.0
placeholder), and it can't fetch over HTTPS itself (the whole reason for the
proxy). So pull `openaps-base` + `openaps-tls-proxy` from the release **on your
workstation** (which has modern TLS) and stream each over `ssh` — `cat`-ing it to
disk on the ECU. No `scp` is needed (dropbear ships none):

Set your ECU target once (no angle brackets — `<…>` are redirections in zsh);
`V` resolves to the latest release automatically, then paste the rest:

```sh
# on your workstation
ECU=root@192.168.1.50   # <-- edit: your ECU's ssh target
V=$(curl -fsSL https://api.github.com/repos/bolkedebruin/openaps/releases/latest \
      | grep -m1 '"tag_name"' | cut -d'"' -f4)
echo "latest release: $V"
REL="https://github.com/bolkedebruin/openaps/releases/download/$V"
curl -fsSL "$REL/openaps-base_${V}_all.ipk" | ssh "$ECU" 'cat > /home/openaps-base.ipk'
curl -fsSL "$REL/openaps-tls-proxy_${V}_armv7ahf-vfp-neon.ipk" | ssh "$ECU" 'cat > /home/openaps-tls-proxy.ipk'
```

## 2. Install the trust anchor + feed proxy (local install)

```sh
ssh "$ECU"
opkg install /home/openaps-base.ipk
opkg install /home/openaps-tls-proxy.ipk
```

`openaps-base` replaces the v1.0.0 **placeholder** `/etc/openaps/release.pub`
with the production key — this is required, or the proxy can't verify the feed.
`openaps-tls-proxy` brings up the loopback proxy (S47) and points opkg at the feed.

## 3. Update + install the rest over the feed

```sh
opkg update          # the proxy fetches + verifies the signed feed from GitHub
opkg install openaps-dropbear openaps-inv-driver openaps-ecu-zb \
             openaps-ecu-web openaps-ecu-sunspec openaps-recoveryd
```

These install over the running v1.0.x binaries, registering them in opkg at
v1.1.x and restarting each service. The box is now fully opkg-managed.
(`openaps-dropbear` reinstalls dropbear under opkg with its reboot-persistent
`S98` init; the host key is unchanged, so your `known_hosts` still matches.)

## 4. From here on

```sh
opkg update && opkg upgrade    # pulls only the packages whose content changed
```

Future releases are picked up the same way — the feed is content-addressed, so
`opkg upgrade` only re-downloads packages that actually changed.

## Notes

- **Root password:** this upgrade does not change it — your existing v1.0.x
  access (SSH key / password) is unaffected. The root password is set only by
  the first-time bootstrap, not by the packages, so `opkg upgrade` never touches it.
- **recoveryd:** `openaps-recoveryd` installs the same recovery surface under
  opkg (it's pulled in automatically by `openaps-ecu-web`), replacing the v1.0.x
  recoveryd — same role: owns root's `authorized_keys` and is the SSH access plane.
- **Verification:** the proxy verifies the release-key signature on the feed
  index and the SHA-256 of every `.ipk` before opkg sees it (opkg 0.1.8 itself
  can't — it enforces only MD5 and ignores signatures, which is why the proxy does it).
- **Rollback to stock** is still the v1.0.x path: restore the install backup
  (`/home/openaps-backup-*.tar.gz`) or run `openaps-rollback`.

# SSH client config for the OpenAPS ECU

The dropbear build bundled with the v1.0.x installer is **dropbear 2012.55** (the latest version available for the ECU's glibc 2.15 / armhf userland — newer dropbear builds need newer glibc). It predates the algorithms modern OpenSSH (9.x+) ships enabled by default, so a fresh `ssh root@<ECU-IP>` will refuse to negotiate.

You need to opt the host into the legacy algorithm set in your client config.

## Drop this into `~/.ssh/config`

Replace `<ECU-IP>` with your ECU's address (or use the friendly `Host openaps` alias).

```
Host openaps
    HostName <ECU-IP>
    User root

    # dropbear 2012.55 supports only ssh-rsa with SHA-1 signatures, not
    # rsa-sha2-256 / rsa-sha2-512. OpenSSH 8.8+ disables ssh-rsa by default
    # for host-key auth (server side) and pubkey auth (client side); re-enable
    # both selectively for this host.
    HostKeyAlgorithms +ssh-rsa
    PubkeyAcceptedAlgorithms +ssh-rsa

    # OpenSSH 9.0+ removed sha1-family KEX. dropbear 2012.55 offers
    # diffie-hellman-group14-sha1 (2048-bit DH group) and the (weaker)
    # group1-sha1. Whitelist group14-sha1 — never group1.
    KexAlgorithms +diffie-hellman-group14-sha1

    # Optional: pin the host-key file so a re-flash of the ECU (new host
    # key) doesn't poison your global known_hosts.
    UserKnownHostsFile ~/.ssh/known_hosts.openaps
    StrictHostKeyChecking accept-new
```

After that, `ssh openaps` should connect. The first connection prints the host fingerprint — verify it matches what the installer logged.

## What's NOT enabled (and why)

- `diffie-hellman-group1-sha1` — too weak (1024-bit DH). dropbear offers it; we deliberately don't whitelist it.
- `hmac-sha1` MAC — dropbear's default MAC list still works under OpenSSH 9.x; no flag needed.
- `aes128-cbc` / `aes256-cbc` — dropbear's `aes*-ctr` ciphers work under OpenSSH 9.x without extra config; CBC ciphers are not needed.

## Generating a key for the bootstrap

The bootstrap bundles an `authorized_keys` file when you pass `AUTHORIZED_KEYS` at packaging time. To bake in your public key:

```sh
make package-bootstrap ROOT_PW=openaps AUTHORIZED_KEYS=~/.ssh/id_rsa.pub
```

dropbear 2012.55 accepts RSA, DSS, and ECDSA keys, but **NOT ed25519** — generate an RSA key (`ssh-keygen -t rsa -b 4096`) if you don't already have one. The bootstrap appends the key to root's real home (`$ROOT_HOME/.ssh/authorized_keys`, derived from `/etc/passwd` — `/home/root` on the ECU) and deduplicates lines on re-run.

## If you don't pre-bake a key

The bootstrap still installs dropbear and sets a known root password from `ROOT_PW` (default `openaps`). Log in with that password, change it (`passwd`), then append your key to `~/.ssh/authorized_keys`.

## Rollback / recovery

If SSH stops working, telnet on `:23` is still active on the ECU. From there you can run the rollback CLI:

```sh
telnet <ECU-IP>
> /usr/local/bin/openaps-rollback
```

`openaps-rollback` reads `/etc/openaps/installed.json` to find the pre-install
backup, stops OpenAPS, and restores the stock firmware.

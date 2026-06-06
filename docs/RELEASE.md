# OpenAPS v1.0.9

Adds **`recoveryd`** — a dedicated, minimal root daemon that owns the box's SSH
access plane — plus a web **Security** page to manage SSH keys. This makes the
migrated box harder to lock yourself out of, independent of the fleet daemon and
the web service.

## New — `recoveryd` access daemon

- **A separate, hardened daemon owns `authorized_keys`.** `recoveryd` is the
  single writer of the root key file, independent of `inv-driver` (fleet) and
  `ecu-web` (UI). Source of truth is `/etc/recoveryd/access.json`; it renders
  `authorized_keys` as a **full rewrite** on every change **and at boot** (started
  by `S97-recoveryd`, before `S98-dropbear`), and ensures the dropbear host key
  exists. Provider-aware: `openaps` → `/root/.ssh/authorized_keys` + dropbear;
  `host` → `~<user>/.ssh/authorized_keys` (defers to the host `sshd`, for the
  Raspberry Pi target); `off` → no-op.
- **Anti-brick by construction.** Boot-render means the key file always reflects
  the managed list; `recoveryd` **refuses to render an empty `authorized_keys`
  over a non-empty one** (you can't accidentally zero out your keys — use
  `provider=off` to deliberately revoke); writes are durable (temp → `fsync` →
  rename → dir-`fsync`), so a power loss can't leave a truncated/empty file.
- **API:** length-prefixed protobuf over a **local UDS** (`/run/recoveryd.sock`,
  mode `0600`, `SO_PEERCRED` uid-0 gate) — `ListKeys` / `AddKey` / `RemoveKey` /
  `Status`. No network listener, no out-of-band path (a button-based recovery is
  future work). Keys are validated with `x/crypto/ssh`, fingerprinted
  (SHA256), and deduped; operator comments are rejected if they contain control
  characters (no `authorized_keys` line injection).

## New — web Security page

`ecu-web` gains a **Security** page that lists / adds / removes root SSH keys
(fingerprint, comment, added date), with an empty-state "add a key for shell
access" nudge. It's a thin proxy to `recoveryd` over the local UDS, behind the
existing operator session auth; **removing** a key requires the same single-use
step-up confirmation as other sensitive writes.

## Installer

The brownfield installer now installs `recoveryd` + `S97-recoveryd` and **seeds
`/etc/recoveryd/access.json`** from the operator's bundled key instead of writing
`/root/.ssh/authorized_keys` directly — `recoveryd` renders it at boot, before
dropbear binds. `idwriter` is unchanged (still the interim out-of-band path).

## Internal

Review extracted shared helpers used across daemons: `internal/udsutil`
(`SO_PEERCRED` peer-uid + stale-socket removal) and `internal/atomicfile`
(durable temp+fsync+rename write); `internal/ipc` and `internal/settings` now use
them.

## Compatibility matrix unchanged from v1.0.4

| Family | Telemetry | Grid profile | Set-power | Pairing |
|---|:--:|:--:|:--:|:--:|
| DS3 | ✅ | ✅ | ✅ | ✅ |
| QS1A | ✅ | ✅ | ✅ | ✅ |
| QS1 | ✅ | ✅ | ✅ | ✅ |
| DSP4 | ✅ | ✅ | ✅ | ⚠️ |
| YC600 | ✅ | ✅ | ✅ | ⚠️ |
| YC1000, QT2 | ⚠️ | ⚠️ | ⚠️ | ⚠️ |

## Install / upgrade

```sh
curl -H "Expect:" -F "file=@openaps-v1.0.9-ecu.tar.bz2" \
     http://<ECU-IP>/index.php/management/exec_upgrade_ecu_app
```

`{"res":0}` = received + launched; watch `/home/openaps-install.log` until the
reboot, then open `https://<ECU-IP>/` after ~1-2 minutes. Roll back with
`ssh root@<ECU-IP> /usr/local/bin/openaps-rollback`.

## Still deferred

- **Out-of-band recovery** — button/GPIO-triggered path into `recoveryd` (Mode-B);
  `idwriter` remains the interim path until then.
- **Provider switching over the UDS API** — set via `access.json` for now.
- **Signed-tarball OTA**, **passkeys**, **AES encrypt wire-up**, **v2 Pi/.deb**.

## Artifacts

- `openaps-v1.0.9-ecu.tar.bz2` — brownfield installer (now ships `recoveryd`).
- `SHA256SUMS`.

## Install caveats

Install only on LANs you control.

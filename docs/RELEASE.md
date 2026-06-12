# OpenAPS v1.1.8

Dependency security release. `govulncheck` reported three vulnerabilities
reachable from deployed code; all are fixed by this release and the scan is
now clean. No functional changes.

## Security

- **Go 1.26.4.** Fixes `net/textproto` unescaped error inputs (GO-2026-5039)
  and inefficient `crypto/x509` hostname parsing (GO-2026-5037), both
  reachable through `openaps-tls-proxy`'s HTTP server.
- **golang.org/x/crypto v0.52.0.** Fixes a DoS in `x/crypto/ssh` on
  pathological RSA/DSA parameters (GO-2026-5018), reachable through
  recoveryd's authorized-keys parsing.

## Changed

- `golang.org/x/sys` v0.46.0, `modernc.org/sqlite` v1.51.0.
- ecu-web frontend: `lit` 3.3.3, `@happy-dom/global-registrator` 20.10.1
  (transitive `happy-dom` pinned in `overrides` to clear the 7-day
  dependency-cooldown gate). The shipped bundle is byte-identical to
  v1.1.7's.

## Upgrading

Install the `.ipk` packages from this release over the opkg feed as usual;
see `UPGRADING.md`. No configuration or schema changes.

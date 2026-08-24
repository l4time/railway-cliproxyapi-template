# Changelog

All notable changes to CLIProxyAPI — Secure Release-Tracked are documented
here.

## 2026-08-25

- Initial package from the corrected Railway R2 proof.
- Pin CLIProxyAPI `v7.2.141` to immutable index digest
  `sha256:7f598ce64478a8a5f90ed76875e0e9b0e7d77b80e17184b13df18c3d5bdb3def`.
- Pin Management Center `v1.22.6` and SHA-256
  `e2643e0875e0024e5ff9ddf4569e4c58611ab0456aeb6fa6065ed3e6c2b721f4`.
- Preserve the proved Dockerfile, entrypoint, and health-proxy sources.
- Define one service, one `/data` volume, port `8080`, `/healthz`, one replica,
  Serverless off, and ON_FAILURE with one retry.
- Add separate generated proxy/management keys with fail-closed validation.
- Add the external stable-release controller, 12-hour soak, immutable digest,
  complete candidate smoke, one-per-24-hour limit, retained prior pin, and
  tested rollback path.
- Complete product kit `2026-07-04-v1`.
- Require numeric semantic-version progression for automated promotion,
  including correct multi-digit ordering and explicit lower-version rejection.
- Isolate emergency rollback smoke to the retained target so a broken outgoing
  image cannot block recovery; target failure still blocks commit.
- Pin the Go builder image by immutable digest. This reproducibility-only
  Dockerfile change does not alter package runtime code or accepted behavior.
- Make promotion and rollback update the exact Dockerfile
  `SOURCE_SHA256SUMS` record in the same fail-closed release transaction;
  malformed, missing, or stale checksum records block before mutation.

## Release update policy

Only the repository controller changes CLIProxyAPI pins. It must accept a
stable semantic GitHub release, resolve the matching immutable Docker manifest,
pass policy, then pass the full container smoke before committing. The
Management Center is reviewed and updated separately.

## Rollback policy

The controller retains one prior tag/digest and tests it before committing a
manual rollback. Persisted provider state is not automatically reversible.
Keep a stopped encrypted pre-update backup and restore it with the prior image
when compatibility cannot be proved.

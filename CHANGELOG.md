# Changelog

All notable changes to CLIProxyAPI — Daily Auto-Update are documented
here.

## 2026-08-25

- Add the consumer-side stable-release updater: 6-hour checks plus bounded
  jitter, rolling-24-hour attempt ceiling, anonymous official GitHub Releases,
  exact architecture/checksum verification, strict archive extraction,
  protected persistent ledger, private candidate probe, crash journal,
  quarantine, graceful cutover, bounded probation, and binary-only rollback.
- Retain the image binary as immutable fallback and bound storage to embedded,
  current, prior, and at most one staged executable. Preserve live user state.
- Add Go unit/race/vet coverage for semantic versions, soak/clock skew,
  cadence, ETag/rate limiting, URL allowlists, checksum drift, archive attacks,
  locks, bounds, journal recovery, and retry ceilings.
- Initial package from the corrected Railway R2 proof.
- Pin CLIProxyAPI `v7.2.141` to immutable index digest
  `sha256:7f598ce64478a8a5f90ed76875e0e9b0e7d77b80e17184b13df18c3d5bdb3def`.
- Pin Management Center `v1.22.6` and SHA-256
  `e2643e0875e0024e5ff9ddf4569e4c58611ab0456aeb6fa6065ed3e6c2b721f4`.
- Preserve the proved Dockerfile, entrypoint, and health-proxy sources.
- Define one service, one `/data` volume, port `8080`, `/healthz`, one replica,
  Serverless off, and ON_FAILURE with a finite maximum of 10 retries.
- Reconcile the restart maximum with Railway's independently reproduced public
  template serialization (`restartPolicyMaxRetries=10`); runtime code, auth,
  topology, health, volume, and release-controller behavior are unchanged.
- Add separate generated proxy/management keys with fail-closed validation.
- Persist Management API changes in protected `/data/state/config.yaml`; add a
  descriptor-relative, no-symlink, atomic reconciler that reapplies rotated
  Railway keys.
- Add restart checksum, generated-key rotation/old-key invalidation,
  ownership/mode, malformed-state, and symlink/path regressions.
- Harden persistence R2 after independent QA: render every wrapper-owned
  security/topology field from the package baseline on every boot; retain only
  `debug` and three bounded retry settings; reject ambiguous YAML tokens,
  duplicate/unknown fields, anchors, aliases, tags, inline/document syntax,
  credential order/cardinality drift, and malformed prefixes.
- Correct the health proxy's standalone config default to
  `/data/state/config.yaml` and prove wildcard `:8317` cannot survive a
  malicious persisted-config seed.
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
- Rename the public display name to `CLIProxyAPI — Daily Auto-Update` while
  retaining the stable public code and deploy URL
  `cliproxyapi-secure-release-tracked`.
- Record independent R7 QA acceptance of the exact 42-file updater package.
  Railway Phase A then proved an overdue boot check, verified
  `v7.2.141 → v7.2.142` promotion, authentication/UI/state continuity, and
  persistence across the sole service restart. A separate fresh Railway
  rollback proof acquired and privately probed bad-live `v7.2.143`, rejected
  its authenticated live semantics, quarantined its exact identity, and
  restored healthy `v7.2.141`; sanitized log and bounded-resource gates passed.

## Release update policy

Each deployed supervisor can advance its local executable after a 6-hour soak
and complete supply-chain/private-probe gates. It checks at boot when overdue,
then every 6 hours plus bounded jitter, with a rolling-24-hour attempt guarantee
for a continuously running service. The repository controller still qualifies
future embedded fallbacks through immutable image pins and full container
smoke. The Management Center is reviewed and updated separately.

## Rollback policy

The runtime retains one prior verified executable and automatically performs a
binary-only rollback on failed readiness/probation. Persisted provider state is
never automatically restored and may not be downgrade-compatible. Keep a
stopped encrypted backup for explicit recovery.

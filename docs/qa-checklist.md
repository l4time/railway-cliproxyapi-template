# CLIProxyAPI Package and Lifecycle QA Checklist

Status: independent R7 package QA and the combined Railway promotion/restart
and bad-live rollback proofs are accepted. Repository/template synchronization
and its clean public consumer smoke remain external lifecycle gates.

## Source and image

- [x] Corrected Dockerfile, entrypoint, and config-reconciler SHA-256 values are
  refreshed after the persistence correction.
- [x] Health proxy and config-reconciler R2 hashes are refreshed after the
  corrected persistent-config security proof.
- [x] CLIProxyAPI current/prior pins are immutable index digests.
- [x] Management Center `v1.22.6` uses `ADD --checksum`.
- [x] No runtime `latest`, branch, provider credential, Railway token, or panel
  downloader is configured.
- [x] Steady state is UID/GID `10001` with no new privileges.
- [x] Independent R7 source/build/history and native updater inspection passes
  with zero P0-P3.

## Railway contract

- [x] One service, one domain on `8080`, one `/data`, one replica.
- [x] `/healthz`, 60-second timeout, ON_FAILURE maximum 10 retries,
  Serverless off.
- [x] No database, Redis, Bucket, worker, scheduler, TCP proxy, or second app.
- [x] Upstream `8317` binds loopback only.
- [ ] Draft editor matches with zero hidden resources/start overrides.

## Variables and authentication

- [x] Separate `${{secret(64)}}` proxy/admin generators documented with correct
  descriptions and no prompt/default.
- [x] Runtime rejects missing, malformed, placeholder-like, short, and equal
  values.
- [x] Missing/wrong/correct proxy auth and management cross-key separation are
  in deterministic smoke.
- [x] Secrets are checked absent from image history, argv, child environment,
  logs, and health.
- [x] Independent secret/log/environment scan passes.
- [ ] Draft/public serialization proves distinct generated 64-hex values.

## Management UI and provider boundary

- [x] Panel checksum route test exists.
- [x] Management Center self-update/fallback is disabled; the separately
  verified CLIProxyAPI runtime updater never replaces the panel.
- [x] Remote authorization path and provider-dependent browser/device-code
  behavior are documented.
- [x] No shared credential, included subscription, permission, free-use,
  provider-access, privacy, cost, or model guarantee exists.
- [x] Independent source/runtime check proves no first-access panel download.
- [x] Provider/legal wording and the no-custom-asset exception were accepted
  for the existing marketplace listing.

## Persistence/recovery

- [x] `/data/auth`, `/data/home`, `/data/state`, and `/data/update`
  ownership/modes tested.
- [x] Protected config ownership/mode/single-link, malformed state, state/config
  symlink, unsafe state directory, atomic temp cleanup, debug checksum-identical
  restart, and generated-key rotation/old-key invalidation are deterministic.
- [x] Wrapper-owned host/port/TLS/remote-management/panel/auth-dir/logging/
  statistics/WebSocket-auth drift is reasserted; `/proc/net/tcp` proves no
  wildcard `:8317`.
- [x] Malformed credential prefixes, duplicate/reordered credential blocks,
  inline/quoted ambiguity, anchors, aliases, tags, directives, multi-document
  markers, unknown fields, wrong shapes, and key cardinality drift fail closed.
- [x] Explicit user allowlist is limited to debug plus three bounded retry
  fields and survives checksum-identically across restart and key rotation.
- [x] Prior/current/prior/current and restart preserve a marker.
- [x] Stopped encrypted backup and same-image restore path documented.
- [x] `/data/lost+found` is identified as excluded ext4 metadata; no unsafe
  recursive chown/delete instruction exists.
- [x] Live-copy, multiple replicas, atomic replacement, and general state
  downgrade claims are rejected.
- [x] Independent recovery wording aligns with Railway Phase A
  promotion/restart and fresh R3 bad-live binary-only rollback evidence.

## Runtime updater

- [x] Exact forward stable semver only; draft/prerelease/future/malformed and
  downgrade candidates rejected.
- [x] Six-hour soak, 6-hour cadence, deterministic jitter at most 30 minutes,
  boot-overdue check, transient retry and rolling-24-hour ceiling implemented.
- [x] Official GitHub metadata and final-download hosts, TLS, fresh equal-tag
  checksum identity retrieval, observed ETag/rate-limit,
  asset count/name/size, architecture and checksum-line contracts enforced.
- [x] Same-tag checksum drift, duplicate assets, links, traversal, duplicate
  tar entries, wrong executable name/mode and storage bounds fail closed.
- [x] `/data/update` uses restrictive modes, `O_NOFOLLOW`, advisory lock,
  atomic `fsync` + rename, bounded retention and phase journal recovery.
- [x] Candidate probe is loopback-only and covers exact version, readiness,
  proxy/management missing/cross/correct auth, pinned UI, and clean exit.
- [x] Cutover reaps the old child, preserves live user state, validates proxy
  and management credential separation/config/UI before and after probation,
  and performs binary-only automatic rollback on failure.
- [x] Transient acquisition failures retry without quarantine; deterministic
  candidate failures quarantine only the exact `tag@checksum` identity.
- [x] Disk headroom and rollback-ledger preparation complete before the old
  child stops; backward or invalid persisted clocks force an immediate check.
- [x] No updater HTTP endpoint or Railway/GitHub/provider credential exists.
- [x] Independent R7 adversarial fixture/container matrix passes at exact
  digest `a5a25a49…`; native updater peak was 7.758 MiB.
- [ ] Clean consumer update from a newer official stable release is observed
  without weakening supply-chain or state protections.

## External release controller

- [x] Exact stable semver only; draft/prerelease/future/malformed rejected.
- [x] 12-hour soak and one-promotion-per-24-hour enforced.
- [x] Docker tag resolves to raw immutable manifest digest.
- [x] Duplicate/current and retained-prior promotion refused.
- [x] Full smoke precedes commit; only Dockerfile/ledger staged.
- [x] Prior retained and manual rollback receives full smoke.
- [x] No Railway/app/provider secret; minimal GitHub contents permission.
- [x] Checkout action uses immutable commit, not a moving branch.
- [x] Numeric semantic ordering requires candidate greater than current;
  `v7.2.139 < v7.2.141` and multi-digit ordering have fixtures.
- [x] Rollback-target mode never builds/boots outgoing current; target failure
  blocks the success-gated commit.
- [x] Promote and rollback refresh the exact Dockerfile source checksum.
- [x] Missing, malformed, reordered, duplicate, or stale source-checksum state
  fails before release mutation; post-change verification is required.
- [x] Workflow stages Dockerfile, release state, and `SOURCE_SHA256SUMS`
  together after smoke.
- [ ] Independent workflow syntax/security/default-branch review passes.
- [ ] Repository branch protection and action permissions accepted.

## Product kit

- [x] README contains deploy/setup, keys, remote OAuth, state, backup/restore,
  update/rollback, troubleshooting, support, legal, and limits.
- [x] Environment, architecture/security, operations, contract, controller,
  build notes, marketplace, inventory, completion, and QA docs exist.
- [x] Changelog, MIT license, notices, trademarks, security policy,
  CONTRIBUTING, CODEOWNERS, issue routes/form, and labels exist.
- [x] Explicit no-custom-asset exception and future provenance rules exist.
- [x] Existing relative/public support and security links passed after original
  repository placement; synchronized update links are rechecked before push.
- [x] Independent QA/publish packet accepted the no-custom-asset exception.

## Builder local gates

- [x] Runtime-updater exact 42-file digest, source manifest and SHA-256 report
  captured.
- [x] Runtime-updater Go unit/race/vet suite passes.
- [x] JSON and YAML parse.
- [x] Shell syntax passes; optional shell/action/Docker linters were unavailable.
- [x] Corrected full forward-transition Docker suite passes.
- [x] Corrected rollback-target-only Docker suite passes with outgoing excluded.
- [x] Each corrected test run leaves zero owned container/network/volume/image/temp state.

## External lifecycle

- [x] Independent R7 QA has zero P0-P3 blockers.
- [x] Public repository/support/security routes work at the pre-correction
  accepted commit; correction sync remains separate.
- [x] Railway draft config/variables/icon/overview inspected.
- [x] Clean unpublished-draft smoke passed before publication.
- [x] Publish Approval Packet approved and publication completed.
- [x] Public listing/search/deploy button work.
- [ ] Clean public consumer smoke passes the synchronized 42-file
  Daily Auto-Update contract and serialization.
- [x] All disposable resources from completed/failed smokes were deleted
  immediately.
- [ ] Product-kit, registry, monitoring, support, metrics, and review rows close.

## Hard blockers

Runtime/hash drift without proof; mutable source; auth bypass or key confusion;
secret exposure; remote panel fallback/checksum drift; root steady state;
missing/wrong volume; extra service; multiple replicas; Serverless; untested
promotion/rollback; updater host/checksum/archive/probe/journal drift;
provider-access or automatic-update overclaim; unsafe
backup; broken support/security route; missing independent lifecycle gate.

# CLIProxyAPI Package and Lifecycle QA Checklist

Status: builder checks are marked. Independent QA and external lifecycle gates
remain unchecked.

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
- [ ] Independent build/image/history inspection passes.

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
- [ ] Independent secret scan passes.
- [ ] Draft/public serialization proves distinct generated 64-hex values.

## Management UI and provider boundary

- [x] Panel checksum route test exists.
- [x] Auto-update/fallback is disabled and logs are checked.
- [x] Remote authorization path and provider-dependent browser/device-code
  behavior are documented.
- [x] No shared credential, included subscription, permission, free-use,
  provider-access, privacy, cost, or model guarantee exists.
- [ ] Independent source/runtime check proves no first-access panel download.
- [ ] Provider/legal wording accepted for marketplace.

## Persistence/recovery

- [x] `/data/auth`, `/data/home`, `/data/state` ownership/modes tested.
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
- [ ] Independent recovery wording and corrected Railway evidence align.

## Release controller

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
- [ ] Relative/public link validation passes after repository placement.
- [ ] Independent QA/publish packet accepts asset exception.

## Builder local gates

- [x] Final 38-file manifest and SHA-256 report captured.
- [x] Final corrected Python unit/static tests pass: 15/15.
- [x] JSON and YAML parse.
- [x] Shell syntax passes; optional shell/action/Docker linters were unavailable.
- [x] Corrected full forward-transition Docker suite passes.
- [x] Corrected rollback-target-only Docker suite passes with outgoing excluded.
- [x] Each corrected test run leaves zero owned container/network/volume/image/temp state.

## External lifecycle

- [ ] Independent QA has zero blockers.
- [x] Public repository/support/security routes work at the pre-correction
  accepted commit; correction sync remains separate.
- [x] Railway draft config/variables/icon/overview inspected.
- [x] Clean unpublished-draft smoke passed before publication.
- [x] Publish Approval Packet approved and publication completed.
- [x] Public listing/search/deploy button work.
- [ ] Clean public consumer smoke passes complete contract and update-policy
  serialization.
- [x] All disposable resources from completed/failed smokes were deleted
  immediately.
- [ ] Product-kit, registry, monitoring, support, metrics, and review rows close.

## Hard blockers

Runtime/hash drift without proof; mutable source; auth bypass or key confusion;
secret exposure; remote panel fallback/checksum drift; root steady state;
missing/wrong volume; extra service; multiple replicas; Serverless; untested
promotion/rollback; provider-access or automatic-update overclaim; unsafe
backup; broken support/security route; missing independent lifecycle gate.
